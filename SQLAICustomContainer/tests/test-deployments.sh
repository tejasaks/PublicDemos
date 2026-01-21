#!/bin/bash

# Deployment Test Suite - Tests all deployment scenarios
# This script builds, deploys, tests, and cleans up all container configurations

# Note: We handle errors manually instead of using 'set -e' to provide better
# error messages and continue through all test scenarios

# Color codes
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

# Configuration
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SCENARIOS_FILE="$SCRIPT_DIR/test-scenarios.conf"
BUILD_SCRIPT="$SCRIPT_DIR/../build-and-run.sh"
CLEANUP_SCRIPT="$SCRIPT_DIR/cleanup.sh"

# Test settings
TEST_TIMEOUT=${TEST_TIMEOUT:-120}  # Seconds to wait for SQL Server startup
SQL_PORT=1433
SKIP_CLEANUP=${SKIP_CLEANUP:-0}

# Base images to test
BASE_IMAGES=("ubuntu" "rhel")

# Test results tracking
TOTAL_TESTS=0
PASSED_TESTS=0
FAILED_TESTS=0
declare -a FAILED_SCENARIOS

# Ensure SA password is set
if [ -z "$SA_PASSWORD" ]; then
    echo -e "${RED}Error: SA_PASSWORD environment variable not set${NC}"
    echo "Please set it with: export SA_PASSWORD='YourComplexPass@123'"
    exit 1
fi

# Verify build script exists and is executable
if [ ! -f "$BUILD_SCRIPT" ]; then
    echo -e "${RED}Error: Build script not found at: $BUILD_SCRIPT${NC}"
    echo -e "${YELLOW}SCRIPT_DIR: $SCRIPT_DIR${NC}"
    echo -e "${YELLOW}Looking for: $BUILD_SCRIPT${NC}"
    exit 1
fi

if [ ! -x "$BUILD_SCRIPT" ]; then
    echo -e "${RED}Error: Build script is not executable: $BUILD_SCRIPT${NC}"
    echo "Run: chmod +x $BUILD_SCRIPT"
    exit 1
fi

echo -e "${BLUE}╔════════════════════════════════════════════════════════════╗${NC}"
echo -e "${BLUE}║    SQL AI Custom Container - Deployment Test Suite        ║${NC}"
echo -e "${BLUE}╚════════════════════════════════════════════════════════════╝${NC}"
echo ""
echo -e "${BLUE}Test Configuration:${NC}"
echo -e "  SA Password: ${GREEN}****** (set)${NC}"
echo -e "  Test Timeout: ${YELLOW}${TEST_TIMEOUT}s${NC}"
echo -e "  Skip Cleanup: ${YELLOW}$SKIP_CLEANUP${NC}"
echo -e "  Base Images: ${YELLOW}${BASE_IMAGES[*]}${NC}"
echo ""

# Function to wait for SQL Server to be ready
wait_for_sql() {
    local container_name="$1"
    local max_wait=$TEST_TIMEOUT
    local elapsed=0
    
    echo -e "${YELLOW}Waiting for SQL Server to be ready (timeout: ${max_wait}s)...${NC}"
    
    while [ $elapsed -lt $max_wait ]; do
        # Check if container is still running
        if ! docker ps --format '{{.Names}}' | grep -q "^${container_name}$"; then
            echo -e "${RED}Container stopped unexpectedly${NC}"
            return 1
        fi
        
        # Try to connect with sqlcmd
        if docker exec "$container_name" /opt/mssql-tools/bin/sqlcmd \
            -S localhost -U sa -P "$SA_PASSWORD" \
            -Q "SELECT 1" &> /dev/null; then
            echo -e "${GREEN}✅ SQL Server is ready (${elapsed}s)${NC}"
            return 0
        fi
        
        sleep 5
        elapsed=$((elapsed + 5))
        echo -n "."
    done
    
    echo ""
    echo -e "${RED}Timeout waiting for SQL Server to start${NC}"
    return 1
}

