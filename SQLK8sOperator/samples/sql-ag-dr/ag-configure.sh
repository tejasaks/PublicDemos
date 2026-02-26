#!/usr/bin/env bash
# ============================================================================
# ag-configure.sh — Automated T-SQL Setup for DR Availability Group
# ============================================================================
# Creates DisasterRecoveryAG: 2 sync-commit + 1 async-commit (geo-DR).
# Manual failover only — no listener.
#
# USAGE:
#   ./ag-configure.sh              # Run all steps (1-8)
#   ./ag-configure.sh verify       # Verify AG status
#   ./ag-configure.sh 6            # Run a single step
# ============================================================================

set -uo pipefail

NAMESPACE="mssql"
PRIMARY="sql-ag-0"
REPLICAS=("sql-ag-0" "sql-ag-1" "sql-ag-2")
SECONDARIES=("sql-ag-1" "sql-ag-2")
CONTAINER="mssql"

SA_PASSWORD='YourStrong@Passw0rd!'
AG_HELPER_PASSWORD='AGHelper@Passw0rd!'
MASTER_KEY_PASSWORD='MasterKey@Passw0rd!'
REPLICA_LOGIN_PASSWORD='ReplicaLogin@Passw0rd!'

AG_NAME="DisasterRecoveryAG"
AG_RESOURCE_NAME="dr-ag"
DATABASE_NAME="CriticalDB"
ENDPOINT_PORT=5022

# ============================================================================
# HELPERS
# ============================================================================

RED='\033[0;31m'; GREEN='\033[0;32m'; YELLOW='\033[1;33m'
CYAN='\033[0;36m'; BOLD='\033[1m'; NC='\033[0m'

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
    else err "Failed on $pod"; return 1; fi
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
            err "Pod $pod not Running. Apply ag-deploy.yaml first."; exit 1
        fi
        ok "Pod $pod Running"
    done
}

# ============================================================================
# STEPS 1-4: Identical infrastructure (login, certs, endpoints)
# ============================================================================

step_1() {
    banner "Step 1: Create AG Helper Login (all replicas)"
    for pod in "${REPLICAS[@]}"; do
        info "Creating on $pod..."
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
    banner "Step 2: Create Master Key and Certificates"
    for i in "${!REPLICAS[@]}"; do
        local pod="${REPLICAS[$i]}"
        info "Creating on $pod..."
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
    info "3.1 — Backing up..."
    for i in "${!REPLICAS[@]}"; do
        run_sql_check "${REPLICAS[$i]}" "
BACKUP CERTIFICATE AG_Cert_${i} TO FILE = '/var/opt/mssql/data/AG_Cert_${i}.cer';
GO
"
    done

    info "3.2 — Exchanging files..."
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
    rm -rf "$tmpdir"; ok "Exchange complete"

    info "3.3 — Importing..."
    for i in "${!REPLICAS[@]}"; do
        local pod="${REPLICAS[$i]}" sql=""
        for j in "${!REPLICAS[@]}"; do
            if [ "$i" != "$j" ]; then
                sql+="
CREATE LOGIN sql_ag_${j}_login WITH PASSWORD = '$REPLICA_LOGIN_PASSWORD';
CREATE USER sql_ag_${j}_user FOR LOGIN sql_ag_${j}_login;
CREATE CERTIFICATE AG_Cert_${j} AUTHORIZATION sql_ag_${j}_user
    FROM FILE = '/var/opt/mssql/data/AG_Cert_${j}.cer';
GO
"
            fi
        done
        run_sql_check "$pod" "$sql"
    done
}

step_4() {
    banner "Step 4: Create Database Mirroring Endpoints"
    for i in "${!REPLICAS[@]}"; do
        local pod="${REPLICAS[$i]}" grants=""
        for j in "${!REPLICAS[@]}"; do
            [ "$i" != "$j" ] && grants+="GRANT CONNECT ON ENDPOINT::AG_Endpoint TO sql_ag_${j}_login;
"
        done
        info "Creating on $pod..."
        run_sql_check "$pod" "
CREATE ENDPOINT AG_Endpoint STATE = STARTED
    AS TCP (LISTENER_PORT = $ENDPOINT_PORT, LISTENER_IP = ALL)
    FOR DATABASE_MIRRORING (
        ROLE = ALL, AUTHENTICATION = CERTIFICATE AG_Cert_${i},
        ENCRYPTION = REQUIRED ALGORITHM AES);
GO
${grants}
GO
"
    done
}

