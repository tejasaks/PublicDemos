#!/usr/bin/env bash
# ============================================================================
# ag-configure.sh — Multi-AG T-SQL configuration (3 AGs, 3 databases)
# ============================================================================
#
# Automates the T-SQL steps in ag-configure.md for the Multi-AG scenario.
#
# Usage:
#   ./ag-configure.sh all              # Run all steps (1-8)
#   ./ag-configure.sh verify           # Run step 8 only (verify AGs)
#   ./ag-configure.sh <step>           # Run a single step (1-8)
#
# Prerequisites:
#   - kubectl configured with target cluster
#   - ag-deploy.yaml already applied
#   - All 3 pods Running and Ready
# ============================================================================

set -euo pipefail

# --- Configuration ---
NAMESPACE="mssql"
INSTANCE_NAME="sql-ag"
REPLICA_COUNT=3
SA_PASSWORD="YourStrong@Passw0rd!"
AG_HELPER_USER="ag_helper"
AG_HELPER_PASSWORD="AGHelper@Passw0rd!"
MASTER_KEY_PASSWORD="MasterKey@Passw0rd!"
CERT_KEY_PASSWORD="CertKey@Passw0rd!"

# AG definitions: NAME|DATABASE|SYNC_MODE|REPLICAS
# REPLICAS is a comma-separated list of "replica:mode:failover:session_timeout:readable:seeding"
declare -A AG_CONFIGS
AG_CONFIGS[ProductionAG]="AppDB"
AG_CONFIGS[ReportingAG]="ReportingDB"
AG_CONFIGS[DisasterRecoveryAG]="CriticalDB"

AG_NAMES=("ProductionAG" "ReportingAG" "DisasterRecoveryAG")
AG_DATABASES=("AppDB" "ReportingDB" "CriticalDB")

# --- Colors ---
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

log_info()  { echo -e "${BLUE}[INFO]${NC}  $*"; }
log_ok()    { echo -e "${GREEN}[OK]${NC}    $*"; }
log_warn()  { echo -e "${YELLOW}[WARN]${NC}  $*"; }
log_error() { echo -e "${RED}[ERROR]${NC} $*"; }

# --- Helper: run T-SQL on a pod ---
run_sql() {
    local pod="$1"
    shift
    local sql="$*"
    kubectl -n "${NAMESPACE}" exec "${pod}" -- \
        /opt/mssql-tools18/bin/sqlcmd \
        -S localhost -U sa -P "${SA_PASSWORD}" \
        -C -Q "${sql}" 2>&1
}

# --- Prerequisite checks ---
check_prerequisites() {
    log_info "Checking prerequisites..."

    if ! command -v kubectl &>/dev/null; then
        log_error "kubectl not found in PATH"
        exit 1
    fi

    for i in $(seq 0 $((REPLICA_COUNT - 1))); do
        local pod="${INSTANCE_NAME}-${i}"
        local phase
        phase=$(kubectl -n "${NAMESPACE}" get pod "${pod}" -o jsonpath='{.status.phase}' 2>/dev/null || echo "NotFound")
        if [[ "${phase}" != "Running" ]]; then
            log_error "Pod ${pod} is ${phase} (expected Running)"
            exit 1
        fi
    done

    log_ok "All ${REPLICA_COUNT} pods are Running"
}

# === STEP 1: Create AG Helper Login ===
step_1() {
    log_info "Step 1: Creating AG Helper login on all replicas..."
    for i in $(seq 0 $((REPLICA_COUNT - 1))); do
        local pod="${INSTANCE_NAME}-${i}"
        run_sql "${pod}" "
            IF NOT EXISTS (SELECT 1 FROM sys.server_principals WHERE name = '${AG_HELPER_USER}')
            BEGIN
                CREATE LOGIN [${AG_HELPER_USER}]
                    WITH PASSWORD = N'${AG_HELPER_PASSWORD}',
                         CHECK_POLICY = OFF,
                         CHECK_EXPIRATION = OFF;
            END;
            IF NOT IS_SRVROLEMEMBER('sysadmin', '${AG_HELPER_USER}') = 1
                ALTER SERVER ROLE [sysadmin] ADD MEMBER [${AG_HELPER_USER}];
        " >/dev/null
        log_ok "  ${pod}: ag_helper login ready"
    done
}

