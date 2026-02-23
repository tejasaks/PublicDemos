#!/usr/bin/env bash
# ============================================================================
# ag-step2-setup-ag.sh — Automated AG Setup
# ============================================================================
# Shell script version of ag-step2-setup-ag.md
#
# Executes all T-SQL steps required to create an Availability Group after
# the SQL Server replicas have been deployed via ag-step1-replicas.yaml.
#
# USAGE:
#   ./ag-step2-setup-ag.sh              # Run all steps (2.1 – 2.8)
#   ./ag-step2-setup-ag.sh all          # Same as above
#   ./ag-step2-setup-ag.sh listener     # Set up AG Listener (after Step 3)
#   ./ag-step2-setup-ag.sh 2.5          # Run a single step
#   ./ag-step2-setup-ag.sh verify       # Verify AG status (same as 2.8)
#
# PREREQUISITES:
#   - kubectl configured and pointing at the target cluster
#   - ag-step1-replicas.yaml already applied (pods Running, secret exists)
#   - For the 'listener' sub-command: ag-step3-ag-config.yaml applied
#
# NOTES:
#   - The .md file remains the authoritative reference with explanations.
#   - This script mirrors the .md steps 1:1 for easy cross-referencing.
#   - All passwords below must match the values used in ag-step1-replicas.yaml.
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
AG_RESOURCE_NAME="production-ag"          # SQLServerAG K8s resource name
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

# Execute T-SQL on a pod via sqlcmd. Returns the sqlcmd exit code.
run_sql() {
    local pod="$1"
    local sql="$2"
    kubectl exec "$pod" -n "$NAMESPACE" -c "$CONTAINER" -- \
        /opt/mssql-tools18/bin/sqlcmd -S localhost -U sa \
        -P "$SA_PASSWORD" -C -b -Q "$sql"
}

# Execute T-SQL with pass/fail messaging.
run_sql_check() {
    local pod="$1"
    local sql="$2"
    if run_sql "$pod" "$sql"; then
        ok "Completed on $pod"
    else
        err "T-SQL execution failed on $pod"
        return 1
    fi
}

# Convert a CIDR prefix length (e.g. 16) to a dotted-decimal subnet mask.
cidr_prefix_to_mask() {
    local prefix=$1
    local mask=$(( 0xFFFFFFFF << (32 - prefix) ))
    printf "%d.%d.%d.%d\n" \
        $(( (mask >> 24) & 255 )) \
        $(( (mask >> 16) & 255 )) \
        $(( (mask >> 8)  & 255 )) \
        $(( mask         & 255 ))
}

# ============================================================================
# PREREQUISITE CHECKS
# ============================================================================

check_prerequisites() {
    banner "Prerequisite Checks"

    if ! command -v kubectl &>/dev/null; then
        err "kubectl not found in PATH"; exit 1
    fi
    ok "kubectl available"

    for pod in "${REPLICAS[@]}"; do
        local phase
        phase=$(kubectl get pod "$pod" -n "$NAMESPACE" -o jsonpath='{.status.phase}' 2>/dev/null) || true
        if [ "$phase" != "Running" ]; then
            err "Pod $pod is not Running (phase: ${phase:-not found})"
            err "Apply ag-step1-replicas.yaml first"
            exit 1
        fi

        # Check ready containers
        local ready
        ready=$(kubectl get pod "$pod" -n "$NAMESPACE" -o jsonpath='{.status.containerStatuses[0].ready}' 2>/dev/null) || true
        if [ "$ready" != "true" ]; then
            err "Pod $pod container is not Ready"
            exit 1
        fi
        ok "Pod $pod is Running and Ready"
    done

    if ! kubectl get secret sql-ag-helper -n "$NAMESPACE" &>/dev/null; then
        err "Secret sql-ag-helper not found in namespace $NAMESPACE"
        err "Deploy ag-step1-replicas.yaml first"
        exit 1
    fi
    ok "Secret sql-ag-helper exists"
}

