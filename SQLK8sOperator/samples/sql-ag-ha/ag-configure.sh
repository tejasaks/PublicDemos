#!/usr/bin/env bash
# ============================================================================
# ag-configure.sh — Automated T-SQL Setup for HA Availability Group
# ============================================================================
# Shell script version of ag-configure.md. Executes all T-SQL steps required
# to create the ProductionAG Availability Group.
#
# USAGE:
#   ./ag-configure.sh              # Run all steps (1 – 8)
#   ./ag-configure.sh all          # Same as above
#   ./ag-configure.sh listener     # Set up AG Listener (step 9)
#   ./ag-configure.sh verify       # Verify AG status (step 8)
#   ./ag-configure.sh 5            # Run a single step
#
# PREREQUISITES:
#   - kubectl configured and pointing at the target cluster
#   - ag-deploy.yaml already applied (pods Running, secrets exist)
# ============================================================================

set -uo pipefail

# ============================================================================
# CONFIGURATION — Update these values to match your environment
# ============================================================================

NAMESPACE="mssql"
PRIMARY="sql-ag-0"
REPLICAS=("sql-ag-0" "sql-ag-1" "sql-ag-2")
SECONDARIES=("sql-ag-1" "sql-ag-2")
CONTAINER="mssql"

SA_PASSWORD='YourStrong@Passw0rd!'
AG_HELPER_PASSWORD='AGHelper@Passw0rd!'
MASTER_KEY_PASSWORD='MasterKey@Passw0rd!'
REPLICA_LOGIN_PASSWORD='ReplicaLogin@Passw0rd!'

AG_NAME="ProductionAG"
AG_RESOURCE_NAME="production-ag"
AG_LISTENER_NAME="productionag-listener"
DATABASE_NAME="ApplicationDB"
ENDPOINT_PORT=5022
LISTENER_PORT=1433

# ============================================================================
# HELPER FUNCTIONS
# ============================================================================

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
CYAN='\033[0;36m'
BOLD='\033[1m'
NC='\033[0m'

info()  { echo -e "${CYAN}[INFO]${NC}  $*"; }
ok()    { echo -e "${GREEN}[ OK ]${NC}  $*"; }
warn()  { echo -e "${YELLOW}[WARN]${NC}  $*"; }
err()   { echo -e "${RED}[FAIL]${NC}  $*"; }
banner() {
    echo ""
    echo -e "${BOLD}${CYAN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
    echo -e "${BOLD}${CYAN}  $*${NC}"
    echo -e "${BOLD}${CYAN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
}

run_sql() {
    local pod="$1"; local sql="$2"
    kubectl exec "$pod" -n "$NAMESPACE" -c "$CONTAINER" -- \
        /opt/mssql-tools18/bin/sqlcmd -S localhost -U sa \
        -P "$SA_PASSWORD" -C -b -Q "$sql"
}

run_sql_check() {
    local pod="$1"; local sql="$2"
    if run_sql "$pod" "$sql"; then ok "Completed on $pod"
    else err "T-SQL execution failed on $pod"; return 1; fi
}

cidr_prefix_to_mask() {
    local prefix=$1
    local mask=$(( 0xFFFFFFFF << (32 - prefix) ))
    printf "%d.%d.%d.%d\n" \
        $(( (mask >> 24) & 255 )) $(( (mask >> 16) & 255 )) \
        $(( (mask >> 8)  & 255 )) $(( mask         & 255 ))
}

# ============================================================================
# PREREQUISITE CHECKS
# ============================================================================

check_prerequisites() {
    banner "Prerequisite Checks"
    if ! command -v kubectl &>/dev/null; then err "kubectl not found"; exit 1; fi
    ok "kubectl available"

    for pod in "${REPLICAS[@]}"; do
        local phase
        phase=$(kubectl get pod "$pod" -n "$NAMESPACE" -o jsonpath='{.status.phase}' 2>/dev/null) || true
        if [ "$phase" != "Running" ]; then
            err "Pod $pod is not Running (phase: ${phase:-not found}). Apply ag-deploy.yaml first."
            exit 1
        fi
        ok "Pod $pod is Running"
    done

    if ! kubectl get secret sql-ag-helper -n "$NAMESPACE" &>/dev/null; then
        err "Secret sql-ag-helper not found. Apply ag-deploy.yaml first."
        exit 1
    fi
    ok "Secret sql-ag-helper exists"
}