# === STEP 2: Create Master Keys ===
step_2() {
    log_info "Step 2: Creating master keys on all replicas..."
    for i in $(seq 0 $((REPLICA_COUNT - 1))); do
        local pod="${INSTANCE_NAME}-${i}"
        run_sql "${pod}" "
            IF NOT EXISTS (SELECT 1 FROM sys.symmetric_keys WHERE name = '##MS_DatabaseMasterKey##')
                CREATE MASTER KEY ENCRYPTION BY PASSWORD = N'${MASTER_KEY_PASSWORD}';
        " >/dev/null
        log_ok "  ${pod}: master key ready"
    done
}

# === STEP 3: Create and Distribute Certificates ===
step_3() {
    log_info "Step 3: Creating and distributing certificates..."

    local primary="${INSTANCE_NAME}-0"

    # Create cert on primary
    run_sql "${primary}" "
        IF NOT EXISTS (SELECT 1 FROM sys.certificates WHERE name = 'ag_cert')
        BEGIN
            CREATE CERTIFICATE ag_cert
                WITH SUBJECT = 'AG Endpoint Certificate',
                     EXPIRY_DATE = '20301231';

            BACKUP CERTIFICATE ag_cert
                TO FILE = '/var/opt/mssql/data/ag_cert.cer'
                WITH PRIVATE KEY (
                    FILE = '/var/opt/mssql/data/ag_cert.key',
                    ENCRYPTION BY PASSWORD = N'${CERT_KEY_PASSWORD}'
                );
        END;
    " >/dev/null
    log_ok "  ${primary}: certificate created and backed up"

    # Copy cert to secondaries
    local tmpdir
    tmpdir=$(mktemp -d)
    kubectl -n "${NAMESPACE}" cp "${primary}:/var/opt/mssql/data/ag_cert.cer" "${tmpdir}/ag_cert.cer"
    kubectl -n "${NAMESPACE}" cp "${primary}:/var/opt/mssql/data/ag_cert.key" "${tmpdir}/ag_cert.key"

    for i in $(seq 1 $((REPLICA_COUNT - 1))); do
        local pod="${INSTANCE_NAME}-${i}"
        kubectl -n "${NAMESPACE}" cp "${tmpdir}/ag_cert.cer" "${pod}:/var/opt/mssql/data/ag_cert.cer"
        kubectl -n "${NAMESPACE}" cp "${tmpdir}/ag_cert.key" "${pod}:/var/opt/mssql/data/ag_cert.key"

        run_sql "${pod}" "
            IF NOT EXISTS (SELECT 1 FROM sys.certificates WHERE name = 'ag_cert')
                CREATE CERTIFICATE ag_cert
                    FROM FILE = '/var/opt/mssql/data/ag_cert.cer'
                    WITH PRIVATE KEY (
                        FILE = '/var/opt/mssql/data/ag_cert.key',
                        DECRYPTION BY PASSWORD = N'${CERT_KEY_PASSWORD}'
                    );
        " >/dev/null
        log_ok "  ${pod}: certificate imported"
    done

    rm -rf "${tmpdir}"
}

# === STEP 4: Create AG Endpoints ===
step_4() {
    log_info "Step 4: Creating AG endpoints on all replicas..."
    for i in $(seq 0 $((REPLICA_COUNT - 1))); do
        local pod="${INSTANCE_NAME}-${i}"
        run_sql "${pod}" "
            IF NOT EXISTS (SELECT 1 FROM sys.endpoints WHERE name = 'ag_endpoint')
                CREATE ENDPOINT [ag_endpoint]
                    STATE = STARTED
                    AS TCP (LISTENER_PORT = 5022)
                    FOR DATABASE_MIRRORING (
                        ROLE = ALL,
                        AUTHENTICATION = CERTIFICATE ag_cert,
                        ENCRYPTION = REQUIRED ALGORITHM AES
                    );
        " >/dev/null
        log_ok "  ${pod}: endpoint ready"
    done
}