# ============================================================================
# STEP 2.1: Create AG Helper Login
# ============================================================================

step_2_1() {
    banner "Step 2.1: Create AG Helper Login (all replicas)"

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

# ============================================================================
# STEP 2.2: Create Master Key and Certificates
# ============================================================================

step_2_2() {
    banner "Step 2.2: Create Master Key and Certificates (all replicas)"

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

# ============================================================================
# STEP 2.3: Export and Import Certificates
# ============================================================================

step_2_3() {
    banner "Step 2.3: Export and Import Certificates"

    # --- 2.3.1: Backup certificates on each replica ---
    info "Step 2.3.1 — Backing up certificates..."
    for i in "${!REPLICAS[@]}"; do
        local pod="${REPLICAS[$i]}"
        info "  Backing up AG_Cert_${i} on $pod..."
        run_sql_check "$pod" "
BACKUP CERTIFICATE AG_Cert_${i}
    TO FILE = '/var/opt/mssql/data/AG_Cert_${i}.cer';
GO
"
    done

    # --- 2.3.2: Exchange certificate files between pods ---
    info "Step 2.3.2 — Exchanging certificate files between replicas..."
    local tmpdir
    tmpdir=$(mktemp -d 2>/dev/null || mktemp -d -t 'ag-certs')
    info "  Temp directory: $tmpdir"

    # Download all certificates to local machine
    for i in "${!REPLICAS[@]}"; do
        local pod="${REPLICAS[$i]}"
        info "  Downloading AG_Cert_${i}.cer from $pod..."
        kubectl cp "$NAMESPACE/$pod:/var/opt/mssql/data/AG_Cert_${i}.cer" \
            "$tmpdir/AG_Cert_${i}.cer" -c "$CONTAINER"
    done

    # Upload each certificate to replicas that don't own it
    for i in "${!REPLICAS[@]}"; do
        local pod="${REPLICAS[$i]}"
        for j in "${!REPLICAS[@]}"; do
            if [ "$i" != "$j" ]; then
                info "  Uploading AG_Cert_${j}.cer → $pod..."
                kubectl cp "$tmpdir/AG_Cert_${j}.cer" \
                    "$NAMESPACE/$pod:/var/opt/mssql/data/AG_Cert_${j}.cer" -c "$CONTAINER"
            fi
        done
    done

    rm -rf "$tmpdir"
    ok "Certificate exchange complete"

    # --- 2.3.3: Create logins and import certificates ---
    info "Step 2.3.3 — Creating logins and importing certificates..."
    for i in "${!REPLICAS[@]}"; do
        local pod="${REPLICAS[$i]}"
        local sql=""

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

        info "  Importing certificates on $pod..."
        run_sql_check "$pod" "$sql"
    done
}

# ============================================================================
# STEP 2.4: Create Database Mirroring Endpoints
# ============================================================================

step_2_4() {
    banner "Step 2.4: Create Database Mirroring Endpoints (all replicas)"

    for i in "${!REPLICAS[@]}"; do
        local pod="${REPLICAS[$i]}"

        # Build GRANT statements for the OTHER replica logins
        local grants=""
        for j in "${!REPLICAS[@]}"; do
            if [ "$i" != "$j" ]; then
                grants+="GRANT CONNECT ON ENDPOINT::AG_Endpoint TO sql_ag_${j}_login;
"
            fi
        done

        info "Creating endpoint on $pod..."
        run_sql_check "$pod" "
CREATE ENDPOINT AG_Endpoint
    STATE = STARTED
    AS TCP (
        LISTENER_PORT = $ENDPOINT_PORT,
        LISTENER_IP = ALL
    )
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

# ============================================================================
# STEP 2.5: Create Databases
# ============================================================================

step_2_5() {
    banner "Step 2.5: Create Database (primary only: $PRIMARY)"

    info "Creating $DATABASE_NAME on $PRIMARY..."
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

# ============================================================================
# STEP 2.6: Create Availability Group
# ============================================================================

step_2_6() {
    banner "Step 2.6: Create Availability Group (primary only: $PRIMARY)"

    # Build the REPLICA ON clause dynamically
    local replica_on=""
    for i in "${!REPLICAS[@]}"; do
        local pod="${REPLICAS[$i]}"
        if [ "$i" -gt 0 ]; then
            replica_on+=","
        fi
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

# ============================================================================
# STEP 2.7: Join Secondary Replicas
# ============================================================================

step_2_7() {
    banner "Step 2.7: Join Secondary Replicas"

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

# ============================================================================
# STEP 2.8: Verify AG Status
# ============================================================================

step_2_8() {
    banner "Step 2.8: Verify AG Status"

    info "Querying replica status from $PRIMARY..."
    run_sql "$PRIMARY" "
SELECT
    ag.name              AS ag_name,
    ar.replica_server_name,
    ars.role_desc,
    ars.synchronization_health_desc,
    ars.connected_state_desc
FROM sys.availability_groups ag
JOIN sys.availability_replicas ar
    ON ag.group_id = ar.group_id
JOIN sys.dm_hadr_availability_replica_states ars
    ON ar.replica_id = ars.replica_id
ORDER BY ar.replica_server_name;
GO
"
    echo ""
    info "Querying database synchronization status..."
    run_sql "$PRIMARY" "
SELECT
    ag.name              AS ag_name,
    d.name               AS database_name,
    drs.synchronization_state_desc,
    drs.is_primary_replica,
    drs.synchronization_health_desc
FROM sys.dm_hadr_database_replica_states drs
JOIN sys.databases d
    ON drs.database_id = d.database_id
JOIN sys.availability_groups ag
    ON drs.group_id = ag.group_id
ORDER BY d.name;
GO
"
    echo ""
    ok "Expected: All replicas CONNECTED + HEALTHY, databases SYNCHRONIZED"
}

# ============================================================================
# STEP 2.9 (BONUS): AG Listener Setup
# ============================================================================
# This step requires ag-step3-ag-config.yaml to be applied first.
# The operator creates a Kubernetes Service whose ClusterIP becomes the VIP.
# This function retrieves the VIP automatically and attempts to detect the
# cluster service CIDR to derive the subnet mask.  If auto-detection fails,
# it prints the exact T-SQL command with the VIP filled in so only the
# subnet mask needs to be supplied manually.

step_2_9_listener() {
    banner "Step 2.9 (Bonus): AG Listener Setup"

    # --- Check that the SQLServerAG resource exists (Step 3) ---
    if ! kubectl get sqlserverag "$AG_RESOURCE_NAME" -n "$NAMESPACE" &>/dev/null; then
        warn "SQLServerAG resource '$AG_RESOURCE_NAME' not found."
        warn "Apply ag-step3-ag-config.yaml first, then re-run:"
        echo ""
        echo "    kubectl apply -f samples/ag-step3-ag-config.yaml"
        echo "    $0 listener"
        return 0
    fi
    ok "SQLServerAG resource '$AG_RESOURCE_NAME' found"

    # --- Wait for the listener VIP to be assigned ---
    info "Waiting for listener VIP assignment (up to 60 s)..."
    local vip=""
    local waited=0
    while [ -z "$vip" ] && [ $waited -lt 60 ]; do
        vip=$(kubectl get sqlserverag "$AG_RESOURCE_NAME" -n "$NAMESPACE" \
            -o jsonpath='{.status.listener.vip}' 2>/dev/null) || true
        if [ -z "$vip" ]; then
            sleep 2
            waited=$((waited + 2))
            printf "."
        fi
    done
    echo ""

    if [ -z "$vip" ]; then
        # Fallback: try reading the ClusterIP from the Service directly
        vip=$(kubectl get svc "$AG_LISTENER_NAME" -n "$NAMESPACE" \
            -o jsonpath='{.spec.clusterIP}' 2>/dev/null) || true
    fi

    if [ -z "$vip" ]; then
        err "Could not retrieve listener VIP after 60 s."
        warn "Debug with:  kubectl describe sqlserverag $AG_RESOURCE_NAME -n $NAMESPACE"
        return 1
    fi
    ok "Listener VIP: $vip"

    # --- Detect subnet mask from cluster service CIDR ---
    info "Detecting cluster service CIDR for subnet mask..."
    local cidr="" subnet_mask=""

    # Method 1: kube-apiserver --service-cluster-ip-range
    cidr=$(kubectl get pods -n kube-system -l component=kube-apiserver \
        -o jsonpath='{.items[0].spec.containers[0].command}' 2>/dev/null \
        | tr '[],"' ' ' \
        | grep -oE 'service-cluster-ip-range=[^ ]+' \
        | head -1 | cut -d= -f2) 2>/dev/null || true

    # Method 2: kubeadm-config ConfigMap
    if [ -z "$cidr" ]; then
        cidr=$(kubectl get cm kubeadm-config -n kube-system \
            -o jsonpath='{.data.ClusterConfiguration}' 2>/dev/null \
            | sed -n 's/.*serviceSubnet:[[:space:]]*\([^ ]*\).*/\1/p') 2>/dev/null || true
    fi

    # Method 3: kube-controller-manager --service-cluster-ip-range
    if [ -z "$cidr" ]; then
        cidr=$(kubectl get pods -n kube-system -l component=kube-controller-manager \
            -o jsonpath='{.items[0].spec.containers[0].command}' 2>/dev/null \
            | tr '[],"' ' ' \
            | grep -oE 'service-cluster-ip-range=[^ ]+' \
            | head -1 | cut -d= -f2) 2>/dev/null || true
    fi

    if [ -n "$cidr" ]; then
        local prefix
        prefix=$(echo "$cidr" | cut -d/ -f2)
        subnet_mask=$(cidr_prefix_to_mask "$prefix")
        ok "Service CIDR: $cidr  →  Subnet mask: $subnet_mask"
    fi

    # --- Create the listener or print manual instructions ---
    if [ -n "$subnet_mask" ]; then
        info "Creating AG Listener via T-SQL on $PRIMARY..."
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
        echo -e "${BOLD}${YELLOW}  ┌──────────────────────────────────────────────────────────────┐${NC}"
        echo -e "${BOLD}${YELLOW}  │  MANUAL STEP: Create the AG Listener                        │${NC}"
        echo -e "${BOLD}${YELLOW}  ├──────────────────────────────────────────────────────────────┤${NC}"
        echo -e "${BOLD}${YELLOW}  │                                                              │${NC}"
        echo -e "${BOLD}${YELLOW}  │  VIP (auto-detected): ${BOLD}$vip${NC}${BOLD}${YELLOW}                                │${NC}"
        echo -e "${BOLD}${YELLOW}  │                                                              │${NC}"
        echo -e "${BOLD}${YELLOW}  │  Find your subnet mask with:                                 │${NC}"
        echo -e "${BOLD}${YELLOW}  │    kubectl get svc -A \\                                      │${NC}"
        echo -e "${BOLD}${YELLOW}  │      -o jsonpath='{.items[*].spec.clusterIP}'                 │${NC}"
        echo -e "${BOLD}${YELLOW}  │                                                              │${NC}"
        echo -e "${BOLD}${YELLOW}  │  Common masks:                                               │${NC}"
        echo -e "${BOLD}${YELLOW}  │    /12 → 255.240.0.0   /16 → 255.255.0.0                     │${NC}"
        echo -e "${BOLD}${YELLOW}  │    /20 → 255.255.240.0 /24 → 255.255.255.0                   │${NC}"
        echo -e "${BOLD}${YELLOW}  │                                                              │${NC}"
        echo -e "${BOLD}${YELLOW}  │  Run on primary ($PRIMARY):                                  │${NC}"
        echo -e "${BOLD}${YELLOW}  │                                                              │${NC}"
        echo -e "${BOLD}${YELLOW}  │  ALTER AVAILABILITY GROUP [$AG_NAME]                         │${NC}"
        echo -e "${BOLD}${YELLOW}  │  ADD LISTENER '$AG_LISTENER_NAME' (                          │${NC}"
        echo -e "${BOLD}${YELLOW}  │      WITH IP (('$vip', '<SUBNET_MASK>')),                    │${NC}"
        echo -e "${BOLD}${YELLOW}  │      PORT = $LISTENER_PORT                                   │${NC}"
        echo -e "${BOLD}${YELLOW}  │  );                                                          │${NC}"
        echo -e "${BOLD}${YELLOW}  │                                                              │${NC}"
        echo -e "${BOLD}${YELLOW}  └──────────────────────────────────────────────────────────────┘${NC}"
        echo ""
        info "After creating the listener, verify with:"
        echo "    kubectl get sqlserverag $AG_RESOURCE_NAME -n $NAMESPACE -o wide"
        return 0
    fi

    # --- Verify listener phase ---
    info "Waiting for listener to become Ready..."
    sleep 5
    local phase
    phase=$(kubectl get sqlserverag "$AG_RESOURCE_NAME" -n "$NAMESPACE" \
        -o jsonpath='{.status.listener.phase}' 2>/dev/null) || true

    if [ "$phase" = "Ready" ]; then
        ok "Listener is Ready!"
    else
        info "Listener phase: ${phase:-unknown} (may take a moment to transition to Ready)"
    fi

    echo ""
    ok "Connect via listener:"
    echo "    sqlcmd -S $vip,$LISTENER_PORT -U sa -P '<password>'"
}

# ============================================================================
# MAIN
# ============================================================================

usage() {
    echo "Usage: $0 [COMMAND]"
    echo ""
    echo "Commands:"
    echo "  all          Run all steps 2.1 – 2.8 (default)"
    echo "  listener     Set up AG Listener (requires Step 3 applied first)"
    echo "  verify       Verify AG status (alias for 2.8)"
    echo "  2.1          Create AG Helper login"
    echo "  2.2          Create master key and certificates"
    echo "  2.3          Export / import certificates"
    echo "  2.4          Create database mirroring endpoints"
    echo "  2.5          Create databases"
    echo "  2.6          Create Availability Group"
    echo "  2.7          Join secondary replicas"
    echo "  2.8          Verify AG status"
    echo ""
    echo "Examples:"
    echo "  $0                   # full setup"
    echo "  $0 listener          # listener only (after Step 3)"
    echo "  $0 2.5               # re-run just the database creation step"
}

main() {
    echo -e "${BOLD}${CYAN}"
    echo "╔══════════════════════════════════════════════════════════════╗"
    echo "║  SQL Server Availability Group — Automated Setup           ║"
    echo "║  Script version of ag-step2-setup-ag.md                    ║"
    echo "╚══════════════════════════════════════════════════════════════╝"
    echo -e "${NC}"

    local cmd="${1:-all}"

    case "$cmd" in
        all)
            check_prerequisites
            step_2_1
            step_2_2
            step_2_3
            step_2_4
            step_2_5
            step_2_6
            step_2_7
            step_2_8
            echo ""
            banner "All Steps Complete!"
            ok "Availability Group '$AG_NAME' is configured and running."
            echo ""
            info "Next steps:"
            echo "  1. Apply ag-step3-ag-config.yaml  (deploys AG Helper + services)"
            echo "  2. Run '$0 listener'              (creates the AG Listener)"
            ;;
        listener)  step_2_9_listener ;;
        verify)    step_2_8 ;;
        2.1)       step_2_1 ;;
        2.2)       step_2_2 ;;
        2.3)       step_2_3 ;;
        2.4)       step_2_4 ;;
        2.5)       step_2_5 ;;
        2.6)       step_2_6 ;;
        2.7)       step_2_7 ;;
        2.8)       step_2_8 ;;
        -h|--help|help)
            usage
            ;;
        *)
            err "Unknown command: $cmd"
            echo ""
            usage
            exit 1
            ;;
    esac
}

main "$@"