# Function to test SQL connectivity
test_sql_connectivity() {
    local container_name="$1"
    local scenario_name="$2"
    
    echo -e "${YELLOW}Testing SQL connectivity...${NC}"
    
    # Test 1: SELECT @@VERSION
    echo -e "  ${BLUE}[1/2]${NC} Testing SELECT @@VERSION..."
    VERSION_OUTPUT=$(docker exec "$container_name" /opt/mssql-tools/bin/sqlcmd \
        -S localhost -U sa -P "$SA_PASSWORD" \
        -Q "SET NOCOUNT ON; SELECT @@VERSION" -h -1 2>&1)
    
    if [ $? -eq 0 ]; then
        echo -e "  ${GREEN}✅ @@VERSION query succeeded${NC}"
        echo -e "  ${BLUE}Version: $(echo "$VERSION_OUTPUT" | head -n 1 | xargs)${NC}"
    else
        echo -e "  ${RED}❌ @@VERSION query failed${NC}"
        echo -e "  ${RED}Error: $VERSION_OUTPUT${NC}"
        return 1
    fi
    
    # Test 2: SELECT @@SERVERNAME
    echo -e "  ${BLUE}[2/2]${NC} Testing SELECT @@SERVERNAME..."
    SERVERNAME_OUTPUT=$(docker exec "$container_name" /opt/mssql-tools/bin/sqlcmd \
        -S localhost -U sa -P "$SA_PASSWORD" \
        -Q "SET NOCOUNT ON; SELECT @@SERVERNAME" -h -1 2>&1)
    
    if [ $? -eq 0 ]; then
        echo -e "  ${GREEN}✅ @@SERVERNAME query succeeded${NC}"
        echo -e "  ${BLUE}Server Name: $(echo "$SERVERNAME_OUTPUT" | xargs)${NC}"
    else
        echo -e "  ${RED}❌ @@SERVERNAME query failed${NC}"
        echo -e "  ${RED}Error: $SERVERNAME_OUTPUT${NC}"
        return 1
    fi
    
    return 0
}

# Function to build and test a scenario
test_scenario() {
    local scenario_name="$1"
    local install_ollama="$2"
    local install_minio="$3"
    local enable_polybase="$4"
    local base_image="$5"
    local description="$6"
    
    local container_name="sql-ai-test-${base_image}-${scenario_name}"
    local image_name="sql-ai-custom:test-${base_image}-${scenario_name}"
    
    echo ""
    echo -e "${BLUE}╔════════════════════════════════════════════════════════════╗${NC}"
    echo -e "${BLUE}║ Test: ${scenario_name} (${base_image})${NC}"
    echo -e "${BLUE}║ ${description}${NC}"
    echo -e "${BLUE}╚════════════════════════════════════════════════════════════╝${NC}"
    ((TOTAL_TESTS++))
    
    # Build command arguments
    BUILD_ARGS="--sa-password \"$SA_PASSWORD\" --no-follow"
    
    if [ "$install_ollama" = "false" ]; then
        BUILD_ARGS="$BUILD_ARGS --install-ollama false"
    fi
    
    if [ "$install_minio" = "true" ]; then
        BUILD_ARGS="$BUILD_ARGS --install-minio true"
    fi
    
    if [ "$enable_polybase" = "true" ]; then
        BUILD_ARGS="$BUILD_ARGS --polybase true"
    fi
    
    # Set base image
    if [ "$base_image" = "rhel" ]; then
        BUILD_ARGS="$BUILD_ARGS --base-image mcr.microsoft.com/mssql/rhel/server:2025-latest"
    fi
    
    echo -e "${YELLOW}Build command: $BUILD_SCRIPT $BUILD_ARGS${NC}"
    echo -e "${YELLOW}Log file: /tmp/build-${scenario_name}-${base_image}.log${NC}"
    echo ""
    
    # Step 1: Build the image
    echo -e "${BLUE}[1/5] Building Docker image...${NC}"
    
    # Run the build command and capture the exit code
    # IMPORTANT: Change to parent directory because build-and-run.sh expects to run from
    # the directory containing the Dockerfile
    LOG_FILE="/tmp/build-${scenario_name}-${base_image}.log"
    (cd "$SCRIPT_DIR/.." && eval "$BUILD_SCRIPT $BUILD_ARGS") &> "$LOG_FILE"
    BUILD_EXIT_CODE=$?
    
    if [ $BUILD_EXIT_CODE -eq 0 ]; then
        echo -e "${GREEN}✅ Build succeeded${NC}"
    else
        echo -e "${RED}❌ Build failed (exit code: $BUILD_EXIT_CODE)${NC}"
        echo -e "${RED}Check logs: $LOG_FILE${NC}"
        echo ""
        if [ -f "$LOG_FILE" ]; then
            echo -e "${YELLOW}Last 50 lines of build log:${NC}"
            tail -n 50 "$LOG_FILE"
        else
            echo -e "${RED}Log file not created - build command may have failed to start${NC}"
            echo -e "${YELLOW}Build script: $BUILD_SCRIPT${NC}"
            echo -e "${YELLOW}Build args: $BUILD_ARGS${NC}"
        fi
        FAILED_SCENARIOS+=("${base_image}/${scenario_name} - Build failed")
        ((FAILED_TESTS++))
        return 1
    fi
    
    # Get the actual container name created by build-and-run.sh
    ACTUAL_CONTAINER=$(docker ps -a --filter "ancestor=sql-ai-custom:latest" --format "{{.Names}}" | head -n 1)
    
    if [ -z "$ACTUAL_CONTAINER" ]; then
        echo -e "${RED}❌ Container not found after build${NC}"
        FAILED_SCENARIOS+=("${base_image}/${scenario_name} - Container not created")
        ((FAILED_TESTS++))
        return 1
    fi
    
    container_name="$ACTUAL_CONTAINER"
    echo -e "${BLUE}Container name: $container_name${NC}"
    
    # Step 2: Wait for SQL Server
    echo -e "${BLUE}[2/5] Waiting for SQL Server to start...${NC}"
    if wait_for_sql "$container_name"; then
        echo -e "${GREEN}✅ SQL Server started successfully${NC}"
    else
        echo -e "${RED}❌ SQL Server failed to start${NC}"
        echo -e "${RED}Container logs:${NC}"
        docker logs "$container_name" | tail -n 50
        FAILED_SCENARIOS+=("${base_image}/${scenario_name} - SQL Server startup failed")
        ((FAILED_TESTS++))
        
        if [ "$SKIP_CLEANUP" = "0" ]; then
            docker stop "$container_name" &> /dev/null || true
            docker rm "$container_name" &> /dev/null || true
        fi
        return 1
    fi
    
    # Step 3: Test SQL connectivity
    echo -e "${BLUE}[3/5] Testing SQL connectivity...${NC}"
    if test_sql_connectivity "$container_name" "$scenario_name"; then
        echo -e "${GREEN}✅ SQL connectivity tests passed${NC}"
    else
        echo -e "${RED}❌ SQL connectivity tests failed${NC}"
        FAILED_SCENARIOS+=("${base_image}/${scenario_name} - SQL connectivity failed")
        ((FAILED_TESTS++))
        
        if [ "$SKIP_CLEANUP" = "0" ]; then
            docker stop "$container_name" &> /dev/null || true
            docker rm "$container_name" &> /dev/null || true
        fi
        return 1
    fi
    
    # Step 4: Cleanup
    if [ "$SKIP_CLEANUP" = "0" ]; then
        echo -e "${BLUE}[4/5] Cleaning up resources...${NC}"
        
        # Stop and remove container
        docker stop "$container_name" &> /dev/null || true
        docker rm "$container_name" &> /dev/null || true
        
        # Remove image
        docker rmi sql-ai-custom:latest &> /dev/null || true
        
        # Remove volumes
        docker volume rm sqldata ollama-models minio-data &> /dev/null || true
        
        echo -e "${GREEN}✅ Cleanup complete${NC}"
    else
        echo -e "${YELLOW}⏭️  Skipping cleanup (SKIP_CLEANUP=1)${NC}"
    fi
    
    # Step 5: Mark as passed
    echo -e "${BLUE}[5/5] Test completed${NC}"
    echo -e "${GREEN}✅✅✅ Scenario PASSED: ${base_image}/${scenario_name}${NC}"
    ((PASSED_TESTS++))
    
    return 0
}