# === STEP 5: Create Databases ===
step_5() {
    log_info "Step 5: Creating databases on primary..."
    local primary="${INSTANCE_NAME}-0"

    for idx in "${!AG_NAMES[@]}"; do
        local db="${AG_DATABASES[$idx]}"
        run_sql "${primary}" "
            IF NOT EXISTS (SELECT 1 FROM sys.databases WHERE name = '${db}')
            BEGIN
                CREATE DATABASE [${db}];
                ALTER DATABASE [${db}] SET RECOVERY FULL;
                BACKUP DATABASE [${db}] TO DISK = '/var/opt/mssql/backup/${db}.bak';
            END;
        " >/dev/null
        log_ok "  ${db}: created and backed up"
    done
}

# === STEP 6: Create Availability Groups ===
step_6() {
    log_info "Step 6: Creating Availability Groups on primary..."
    local primary="${INSTANCE_NAME}-0"
    local domain="${NAMESPACE}.svc.cluster.local"

    # 6a — ProductionAG: 3 sync, auto failover, automatic seeding
    run_sql "${primary}" "
        IF NOT EXISTS (SELECT 1 FROM sys.availability_groups WHERE name = 'ProductionAG')
            CREATE AVAILABILITY GROUP [ProductionAG]
                WITH (CLUSTER_TYPE = EXTERNAL, DB_FAILOVER = ON, AUTOMATED_BACKUP_PREFERENCE = SECONDARY)
                FOR DATABASE [AppDB]
                REPLICA ON
                    N'${INSTANCE_NAME}-0' WITH (
                        ENDPOINT_URL = N'TCP://${INSTANCE_NAME}-0.${domain}:5022',
                        AVAILABILITY_MODE = SYNCHRONOUS_COMMIT,
                        FAILOVER_MODE = EXTERNAL,
                        SEEDING_MODE = AUTOMATIC,
                        SESSION_TIMEOUT = 10,
                        SECONDARY_ROLE (ALLOW_CONNECTIONS = READ_ONLY)),
                    N'${INSTANCE_NAME}-1' WITH (
                        ENDPOINT_URL = N'TCP://${INSTANCE_NAME}-1.${domain}:5022',
                        AVAILABILITY_MODE = SYNCHRONOUS_COMMIT,
                        FAILOVER_MODE = EXTERNAL,
                        SEEDING_MODE = AUTOMATIC,
                        SESSION_TIMEOUT = 10,
                        SECONDARY_ROLE (ALLOW_CONNECTIONS = READ_ONLY)),
                    N'${INSTANCE_NAME}-2' WITH (
                        ENDPOINT_URL = N'TCP://${INSTANCE_NAME}-2.${domain}:5022',
                        AVAILABILITY_MODE = SYNCHRONOUS_COMMIT,
                        FAILOVER_MODE = EXTERNAL,
                        SEEDING_MODE = AUTOMATIC,
                        SESSION_TIMEOUT = 10,
                        SECONDARY_ROLE (ALLOW_CONNECTIONS = READ_ONLY));
    " >/dev/null
    log_ok "  ProductionAG created (3 sync, auto failover)"

    # 6b — ReportingAG: 1 sync + 2 async, manual failover
    run_sql "${primary}" "
        IF NOT EXISTS (SELECT 1 FROM sys.availability_groups WHERE name = 'ReportingAG')
            CREATE AVAILABILITY GROUP [ReportingAG]
                WITH (CLUSTER_TYPE = EXTERNAL, DB_FAILOVER = OFF, AUTOMATED_BACKUP_PREFERENCE = SECONDARY)
                FOR DATABASE [ReportingDB]
                REPLICA ON
                    N'${INSTANCE_NAME}-0' WITH (
                        ENDPOINT_URL = N'TCP://${INSTANCE_NAME}-0.${domain}:5022',
                        AVAILABILITY_MODE = SYNCHRONOUS_COMMIT,
                        FAILOVER_MODE = EXTERNAL,
                        SEEDING_MODE = AUTOMATIC,
                        SESSION_TIMEOUT = 10,
                        SECONDARY_ROLE (ALLOW_CONNECTIONS = NO)),
                    N'${INSTANCE_NAME}-1' WITH (
                        ENDPOINT_URL = N'TCP://${INSTANCE_NAME}-1.${domain}:5022',
                        AVAILABILITY_MODE = ASYNCHRONOUS_COMMIT,
                        FAILOVER_MODE = MANUAL,
                        SEEDING_MODE = AUTOMATIC,
                        SESSION_TIMEOUT = 30,
                        SECONDARY_ROLE (ALLOW_CONNECTIONS = READ_ONLY)),
                    N'${INSTANCE_NAME}-2' WITH (
                        ENDPOINT_URL = N'TCP://${INSTANCE_NAME}-2.${domain}:5022',
                        AVAILABILITY_MODE = ASYNCHRONOUS_COMMIT,
                        FAILOVER_MODE = MANUAL,
                        SEEDING_MODE = AUTOMATIC,
                        SESSION_TIMEOUT = 30,
                        SECONDARY_ROLE (ALLOW_CONNECTIONS = READ_ONLY));
    " >/dev/null
    log_ok "  ReportingAG created (1 sync + 2 async)"

    # 6c — DisasterRecoveryAG: 1 sync + 1 async, manual seeding, 2 replicas only
    run_sql "${primary}" "
        IF NOT EXISTS (SELECT 1 FROM sys.availability_groups WHERE name = 'DisasterRecoveryAG')
            CREATE AVAILABILITY GROUP [DisasterRecoveryAG]
                WITH (CLUSTER_TYPE = EXTERNAL, DB_FAILOVER = OFF, AUTOMATED_BACKUP_PREFERENCE = PRIMARY)
                FOR DATABASE [CriticalDB]
                REPLICA ON
                    N'${INSTANCE_NAME}-0' WITH (
                        ENDPOINT_URL = N'TCP://${INSTANCE_NAME}-0.${domain}:5022',
                        AVAILABILITY_MODE = SYNCHRONOUS_COMMIT,
                        FAILOVER_MODE = EXTERNAL,
                        SEEDING_MODE = MANUAL,
                        SESSION_TIMEOUT = 10,
                        SECONDARY_ROLE (ALLOW_CONNECTIONS = NO)),
                    N'${INSTANCE_NAME}-1' WITH (
                        ENDPOINT_URL = N'TCP://${INSTANCE_NAME}-1.${domain}:5022',
                        AVAILABILITY_MODE = ASYNCHRONOUS_COMMIT,
                        FAILOVER_MODE = MANUAL,
                        SEEDING_MODE = MANUAL,
                        SESSION_TIMEOUT = 60,
                        SECONDARY_ROLE (ALLOW_CONNECTIONS = NO));
    " >/dev/null
    log_ok "  DisasterRecoveryAG created (1 sync + 1 async, manual seeding)"
}