# ============================================================================
# STEPS 5-8: DR-specific (different AG name, async replica)
# ============================================================================

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
    banner "Step 6: Create DR Availability Group (primary: $PRIMARY)"
    info "Creating $AG_NAME (2 sync + 1 async)..."
    run_sql_check "$PRIMARY" "
CREATE AVAILABILITY GROUP [$AG_NAME]
    WITH (
        CLUSTER_TYPE = EXTERNAL,
        DB_FAILOVER = ON,
        REQUIRED_SYNCHRONIZED_SECONDARIES_TO_COMMIT = 1
    )
    FOR DATABASE [$DATABASE_NAME]
    REPLICA ON
        N'sql-ag-0' WITH (
            ENDPOINT_URL = N'TCP://sql-ag-0.${NAMESPACE}.svc.cluster.local:${ENDPOINT_PORT}',
            AVAILABILITY_MODE = SYNCHRONOUS_COMMIT,
            FAILOVER_MODE = EXTERNAL,
            SEEDING_MODE = AUTOMATIC,
            SESSION_TIMEOUT = 10,
            SECONDARY_ROLE (ALLOW_CONNECTIONS = READ_ONLY)
        ),
        N'sql-ag-1' WITH (
            ENDPOINT_URL = N'TCP://sql-ag-1.${NAMESPACE}.svc.cluster.local:${ENDPOINT_PORT}',
            AVAILABILITY_MODE = SYNCHRONOUS_COMMIT,
            FAILOVER_MODE = EXTERNAL,
            SEEDING_MODE = AUTOMATIC,
            SESSION_TIMEOUT = 10,
            SECONDARY_ROLE (ALLOW_CONNECTIONS = READ_ONLY)
        ),
        N'sql-ag-2' WITH (
            ENDPOINT_URL = N'TCP://sql-ag-2.${NAMESPACE}.svc.cluster.local:${ENDPOINT_PORT}',
            AVAILABILITY_MODE = ASYNCHRONOUS_COMMIT,
            FAILOVER_MODE = MANUAL,
            SEEDING_MODE = AUTOMATIC,
            SESSION_TIMEOUT = 30,
            SECONDARY_ROLE (ALLOW_CONNECTIONS = NO)
        );
GO
"
}

step_7() {
    banner "Step 7: Join Secondary Replicas"
    for pod in "${SECONDARIES[@]}"; do
        info "Joining $pod..."
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
    ok "Expected: sql-ag-0/1 SYNCHRONIZED, sql-ag-2 SYNCHRONIZING (async)"
}

# ============================================================================
# MAIN
# ============================================================================

main() {
    echo -e "${BOLD}${CYAN}"
    echo "╔══════════════════════════════════════════════════════════════╗"
    echo "║  SQL Server AG — DR Configuration                          ║"
    echo "╚══════════════════════════════════════════════════════════════╝"
    echo -e "${NC}"

    local cmd="${1:-all}"
    case "$cmd" in
        all)
            check_prerequisites; step_1; step_2; step_3; step_4
            step_5; step_6; step_7; step_8
            echo ""
            banner "All Steps Complete!"
            ok "$AG_NAME configured with 2 sync + 1 async replica."
            info "Manual failover: kubectl annotate sqlserverag $AG_RESOURCE_NAME -n $NAMESPACE mssql.microsoft.com/failover-to=sql-ag-1"
            ;;
        verify)  step_8 ;;
        [1-8])   "step_$cmd" ;;
        -h|--help) echo "Usage: $0 [all|verify|1-8]" ;;
        *)       err "Unknown: $cmd"; exit 1 ;;
    esac
}

main "$@"