# ============================================================================
# STEPS 1-8
# ============================================================================

step_1() {
    banner "Step 1: Create AG Helper Login (all replicas)"
    for pod in "${REPLICAS[@]}"; do
        info "Creating AG Helper login on $pod..."
        run_sql_check "$pod" "
CREATE LOGIN ag_helper WITH PASSWORD = '$AG_HELPER_PASSWORD';
GO
GRANT VIEW SERVER STATE TO ag_helper;
GRANT ALTER ANY AVAILABILITY GROUP TO ag_helper;
GO
"
    done
}

step_2() {
    banner "Step 2: Create Master Key and Certificates (all replicas)"
    for i in "${!REPLICAS[@]}"; do
        local pod="${REPLICAS[$i]}"
        info "Creating master key and certificate on $pod..."
        run_sql_check "$pod" "
CREATE MASTER KEY ENCRYPTION BY PASSWORD = '$MASTER_KEY_PASSWORD';
GO
CREATE CERTIFICATE AG_Cert_${i}
    WITH SUBJECT = 'AG Certificate for ${pod}',
    EXPIRY_DATE = '2030-12-31';
GO
"
    done
}

step_3() {
    banner "Step 3: Export and Import Certificates"

    info "3.1 — Backing up certificates..."
    for i in "${!REPLICAS[@]}"; do
        local pod="${REPLICAS[$i]}"
        run_sql_check "$pod" "
BACKUP CERTIFICATE AG_Cert_${i}
    TO FILE = '/var/opt/mssql/data/AG_Cert_${i}.cer';
GO
"
    done

    info "3.2 — Exchanging certificate files..."
    local tmpdir; tmpdir=$(mktemp -d 2>/dev/null || mktemp -d -t 'ag-certs')
    for i in "${!REPLICAS[@]}"; do
        kubectl cp "$NAMESPACE/${REPLICAS[$i]}:/var/opt/mssql/data/AG_Cert_${i}.cer" \
            "$tmpdir/AG_Cert_${i}.cer" -c "$CONTAINER"
    done
    for i in "${!REPLICAS[@]}"; do
        for j in "${!REPLICAS[@]}"; do
            [ "$i" != "$j" ] && kubectl cp "$tmpdir/AG_Cert_${j}.cer" \
                "$NAMESPACE/${REPLICAS[$i]}:/var/opt/mssql/data/AG_Cert_${j}.cer" -c "$CONTAINER"
        done
    done
    rm -rf "$tmpdir"
    ok "Certificate exchange complete"

    info "3.3 — Importing certificates..."
    for i in "${!REPLICAS[@]}"; do
        local pod="${REPLICAS[$i]}" sql=""
        for j in "${!REPLICAS[@]}"; do
            if [ "$i" != "$j" ]; then
                sql+="
CREATE LOGIN sql_ag_${j}_login WITH PASSWORD = '$REPLICA_LOGIN_PASSWORD';
CREATE USER sql_ag_${j}_user FOR LOGIN sql_ag_${j}_login;
CREATE CERTIFICATE AG_Cert_${j}
    AUTHORIZATION sql_ag_${j}_user
    FROM FILE = '/var/opt/mssql/data/AG_Cert_${j}.cer';
GO
"
            fi
        done
        info "  Importing on $pod..."
        run_sql_check "$pod" "$sql"
    done
}

step_4() {
    banner "Step 4: Create Database Mirroring Endpoints (all replicas)"
    for i in "${!REPLICAS[@]}"; do
        local pod="${REPLICAS[$i]}" grants=""
        for j in "${!REPLICAS[@]}"; do
            [ "$i" != "$j" ] && grants+="GRANT CONNECT ON ENDPOINT::AG_Endpoint TO sql_ag_${j}_login;
"
        done
        info "Creating endpoint on $pod..."
        run_sql_check "$pod" "
CREATE ENDPOINT AG_Endpoint
    STATE = STARTED
    AS TCP (LISTENER_PORT = $ENDPOINT_PORT, LISTENER_IP = ALL)
    FOR DATABASE_MIRRORING (
        ROLE = ALL,
        AUTHENTICATION = CERTIFICATE AG_Cert_${i},
        ENCRYPTION = REQUIRED ALGORITHM AES
    );
GO
${grants}
GO
"
    done
}

