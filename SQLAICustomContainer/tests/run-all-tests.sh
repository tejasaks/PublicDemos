#!/bin/bash

# Main Test Runner - Orchestrates all test suites
# This script runs prerequisites checks followed by deployment tests

set -e

# Color codes
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# Print header
clear
echo -e "${BLUE}"
echo "╔══════════════════════════════════════════════════════════════════╗"
echo "║                                                                  ║"
echo "║      SQL AI Custom Container - Complete Test Suite              ║"
echo "║                                                                  ║"
echo "║  This test suite validates all deployment configurations        ║"
echo "║  across Ubuntu and RHEL base images.                            ║"
echo "║                                                                  ║"
echo "╚══════════════════════════════════════════════════════════════════╝"
echo -e "${NC}"
echo ""

# Check if SA password is set
if [ -z "$SA_PASSWORD" ]; then
    echo -e "${RED}Error: SA_PASSWORD environment variable not set${NC}"
    echo ""
    echo "Please set a strong SA password that meets SQL Server requirements:"
    echo "  - At least 8 characters"
    echo "  - Contains uppercase and lowercase letters"
    echo "  - Contains digits"
    echo "  - Contains special characters"
    echo ""
    echo "Example:"
    echo "  export SA_PASSWORD='MyComplexPass@123'"
    echo ""
    exit 1
fi

# Validate password complexity
PASSWORD_VALID=true
PASSWORD_ERRORS=""

