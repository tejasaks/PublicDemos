#!/bin/bash
# Test helper functions for MSSQL Operator tests

set -o pipefail

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

# Globals
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"
TEST_NAME=""
TEST_NAMESPACE=""
TEST_PASSED=true
TEST_START_TIME=""
RESOURCES_CREATED=()

# Configuration
OPERATOR_NAMESPACE="${OPERATOR_NAMESPACE:-mssql-system}"
OPERATOR_IMAGE="${OPERATOR_IMAGE:-mssql-operator:dev}"
SIDECAR_IMAGE="${SIDECAR_IMAGE:-mssql-ag-helper:dev}"
SKIP_CLEANUP="${SKIP_CLEANUP:-false}"
TIMEOUT="${TIMEOUT:-300}"

# Logging functions
log_info() {
    echo -e "${BLUE}[INFO]${NC} $(date '+%H:%M:%S') $1"
}

log_success() {
    echo -e "${GREEN}[PASS]${NC} $(date '+%H:%M:%S') $1"
}

log_warn() {
    echo -e "${YELLOW}[WARN]${NC} $(date '+%H:%M:%S') $1"
}

log_error() {
    echo -e "${RED}[FAIL]${NC} $(date '+%H:%M:%S') $1"
    TEST_PASSED=false
}

log_step() {
    echo -e "\n${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
    echo -e "${BLUE}▶ $1${NC}"
    echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}\n"
}

# Initialize test
test_init() {
    TEST_NAME="$1"
    TEST_START_TIME=$(date +%s)
    TEST_NAMESPACE="${TEST_NAMESPACE:-mssql-test-$(date +%s)}"
    
    echo -e "\n${GREEN}╔════════════════════════════════════════════════════════════╗${NC}"
    echo -e "${GREEN}║  MSSQL Operator Test: ${TEST_NAME}${NC}"
    echo -e "${GREEN}║  Namespace: ${TEST_NAMESPACE}${NC}"
    echo -e "${GREEN}║  Started: $(date)${NC}"
    echo -e "${GREEN}╚════════════════════════════════════════════════════════════╝${NC}\n"
    
    # Create log directory
    mkdir -p "${SCRIPT_DIR}/logs"
    
    # Set up trap for cleanup on exit
    trap 'test_cleanup; test_result' EXIT
}

# Create namespace
create_test_namespace() {
    local ns="${1:-$TEST_NAMESPACE}"
    log_info "Creating namespace: ${ns}"
    kubectl create namespace "${ns}" --dry-run=client -o yaml | kubectl apply -f -
    RESOURCES_CREATED+=("namespace/${ns}")
}

# Delete namespace
delete_namespace() {
    local ns="$1"
    if kubectl get namespace "${ns}" &>/dev/null; then
        log_info "Deleting namespace: ${ns}"
        kubectl delete namespace "${ns}" --timeout=120s --wait=true 2>/dev/null || true
    fi
}

# Wait for condition
wait_for() {
    local resource="$1"
    local condition="$2"
    local timeout="${3:-$TIMEOUT}"
    local namespace="${4:-$TEST_NAMESPACE}"
    
    log_info "Waiting for ${resource} to be ${condition} (timeout: ${timeout}s)"
    
    if kubectl wait --for="${condition}" "${resource}" \
        -n "${namespace}" --timeout="${timeout}s" 2>/dev/null; then
        log_success "${resource} is ${condition}"
        return 0
    else
        log_error "${resource} did not reach ${condition} within ${timeout}s"
        return 1
    fi
}

# Wait for pod ready
wait_for_pod_ready() {
    local label="$1"
    local namespace="${2:-$TEST_NAMESPACE}"
    local timeout="${3:-$TIMEOUT}"
    local count="${4:-1}"
    
    log_info "Waiting for ${count} pod(s) with label ${label} to be ready"
    
    local end_time=$(($(date +%s) + timeout))
    while [[ $(date +%s) -lt $end_time ]]; do
        local ready_count=$(kubectl get pods -n "${namespace}" -l "${label}" \
            -o jsonpath='{.items[*].status.conditions[?(@.type=="Ready")].status}' 2>/dev/null \
            | grep -o "True" | wc -l)
        
        if [[ $ready_count -ge $count ]]; then
            log_success "${ready_count} pod(s) ready"
            return 0
        fi
        
        sleep 5
    done
    
    log_error "Pods did not become ready within ${timeout}s"
    kubectl get pods -n "${namespace}" -l "${label}" -o wide
    return 1
}

# Wait for SQL Server to accept connections
wait_for_sql_ready() {
    local pod="$1"
    local namespace="${2:-$TEST_NAMESPACE}"
    local timeout="${3:-180}"
    local password="$4"
    
    log_info "Waiting for SQL Server in ${pod} to accept connections"
    
    local end_time=$(($(date +%s) + timeout))
    while [[ $(date +%s) -lt $end_time ]]; do
        if kubectl exec -n "${namespace}" "${pod}" -c mssql -- \
            /opt/mssql-tools/bin/sqlcmd -S localhost -U sa -P "${password}" \
            -Q "SELECT 1" &>/dev/null; then
            log_success "SQL Server is accepting connections"
            return 0
        fi
        sleep 10
    done
    
    log_error "SQL Server did not become ready within ${timeout}s"
    return 1
}

# Apply YAML and track resources
apply_yaml() {
    local yaml_file="$1"
    local namespace="${2:-$TEST_NAMESPACE}"
    
    log_info "Applying ${yaml_file}"
    if kubectl apply -f "${yaml_file}" -n "${namespace}"; then
        log_success "Applied ${yaml_file}"
        return 0
    else
        log_error "Failed to apply ${yaml_file}"
        return 1
    fi
}

