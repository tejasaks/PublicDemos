#!/bin/bash

# Test Prerequisites - Validates Docker and SQL Server tools installation
# This script checks if all required tools are installed and properly configured

# Note: We don't use 'set -e' here so the script continues through all checks
# even if some fail, providing complete diagnostic information

# Color codes for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Test results
TESTS_PASSED=0
TESTS_FAILED=0

echo -e "${BLUE}╔════════════════════════════════════════════════════════════╗${NC}"
echo -e "${BLUE}║  SQL AI Custom Container - Prerequisites Test Suite       ║${NC}"
echo -e "${BLUE}╔════════════════════════════════════════════════════════════╗${NC}"
echo ""

# Function to print test result
print_test_result() {
    local test_name="$1"
    local result="$2"
    local message="$3"
    
    if [ "$result" = "PASS" ]; then
        echo -e "${GREEN}✅ PASS${NC} - $test_name"
        ((TESTS_PASSED++))
    else
        echo -e "${RED}❌ FAIL${NC} - $test_name"
        if [ -n "$message" ]; then
            echo -e "${RED}   └─ $message${NC}"
        fi
        ((TESTS_FAILED++))
    fi
}

# Test 1: Check if Docker is installed
echo -e "${YELLOW}[1/7]${NC} Checking Docker installation..."
if command -v docker &> /dev/null; then
    DOCKER_VERSION=$(docker --version)
    print_test_result "Docker installed" "PASS" "Version: $DOCKER_VERSION"
else
    print_test_result "Docker installed" "FAIL" "Docker not found in PATH. Please install Docker Desktop or Docker Engine."
fi

# Test 2: Check if Docker daemon is running
echo -e "${YELLOW}[2/7]${NC} Checking Docker daemon status..."
if docker info &> /dev/null; then
    print_test_result "Docker daemon running" "PASS"
else
    print_test_result "Docker daemon running" "FAIL" "Docker daemon is not running. Please start Docker."
fi

# Test 3: Check Docker permissions
echo -e "${YELLOW}[3/7]${NC} Checking Docker permissions..."
if docker ps &> /dev/null; then
    print_test_result "Docker permissions" "PASS" "User has permission to run Docker commands"
else
    print_test_result "Docker permissions" "FAIL" "User does not have permission to run Docker. Add user to 'docker' group or run with sudo."
fi

# Test 4: Check if sqlcmd is installed
echo -e "${YELLOW}[4/7]${NC} Checking SQL Server command-line tools..."
if command -v sqlcmd &> /dev/null; then
    SQLCMD_VERSION=$(sqlcmd -? 2>&1 | head -n 1 || echo "Version info not available")
    print_test_result "sqlcmd installed" "PASS" "$SQLCMD_VERSION"
else
    print_test_result "sqlcmd installed" "FAIL" "sqlcmd not found. Install SQL Server command-line tools from https://docs.microsoft.com/sql/tools/sqlcmd-utility"
fi

# Test 5: Check if sqlcmd is in PATH
echo -e "${YELLOW}[5/7]${NC} Checking sqlcmd PATH configuration..."
if which sqlcmd &> /dev/null; then
    SQLCMD_PATH=$(which sqlcmd)
    print_test_result "sqlcmd in PATH" "PASS" "Location: $SQLCMD_PATH"
else
    print_test_result "sqlcmd in PATH" "FAIL" "sqlcmd not found in PATH. Update your .bashrc or .bash_profile to include sqlcmd location."
fi

# Test 6: Check if SA password is set
echo -e "${YELLOW}[6/7]${NC} Checking SA password environment variable..."
if [ -n "$SA_PASSWORD" ]; then
    # Validate password complexity (at least 8 chars, uppercase, lowercase, digit, special)
    if [[ ${#SA_PASSWORD} -ge 8 ]] && \
       [[ "$SA_PASSWORD" =~ [A-Z] ]] && \
       [[ "$SA_PASSWORD" =~ [a-z] ]] && \
       [[ "$SA_PASSWORD" =~ [0-9] ]] && \
       [[ "$SA_PASSWORD" =~ [^a-zA-Z0-9] ]]; then
        print_test_result "SA_PASSWORD set and valid" "PASS" "Password meets SQL Server complexity requirements"
    else
        print_test_result "SA_PASSWORD set and valid" "FAIL" "Password does not meet SQL Server requirements (8+ chars, uppercase, lowercase, digit, special character)"
    fi
else
    print_test_result "SA_PASSWORD set" "FAIL" "SA_PASSWORD environment variable not set. Run: export SA_PASSWORD='YourComplexPass@123'"
fi

# Test 7: Check available disk space
echo -e "${YELLOW}[7/7]${NC} Checking available disk space..."
AVAILABLE_SPACE=$(df -BG . | tail -1 | awk '{print $4}' | sed 's/G//')
REQUIRED_SPACE=20

if [ "$AVAILABLE_SPACE" -ge "$REQUIRED_SPACE" ]; then
    print_test_result "Disk space available" "PASS" "${AVAILABLE_SPACE}GB available (required: ${REQUIRED_SPACE}GB)"
else
    print_test_result "Disk space available" "FAIL" "Only ${AVAILABLE_SPACE}GB available, ${REQUIRED_SPACE}GB required for full test suite"
fi

# Print summary
echo ""
echo -e "${BLUE}════════════════════════════════════════════════════════════${NC}"
echo -e "${BLUE}                    Test Summary                            ${NC}"
echo -e "${BLUE}════════════════════════════════════════════════════════════${NC}"
echo -e "Tests Passed: ${GREEN}$TESTS_PASSED${NC}"
echo -e "Tests Failed: ${RED}$TESTS_FAILED${NC}"
echo ""

if [ $TESTS_FAILED -eq 0 ]; then
    echo -e "${GREEN}✅ All prerequisite tests passed! Ready to run deployment tests.${NC}"
    exit 0
else
    echo -e "${RED}❌ Some prerequisite tests failed. Please fix the issues above before running deployment tests.${NC}"
    exit 1
fi