step_5() {
    banner "Step 5: Create Database (primary: $PRIMARY)"
    info "Creating $DATABASE_NAME..."
    run_sql_check "$PRIMARY" "
CREATE DATABASE [$DATABASE_NAME];
GO
ALTER DATABASE [$DATABASE_NAME] SET RECOVERY FULL;
GO
BACKUP DATABASE [$DATABASE_NAME]
    TO DISK = '/var/opt/mssql/backup/${DATABASE_NAME}_init.bak'
    WITH INIT, COMPRESSION;
GO
"
}

step_6() {
    banner "Step 6: Create Availability Group (primary: $PRIMARY)"
    local replica_on=""
    for i in "${!REPLICAS[@]}"; do
        local pod="${REPLICAS[$i]}"
        [ "$i" -gt 0 ] && replica_on+=","
        replica_on+="
        N'${pod}' WITH (
            ENDPOINT_URL = N'TCP://${pod}.${NAMESPACE}.svc.cluster.local:${ENDPOINT_PORT}',
            AVAILABILITY_MODE = SYNCHRONOUS_COMMIT,
            FAILOVER_MODE = EXTERNAL,
            SEEDING_MODE = AUTOMATIC,
            SESSION_TIMEOUT = 10,
            SECONDARY_ROLE (ALLOW_CONNECTIONS = READ_ONLY)
        )"
    done
    info "Creating $AG_NAME..."
    run_sql_check "$PRIMARY" "
CREATE AVAILABILITY GROUP [$AG_NAME]
    WITH (
        CLUSTER_TYPE = EXTERNAL,
        DB_FAILOVER = ON,
        REQUIRED_SYNCHRONIZED_SECONDARIES_TO_COMMIT = 1
    )
    FOR DATABASE [$DATABASE_NAME]
    REPLICA ON${replica_on};
GO
"
}

step_7() {
    banner "Step 7: Join Secondary Replicas"
    for pod in "${SECONDARIES[@]}"; do
        info "Joining $pod to $AG_NAME..."
        run_sql_check "$pod" "
ALTER AVAILABILITY GROUP [$AG_NAME] JOIN WITH (CLUSTER_TYPE = EXTERNAL);
GO
ALTER AVAILABILITY GROUP [$AG_NAME] GRANT CREATE ANY DATABASE;
GO
"
    done
}

step_8() {
    banner "Step 8: Verify AG Status"
    info "Querying replica status..."
    run_sql "$PRIMARY" "
SELECT ag.name AS ag_name, ar.replica_server_name,
    ars.role_desc, ars.synchronization_health_desc, ars.connected_state_desc
FROM sys.availability_groups ag
JOIN sys.availability_replicas ar ON ag.group_id = ar.group_id
JOIN sys.dm_hadr_availability_replica_states ars ON ar.replica_id = ars.replica_id
ORDER BY ar.replica_server_name;
GO
"
    echo ""
    info "Querying database sync status..."
    run_sql "$PRIMARY" "
SELECT ag.name AS ag_name, d.name AS database_name,
    drs.synchronization_state_desc, drs.is_primary_replica, drs.synchronization_health_desc
FROM sys.dm_hadr_database_replica_states drs
JOIN sys.databases d ON drs.database_id = d.database_id
JOIN sys.availability_groups ag ON drs.group_id = ag.group_id
ORDER BY d.name;
GO
"
    echo ""
    ok "Expected: All replicas CONNECTED + HEALTHY, databases SYNCHRONIZED"
}

# ============================================================================
# STEP 9: AG Listener Setup
# ============================================================================