# Main test execution
echo -e "${BLUE}Reading test scenarios from: $SCENARIOS_FILE${NC}"
echo ""

# Loop through base images
for base_image in "${BASE_IMAGES[@]}"; do
    echo ""
    echo -e "${BLUE}════════════════════════════════════════════════════════════${NC}"
    echo -e "${BLUE}Testing with base image: $base_image${NC}"
    echo -e "${BLUE}════════════════════════════════════════════════════════════${NC}"
    
    # Loop through scenarios
    while IFS='|' read -r scenario_name install_ollama install_minio enable_polybase description; do
        # Skip comments and empty lines
        [[ "$scenario_name" =~ ^#.*$ ]] && continue
        [[ -z "$scenario_name" ]] && continue
        
        # Run test
        test_scenario "$scenario_name" "$install_ollama" "$install_minio" "$enable_polybase" "$base_image" "$description"
        
        # Small delay between tests
        sleep 2
        
    done < "$SCENARIOS_FILE"
done

# Print final summary
echo ""
echo -e "${BLUE}════════════════════════════════════════════════════════════${NC}"
echo -e "${BLUE}                    Final Test Summary                      ${NC}"
echo -e "${BLUE}════════════════════════════════════════════════════════════${NC}"
echo -e "Total Tests Run: ${BLUE}$TOTAL_TESTS${NC}"
echo -e "Tests Passed: ${GREEN}$PASSED_TESTS${NC}"
echo -e "Tests Failed: ${RED}$FAILED_TESTS${NC}"
echo ""

if [ $FAILED_TESTS -gt 0 ]; then
    echo -e "${RED}Failed Scenarios:${NC}"
    for failed in "${FAILED_SCENARIOS[@]}"; do
        echo -e "${RED}  ❌ $failed${NC}"
    done
    echo ""
    echo -e "${RED}❌ Some tests failed. See details above.${NC}"
    exit 1
else
    echo -e "${GREEN}✅✅✅ All tests passed successfully! ✅✅✅${NC}"
    exit 0
fi