# === STEP 7: Join Secondaries ===
step_7() {
    log_info "Step 7: Joining secondary replicas..."

    # sql-ag-1 joins all 3 AGs
    local pod1="${INSTANCE_NAME}-1"
    run_sql "${pod1}" "
        ALTER AVAILABILITY GROUP [ProductionAG] JOIN WITH (CLUSTER_TYPE = EXTERNAL);
        ALTER AVAILABILITY GROUP [ProductionAG] GRANT CREATE ANY DATABASE;
    " >/dev/null
    log_ok "  ${pod1}: joined ProductionAG"

    run_sql "${pod1}" "
        ALTER AVAILABILITY GROUP [ReportingAG] JOIN WITH (CLUSTER_TYPE = EXTERNAL);
        ALTER AVAILABILITY GROUP [ReportingAG] GRANT CREATE ANY DATABASE;
    " >/dev/null
    log_ok "  ${pod1}: joined ReportingAG"

    run_sql "${pod1}" "
        ALTER AVAILABILITY GROUP [DisasterRecoveryAG] JOIN WITH (CLUSTER_TYPE = EXTERNAL);
    " >/dev/null
    log_ok "  ${pod1}: joined DisasterRecoveryAG"

    # sql-ag-2 joins ProductionAG and ReportingAG only (not DR)
    local pod2="${INSTANCE_NAME}-2"
    run_sql "${pod2}" "
        ALTER AVAILABILITY GROUP [ProductionAG] JOIN WITH (CLUSTER_TYPE = EXTERNAL);
        ALTER AVAILABILITY GROUP [ProductionAG] GRANT CREATE ANY DATABASE;
    " >/dev/null
    log_ok "  ${pod2}: joined ProductionAG"

    run_sql "${pod2}" "
        ALTER AVAILABILITY GROUP [ReportingAG] JOIN WITH (CLUSTER_TYPE = EXTERNAL);
        ALTER AVAILABILITY GROUP [ReportingAG] GRANT CREATE ANY DATABASE;
    " >/dev/null
    log_ok "  ${pod2}: joined ReportingAG"

    # Manual restore for DR AG on sql-ag-1
    log_info "  Restoring CriticalDB on ${pod1} (manual seeding)..."
    local tmpdir
    tmpdir=$(mktemp -d)
    local primary="${INSTANCE_NAME}-0"

    kubectl -n "${NAMESPACE}" cp "${primary}:/var/opt/mssql/backup/CriticalDB.bak" "${tmpdir}/CriticalDB.bak"
    kubectl -n "${NAMESPACE}" cp "${tmpdir}/CriticalDB.bak" "${pod1}:/var/opt/mssql/backup/CriticalDB.bak"

    run_sql "${pod1}" "
        IF NOT EXISTS (SELECT 1 FROM sys.databases WHERE name = 'CriticalDB' AND state = 0)
        BEGIN
            RESTORE DATABASE [CriticalDB]
                FROM DISK = '/var/opt/mssql/backup/CriticalDB.bak'
                WITH NORECOVERY, REPLACE;

            RESTORE LOG [CriticalDB]
                FROM DISK = '/var/opt/mssql/backup/CriticalDB.bak'
                WITH NORECOVERY;

            ALTER DATABASE [CriticalDB] SET HADR AVAILABILITY GROUP = [DisasterRecoveryAG];
        END;
    " >/dev/null
    log_ok "  ${pod1}: CriticalDB restored and joined to DisasterRecoveryAG"

    rm -rf "${tmpdir}"
}