step_9_listener() {
    banner "Step 9: AG Listener Setup"

    if ! kubectl get sqlserverag "$AG_RESOURCE_NAME" -n "$NAMESPACE" &>/dev/null; then
        warn "SQLServerAG resource '$AG_RESOURCE_NAME' not found."
        warn "Apply ag-deploy.yaml first."
        return 1
    fi
    ok "SQLServerAG resource found"

    info "Waiting for listener VIP (up to 60s)..."
    local vip="" waited=0
    while [ -z "$vip" ] && [ $waited -lt 60 ]; do
        vip=$(kubectl get sqlserverag "$AG_RESOURCE_NAME" -n "$NAMESPACE" \
            -o jsonpath='{.status.listener.vip}' 2>/dev/null) || true
        if [ -z "$vip" ]; then sleep 2; waited=$((waited + 2)); printf "."; fi
    done
    echo ""
    [ -z "$vip" ] && vip=$(kubectl get svc "$AG_LISTENER_NAME" -n "$NAMESPACE" \
        -o jsonpath='{.spec.clusterIP}' 2>/dev/null) || true
    if [ -z "$vip" ]; then err "Could not retrieve VIP after 60s."; return 1; fi
    ok "Listener VIP: $vip"

    info "Detecting cluster service CIDR..."
    local cidr="" subnet_mask=""
    cidr=$(kubectl get pods -n kube-system -l component=kube-apiserver \
        -o jsonpath='{.items[0].spec.containers[0].command}' 2>/dev/null \
        | tr '[],"' ' ' | grep -oE 'service-cluster-ip-range=[^ ]+' \
        | head -1 | cut -d= -f2) 2>/dev/null || true
    [ -z "$cidr" ] && cidr=$(kubectl get cm kubeadm-config -n kube-system \
        -o jsonpath='{.data.ClusterConfiguration}' 2>/dev/null \
        | sed -n 's/.*serviceSubnet:[[:space:]]*\([^ ]*\).*/\1/p') 2>/dev/null || true
    [ -z "$cidr" ] && cidr=$(kubectl get pods -n kube-system -l component=kube-controller-manager \
        -o jsonpath='{.items[0].spec.containers[0].command}' 2>/dev/null \
        | tr '[],"' ' ' | grep -oE 'service-cluster-ip-range=[^ ]+' \
        | head -1 | cut -d= -f2) 2>/dev/null || true

    if [ -n "$cidr" ]; then
        subnet_mask=$(cidr_prefix_to_mask "$(echo "$cidr" | cut -d/ -f2)")
        ok "Service CIDR: $cidr → Subnet mask: $subnet_mask"
    fi

    if [ -n "$subnet_mask" ]; then
        info "Creating AG Listener via T-SQL..."
        run_sql_check "$PRIMARY" "
ALTER AVAILABILITY GROUP [$AG_NAME]
ADD LISTENER '$AG_LISTENER_NAME' (
    WITH IP (('$vip', '$subnet_mask')),
    PORT = $LISTENER_PORT
);
GO
"
    else
        warn "Could not auto-detect subnet mask."
        echo ""
        echo -e "${BOLD}${YELLOW}  Run on primary ($PRIMARY):${NC}"
        echo ""
        echo "  ALTER AVAILABILITY GROUP [$AG_NAME]"
        echo "  ADD LISTENER '$AG_LISTENER_NAME' ("
        echo "      WITH IP (('$vip', '<SUBNET_MASK>')),"
        echo "      PORT = $LISTENER_PORT"
        echo "  );"
        echo ""
        echo "  Common masks: /12=255.240.0.0  /16=255.255.0.0  /24=255.255.255.0"
        return 0
    fi

    sleep 5
    local phase
    phase=$(kubectl get sqlserverag "$AG_RESOURCE_NAME" -n "$NAMESPACE" \
        -o jsonpath='{.status.listener.phase}' 2>/dev/null) || true
    [ "$phase" = "Ready" ] && ok "Listener is Ready!" || info "Listener phase: ${phase:-unknown}"
    echo ""
    ok "Connect via: sqlcmd -S $vip,$LISTENER_PORT -U sa -P '<password>'"
}

# ============================================================================
# MAIN
# ============================================================================

usage() {
    echo "Usage: $0 [COMMAND]"
    echo ""
    echo "Commands:"
    echo "  all       Run all steps 1-8 (default)"
    echo "  listener  Set up AG Listener (step 9)"
    echo "  verify    Verify AG status (step 8)"
    echo "  1-8       Run a specific step"
}

main() {
    echo -e "${BOLD}${CYAN}"
    echo "╔══════════════════════════════════════════════════════════════╗"
    echo "║  SQL Server AG — HA Configuration                          ║"
    echo "╚══════════════════════════════════════════════════════════════╝"
    echo -e "${NC}"

    local cmd="${1:-all}"
    case "$cmd" in
        all)
            check_prerequisites; step_1; step_2; step_3; step_4
            step_5; step_6; step_7; step_8
            echo ""
            banner "All Steps Complete!"
            ok "Availability Group '$AG_NAME' is configured."
            info "Run '$0 listener' to set up the AG Listener."
            ;;
        listener)  step_9_listener ;;
        verify)    step_8 ;;
        [1-8])     "step_$cmd" ;;
        -h|--help) usage ;;
        *)         err "Unknown: $cmd"; usage; exit 1 ;;
    esac
}

main "$@"
