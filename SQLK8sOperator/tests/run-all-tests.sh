#!/bin/bash
# Run all MSSQL Operator tests in sequence

set -o pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

# Test results
TESTS_RUN=0
TESTS_PASSED=0
TESTS_FAILED=0
FAILED_TESTS=()

# Logging
log_header() {
    echo -e "\n${BLUE}════════════════════════════════════════════════════════════════${NC}"
    echo -e "${BLUE}  $1${NC}"
    echo -e "${BLUE}════════════════════════════════════════════════════════════════${NC}\n"
}

log_info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

log_success() {
    echo -e "${GREEN}[PASS]${NC} $1"
}

log_error() {
    echo -e "${RED}[FAIL]${NC} $1"
}

# Run a single test
run_test() {
    local test_script="$1"
    local test_name=$(basename "${test_script}" .sh)
    
    TESTS_RUN=$((TESTS_RUN + 1))
    
    log_header "Running: ${test_name}"
    
    if "${test_script}"; then
        TESTS_PASSED=$((TESTS_PASSED + 1))
        log_success "${test_name} PASSED"
        return 0
    else
        TESTS_FAILED=$((TESTS_FAILED + 1))
        FAILED_TESTS+=("${test_name}")
        log_error "${test_name} FAILED"
        return 1
    fi
}

# Print summary
print_summary() {
    echo -e "\n"
    echo -e "${BLUE}╔════════════════════════════════════════════════════════════════╗${NC}"
    echo -e "${BLUE}║                      TEST SUMMARY                               ║${NC}"
    echo -e "${BLUE}╠════════════════════════════════════════════════════════════════╣${NC}"
    echo -e "${BLUE}║${NC}  Tests Run:    ${TESTS_RUN}                                             ${BLUE}║${NC}"
    echo -e "${BLUE}║${NC}  ${GREEN}Tests Passed: ${TESTS_PASSED}${NC}                                             ${BLUE}║${NC}"
    echo -e "${BLUE}║${NC}  ${RED}Tests Failed: ${TESTS_FAILED}${NC}                                             ${BLUE}║${NC}"
    echo -e "${BLUE}╚════════════════════════════════════════════════════════════════╝${NC}"
    
    if [[ ${#FAILED_TESTS[@]} -gt 0 ]]; then
        echo -e "\n${RED}Failed Tests:${NC}"
        for test in "${FAILED_TESTS[@]}"; do
            echo -e "  ${RED}✗${NC} ${test}"
        done
    fi
    
    echo ""
}

# Check prerequisites
check_prerequisites() {
    log_header "Checking Prerequisites"
    
    # Check kubectl
    if ! command -v kubectl &>/dev/null; then
        log_error "kubectl not found"
        exit 1
    fi
    log_success "kubectl found"
    
    # Check cluster connection
    if ! kubectl cluster-info &>/dev/null; then
        log_error "Cannot connect to Kubernetes cluster"
        exit 1
    fi
    log_success "Connected to Kubernetes cluster"
    
    # Check jq (optional but useful)
    if command -v jq &>/dev/null; then
        log_success "jq found"
    else
        log_info "jq not found (some tests may have reduced functionality)"
    fi
}

# Main
main() {
    local start_time=$(date +%s)
    
    echo -e "${GREEN}"
    echo "╔════════════════════════════════════════════════════════════════╗"
    echo "║                                                                ║"
    echo "║              MSSQL KUBERNETES OPERATOR TESTS                   ║"
    echo "║                                                                ║"
    echo "║  Started: $(date)                     ║"
    echo "║                                                                ║"
    echo "╚════════════════════════════════════════════════════════════════╝"
    echo -e "${NC}"
    
    check_prerequisites
    
    # Create logs directory
    mkdir -p "${SCRIPT_DIR}/logs"
    
    # Parse arguments
    local run_specific=""
    if [[ $# -gt 0 ]]; then
        run_specific="$1"
    fi
    
    # Define test order
    local tests=(
        "test-operator-deployment.sh"
        "test-crd-validation.sh"
        "test-sqlserver-standalone.sh"
        "test-sqlserver-ag.sh"
    )
    
    # Run tests
    for test in "${tests[@]}"; do
        test_path="${SCRIPT_DIR}/${test}"
        test_name=$(basename "${test}" .sh)
        
        # Skip if not matching specific test
        if [[ -n "${run_specific}" && "${test_name}" != *"${run_specific}"* ]]; then
            continue
        fi
        
        if [[ -f "${test_path}" && -x "${test_path}" ]]; then
            run_test "${test_path}" 2>&1 | tee "${SCRIPT_DIR}/logs/${test_name}.log"
        else
            log_info "Skipping ${test} (not executable or not found)"
        fi
    done
    
    local end_time=$(date +%s)
    local duration=$((end_time - start_time))
    
    print_summary
    
    echo -e "Total Duration: ${duration}s"
    echo -e "Logs saved to: ${SCRIPT_DIR}/logs/"
    echo ""
    
    if [[ ${TESTS_FAILED} -gt 0 ]]; then
        exit 1
    else
        exit 0
    fi
}

# Show usage
usage() {
    echo "Usage: $0 [test-name-filter]"
    echo ""
    echo "Examples:"
    echo "  $0                    # Run all tests"
    echo "  $0 operator           # Run tests matching 'operator'"
    echo "  $0 standalone         # Run tests matching 'standalone'"
    echo ""
    echo "Available tests:"
    echo "  test-operator-deployment  - Operator installation tests"
    echo "  test-crd-validation       - CRD schema validation tests"
    echo "  test-sqlserver-standalone - Standalone SQL Server tests"
    echo "  test-sqlserver-ag         - Availability Group tests"
}

if [[ "$1" == "-h" || "$1" == "--help" ]]; then
    usage
    exit 0
fi

main "$@"