# === STEP 8: Verify All AGs ===
step_8() {
    log_info "Step 8: Verifying all Availability Groups..."
    local primary="${INSTANCE_NAME}-0"

    echo ""
    run_sql "${primary}" "
        SELECT
            ag.name AS ag_name,
            rs.role_desc,
            rs.connected_state_desc,
            rs.synchronization_health_desc,
            db.database_name,
            dbs.synchronization_state_desc
        FROM sys.dm_hadr_availability_replica_states rs
        JOIN sys.availability_groups ag ON rs.group_id = ag.group_id
        LEFT JOIN sys.dm_hadr_database_replica_states dbs ON rs.replica_id = dbs.replica_id
        LEFT JOIN sys.availability_databases_cluster db ON dbs.group_database_id = db.group_database_id
        ORDER BY ag.name, rs.role_desc;
    "
    echo ""

    # Count healthy AGs
    local ag_count
    ag_count=$(run_sql "${primary}" "
        SET NOCOUNT ON;
        SELECT COUNT(DISTINCT name) FROM sys.availability_groups;
    " | tr -d '[:space:]')

    if [[ "${ag_count}" -ge 3 ]]; then
        log_ok "All 3 Availability Groups are present"
    else
        log_warn "Expected 3 AGs, found ${ag_count}"
    fi
}

# === Main ===
usage() {
    echo "Usage: $0 [all|verify|1-8]"
    echo ""
    echo "Commands:"
    echo "  all     Run all steps (1-8)"
    echo "  verify  Run step 8 only (verify AGs)"
    echo "  1-8     Run a specific step"
    echo ""
    echo "Steps:"
    echo "  1  Create AG Helper login"
    echo "  2  Create master keys"
    echo "  3  Create and distribute certificates"
    echo "  4  Create AG endpoints"
    echo "  5  Create databases (AppDB, ReportingDB, CriticalDB)"
    echo "  6  Create Availability Groups (3 AGs)"
    echo "  7  Join secondaries + manual DR restore"
    echo "  8  Verify all AGs"
    exit 1
}

main() {
    local cmd="${1:-}"

    if [[ -z "${cmd}" ]]; then
        usage
    fi

    check_prerequisites

    case "${cmd}" in
        all)
            step_1; step_2; step_3; step_4
            step_5; step_6; step_7; step_8
            echo ""
            log_ok "All steps completed successfully!"
            echo ""
            echo "Service endpoints:"
            echo "  OLTP writes       → production-ag-primary:1433"
            echo "  OLTP reads        → production-ag-secondary:1434"
            echo "  Reporting queries  → reporting-ag-secondary:2434"
            echo "  DR (internal)     → dr-ag-primary:3433"
            ;;
        verify) step_8 ;;
        [1-8])  eval "step_${cmd}" ;;
        *)      usage ;;
    esac
}

main "$@"