# Create secret
create_secret() {
    local name="$1"
    local key="$2"
    local value="$3"
    local namespace="${4:-$TEST_NAMESPACE}"
    
    log_info "Creating secret: ${name}"
    kubectl create secret generic "${name}" \
        --from-literal="${key}=${value}" \
        -n "${namespace}" --dry-run=client -o yaml | kubectl apply -f -
}

# Assert condition
assert_equals() {
    local expected="$1"
    local actual="$2"
    local message="${3:-Values should be equal}"
    
    if [[ "${expected}" == "${actual}" ]]; then
        log_success "${message}: ${actual}"
        return 0
    else
        log_error "${message}: expected '${expected}', got '${actual}'"
        return 1
    fi
}

assert_not_empty() {
    local value="$1"
    local message="${2:-Value should not be empty}"
    
    if [[ -n "${value}" ]]; then
        log_success "${message}"
        return 0
    else
        log_error "${message}: value is empty"
        return 1
    fi
}

assert_contains() {
    local haystack="$1"
    local needle="$2"
    local message="${3:-Should contain value}"
    
    if [[ "${haystack}" == *"${needle}"* ]]; then
        log_success "${message}"
        return 0
    else
        log_error "${message}: '${needle}' not found in output"
        return 1
    fi
}

# Get resource field
get_field() {
    local resource="$1"
    local jsonpath="$2"
    local namespace="${3:-$TEST_NAMESPACE}"
    
    kubectl get "${resource}" -n "${namespace}" -o jsonpath="${jsonpath}" 2>/dev/null
}

# Check if resource exists
resource_exists() {
    local resource="$1"
    local namespace="${2:-$TEST_NAMESPACE}"
    
    kubectl get "${resource}" -n "${namespace}" &>/dev/null
}

# Delete all PVCs in namespace
delete_pvcs() {
    local namespace="$1"
    log_info "Deleting all PVCs in namespace ${namespace}"
    kubectl delete pvc --all -n "${namespace}" --timeout=60s 2>/dev/null || true
}

# Delete all SQLServer resources
delete_sqlservers() {
    local namespace="$1"
    log_info "Deleting all SQLServer resources in namespace ${namespace}"
    kubectl delete sqlserver --all -n "${namespace}" --timeout=120s 2>/dev/null || true
}

# Delete all SQLServerAG resources
delete_sqlserverags() {
    local namespace="$1"
    log_info "Deleting all SQLServerAG resources in namespace ${namespace}"
    kubectl delete sqlserverag --all -n "${namespace}" --timeout=120s 2>/dev/null || true
}

# Comprehensive cleanup
test_cleanup() {
    if [[ "${SKIP_CLEANUP}" == "true" ]]; then
        log_warn "Skipping cleanup (SKIP_CLEANUP=true)"
        log_warn "Resources remain in namespace: ${TEST_NAMESPACE}"
        return
    fi
    
    log_step "Cleaning Up Test Resources"
    
    # Delete custom resources first
    if [[ -n "${TEST_NAMESPACE}" ]]; then
        delete_sqlserverags "${TEST_NAMESPACE}"
        delete_sqlservers "${TEST_NAMESPACE}"
        
        # Wait for StatefulSets to be deleted
        sleep 5
        
        # Delete PVCs
        delete_pvcs "${TEST_NAMESPACE}"
        
        # Delete secrets and configmaps
        kubectl delete secret --all -n "${TEST_NAMESPACE}" 2>/dev/null || true
        kubectl delete configmap --all -n "${TEST_NAMESPACE}" 2>/dev/null || true
        
        # Delete the namespace
        delete_namespace "${TEST_NAMESPACE}"
    fi
    
    log_info "Cleanup complete"
}

# Print test result
test_result() {
    local end_time=$(date +%s)
    local duration=$((end_time - TEST_START_TIME))
    
    echo -e "\n"
    if [[ "${TEST_PASSED}" == "true" ]]; then
        echo -e "${GREEN}╔════════════════════════════════════════════════════════════╗${NC}"
        echo -e "${GREEN}║  ✓ TEST PASSED: ${TEST_NAME}${NC}"
        echo -e "${GREEN}║  Duration: ${duration}s${NC}"
        echo -e "${GREEN}╚════════════════════════════════════════════════════════════╝${NC}"
        exit 0
    else
        echo -e "${RED}╔════════════════════════════════════════════════════════════╗${NC}"
        echo -e "${RED}║  ✗ TEST FAILED: ${TEST_NAME}${NC}"
        echo -e "${RED}║  Duration: ${duration}s${NC}"
        echo -e "${RED}╚════════════════════════════════════════════════════════════╝${NC}"
        exit 1
    fi
}

# Dump debug info on failure
dump_debug_info() {
    local namespace="${1:-$TEST_NAMESPACE}"
    
    log_warn "Dumping debug information..."
    
    echo "=== Pods ==="
    kubectl get pods -n "${namespace}" -o wide 2>/dev/null || true
    
    echo "=== Pod Descriptions ==="
    kubectl describe pods -n "${namespace}" 2>/dev/null || true
    
    echo "=== SQLServer Resources ==="
    kubectl get sqlserver -n "${namespace}" -o yaml 2>/dev/null || true
    
    echo "=== SQLServerAG Resources ==="
    kubectl get sqlserverag -n "${namespace}" -o yaml 2>/dev/null || true
    
    echo "=== Events ==="
    kubectl get events -n "${namespace}" --sort-by='.lastTimestamp' 2>/dev/null || true
    
    echo "=== Operator Logs ==="
    kubectl logs -n "${OPERATOR_NAMESPACE}" -l app.kubernetes.io/name=mssql-operator \
        --tail=100 2>/dev/null || true
}
