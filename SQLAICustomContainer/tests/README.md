# SQL AI Custom Container - Test Suite

This test suite validates all deployment configurations for the SQL Server + AI + Object Storage container across both Ubuntu and RHEL base images.

## Test Structure

- `run-all-tests.sh` - Main test orchestrator that runs all tests
- `test-prerequisites.sh` - Validates Docker and SQL Server tools installation
- `test-deployments.sh` - Tests all deployment scenarios
- `test-scenarios.conf` - Configuration file defining all test scenarios
- `cleanup.sh` - Cleanup helper for removing containers, images, and volumes

## Prerequisites

Before running tests, ensure you have:

1. **Docker** installed and running
   - Installation guide: [Get Docker](https://docs.docker.com/get-docker/)
   - Linux: [Install Docker Engine](https://docs.docker.com/engine/install/)
   - macOS: [Install Docker Desktop](https://docs.docker.com/desktop/install/mac-install/)

2. **SQL Server command-line tools** (`sqlcmd`) installed and in PATH
   - Installation guide: [Install sqlcmd on Linux](https://learn.microsoft.com/en-us/sql/linux/sql-server-linux-setup-tools)

3. **Bash** shell (Linux or macOS)

4. **SA Password** set as environment variable: `export SA_PASSWORD='YourPass@123'`

## Running Tests

### Run All Tests

```bash
cd tests
./run-all-tests.sh
```

### Run Individual Test Suites

```bash
# Check prerequisites only
./test-prerequisites.sh

# Test specific deployment scenarios
./test-deployments.sh
```

### Clean Up After Failed Tests

```bash
./cleanup.sh
```

## Test Scenarios

The test suite validates **14 deployment configurations**:

### Ubuntu Base Image (7 scenarios)
1. Minimal: SQL + FTS only
2. SQL + FTS + Polybase
3. SQL + FTS + MinIO
4. SQL + FTS + Ollama (default)
5. SQL + FTS + Ollama + Polybase
6. SQL + FTS + Ollama + MinIO
7. Full Stack: All components

### RHEL Base Image (7 scenarios)
Same configurations as Ubuntu, using RHEL base image

## What Gets Tested

For each scenario:
1. ✅ Container builds successfully
2. ✅ Container starts without errors
3. ✅ SQL Server accepts connections
4. ✅ `SELECT @@VERSION` returns expected output
5. ✅ `SELECT @@SERVERNAME` returns container hostname
6. ✅ Cleanup removes container, image, and volumes

## Test Output

Tests produce detailed output with:
- ✅ Green checkmarks for passed tests
- ❌ Red X marks for failed tests
- Detailed error messages for failures
- Summary report at the end

## Environment Variables

- `SA_PASSWORD` - Required: SA password for SQL Server (must meet complexity requirements)
- `SKIP_CLEANUP` - Optional: Set to `1` to keep containers/images after tests for debugging
- `TEST_TIMEOUT` - Optional: Timeout in seconds for SQL Server startup (default: 120)

## Troubleshooting

### Docker Not Found
```
Error: Docker is not installed or not in PATH
```
**Solution**: Install Docker Desktop or Docker Engine and ensure it's running

### sqlcmd Not Found
```
Error: SQL Server tools (sqlcmd) not found
```
**Solution**: Install SQL Server command-line tools
- macOS: `brew install mssql-tools`
- Linux: Install via package manager

### SA Password Not Set
```
Error: SA_PASSWORD environment variable not set
```
**Solution**: `export SA_PASSWORD='YourComplexPass@123'`

### Container Won't Start
Check logs for the specific container:
```bash
docker logs sql-ai-test-<scenario>
```

## Advanced Usage

### Test Only Specific Base Image

Edit `test-deployments.sh` and comment out the base images you don't want to test:

```bash
# BASE_IMAGES=("ubuntu" "rhel")
BASE_IMAGES=("ubuntu")  # Test Ubuntu only
```

### Keep Failed Containers for Debugging

```bash
export SKIP_CLEANUP=1
./run-all-tests.sh
```

Then inspect the failed container:
```bash
docker logs sql-ai-test-failed-scenario
docker exec -it sql-ai-test-failed-scenario /bin/bash
```

### Increase Startup Timeout

For slower systems:
```bash
export TEST_TIMEOUT=180  # 3 minutes
./run-all-tests.sh
```

## CI/CD Integration

The test suite is designed for CI/CD pipelines:

```yaml
# Example GitHub Actions workflow
test:
  runs-on: ubuntu-latest
  steps:
    - uses: actions/checkout@v2
    - name: Install SQL Server tools
      run: |
        curl https://packages.microsoft.com/keys/microsoft.asc | sudo apt-key add -
        sudo add-apt-repository "$(curl https://packages.microsoft.com/config/ubuntu/20.04/prod.list)"
        sudo apt-get update
        sudo apt-get install -y mssql-tools unixodbc-dev
    - name: Run tests
      env:
        SA_PASSWORD: ${{ secrets.SA_PASSWORD }}
      run: |
        cd tests
        ./run-all-tests.sh
```

## Contributing

When adding new deployment scenarios:
1. Add the configuration to `test-scenarios.conf`
2. Update this README with the new scenario
3. Test with both Ubuntu and RHEL base images