if [[ ${#SA_PASSWORD} -lt 8 ]]; then
    PASSWORD_VALID=false
    PASSWORD_ERRORS="${PASSWORD_ERRORS}\n  ❌ Password must be at least 8 characters (currently ${#SA_PASSWORD})"
fi

if [[ ! "$SA_PASSWORD" =~ [A-Z] ]]; then
    PASSWORD_VALID=false
    PASSWORD_ERRORS="${PASSWORD_ERRORS}\n  ❌ Password must contain at least one uppercase letter (A-Z)"
fi

if [[ ! "$SA_PASSWORD" =~ [a-z] ]]; then
    PASSWORD_VALID=false
    PASSWORD_ERRORS="${PASSWORD_ERRORS}\n  ❌ Password must contain at least one lowercase letter (a-z)"
fi

if [[ ! "$SA_PASSWORD" =~ [0-9] ]]; then
    PASSWORD_VALID=false
    PASSWORD_ERRORS="${PASSWORD_ERRORS}\n  ❌ Password must contain at least one digit (0-9)"
fi

if [[ ! "$SA_PASSWORD" =~ [^a-zA-Z0-9] ]]; then
    PASSWORD_VALID=false
    PASSWORD_ERRORS="${PASSWORD_ERRORS}\n  ❌ Password must contain at least one special character (!@#\$%^&*)"
fi

if [ "$PASSWORD_VALID" = false ]; then
    echo -e "${RED}Error: SA_PASSWORD does not meet SQL Server complexity requirements${NC}"
    echo ""
    echo -e "${RED}Password validation failed:${NC}"
    echo -e "${PASSWORD_ERRORS}"
    echo ""
    echo -e "${YELLOW}SQL Server Password Requirements:${NC}"
    echo "  ✓ At least 8 characters"
    echo "  ✓ Contains uppercase letters (A-Z)"
    echo "  ✓ Contains lowercase letters (a-z)"
    echo "  ✓ Contains digits (0-9)"
    echo "  ✓ Contains special characters (!@#\$%^&*)"
    echo ""
    echo -e "${GREEN}Example of a valid password:${NC}"
    echo "  export SA_PASSWORD='MyComplexPass@123'"
    echo ""
    exit 1
fi

# Display configuration
echo -e "${BLUE}Test Configuration:${NC}"
echo -e "  Working Directory: ${YELLOW}$SCRIPT_DIR${NC}"
echo -e "  SA Password: ${GREEN}****** (configured)${NC}"
echo -e "  Test Timeout: ${YELLOW}${TEST_TIMEOUT:-120}s${NC}"
echo -e "  Skip Cleanup: ${YELLOW}${SKIP_CLEANUP:-0}${NC}"
echo ""

# Ask for confirmation
echo -e "${YELLOW}This test suite will:${NC}"
echo "  1. Validate prerequisites (Docker, SQL tools)"
echo "  2. Build and test 14 deployment scenarios (7 × 2 base images)"
echo "  3. Test SQL connectivity for each deployment"
echo "  4. Clean up containers, images, and volumes"
echo ""
echo -e "${YELLOW}Estimated duration: 30-60 minutes depending on your system${NC}"
echo ""
echo -e "${YELLOW}Continue? [y/N]${NC} "
read -r CONTINUE

if [[ ! "$CONTINUE" =~ ^[Yy]$ ]]; then
    echo -e "${BLUE}Test suite cancelled.${NC}"
    exit 0
fi

echo ""
echo -e "${BLUE}════════════════════════════════════════════════════════════${NC}"
echo -e "${BLUE}Phase 1: Prerequisites Check${NC}"
echo -e "${BLUE}════════════════════════════════════════════════════════════${NC}"
echo ""

# Run prerequisite tests
if bash "$SCRIPT_DIR/test-prerequisites.sh"; then
    echo ""
    echo -e "${GREEN}✅ Prerequisites check passed!${NC}"
    PREREQ_PASSED=1
else
    echo ""
    echo -e "${RED}❌ Prerequisites check failed!${NC}"
    echo ""
    echo -e "${YELLOW}Do you want to continue with deployment tests anyway? [y/N]${NC} "
    read -r CONTINUE_ANYWAY
    
    if [[ ! "$CONTINUE_ANYWAY" =~ ^[Yy]$ ]]; then
        echo -e "${RED}Test suite aborted. Please fix prerequisite issues.${NC}"
        exit 1
    fi
    PREREQ_PASSED=0
fi

echo ""
echo -e "${BLUE}════════════════════════════════════════════════════════════${NC}"
echo -e "${BLUE}Phase 2: Deployment Tests${NC}"
echo -e "${BLUE}════════════════════════════════════════════════════════════${NC}"
echo ""

# Small pause before starting deployment tests
sleep 3

# Run deployment tests
START_TIME=$(date +%s)

if bash "$SCRIPT_DIR/test-deployments.sh"; then
    DEPLOYMENT_PASSED=1
else
    DEPLOYMENT_PASSED=0
fi

END_TIME=$(date +%s)
DURATION=$((END_TIME - START_TIME))
MINUTES=$((DURATION / 60))
SECONDS=$((DURATION % 60))

# Print final summary
echo ""
echo -e "${BLUE}╔══════════════════════════════════════════════════════════════════╗${NC}"
echo -e "${BLUE}║                    Complete Test Suite Summary                  ║${NC}"
echo -e "${BLUE}╚══════════════════════════════════════════════════════════════════╝${NC}"
echo ""
echo -e "Test Duration: ${YELLOW}${MINUTES}m ${SECONDS}s${NC}"
echo ""

if [ $PREREQ_PASSED -eq 1 ]; then
    echo -e "Prerequisites: ${GREEN}✅ PASSED${NC}"
else
    echo -e "Prerequisites: ${RED}❌ FAILED${NC}"
fi

if [ $DEPLOYMENT_PASSED -eq 1 ]; then
    echo -e "Deployment Tests: ${GREEN}✅ PASSED${NC}"
else
    echo -e "Deployment Tests: ${RED}❌ FAILED${NC}"
fi

echo ""

# Overall result
if [ $PREREQ_PASSED -eq 1 ] && [ $DEPLOYMENT_PASSED -eq 1 ]; then
    echo -e "${GREEN}╔══════════════════════════════════════════════════════════════════╗${NC}"
    echo -e "${GREEN}║              ✅✅✅ ALL TESTS PASSED! ✅✅✅                      ║${NC}"
    echo -e "${GREEN}╚══════════════════════════════════════════════════════════════════╝${NC}"
    echo ""
    echo -e "${GREEN}Your SQL AI Custom Container is working correctly!${NC}"
    exit 0
else
    echo -e "${RED}╔══════════════════════════════════════════════════════════════════╗${NC}"
    echo -e "${RED}║                  ❌ SOME TESTS FAILED ❌                         ║${NC}"
    echo -e "${RED}╚══════════════════════════════════════════════════════════════════╝${NC}"
    echo ""
    echo -e "${RED}Please review the test output above for details.${NC}"
    exit 1
fi
