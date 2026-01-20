# Quick Start Guide - SQL AI Custom Container Test Suite

## One-Line Test Execution

```bash
cd tests && export SA_PASSWORD='YourComplexPass@123' && ./run-all-tests.sh
```

## Step-by-Step

### 1. Set SA Password
```bash
export SA_PASSWORD='YourComplexPass@123'
```

**Password Requirements:**
- Minimum 8 characters
- Must include uppercase letters
- Must include lowercase letters
- Must include digits
- Must include special characters (!@#$%^&*)

### 2. Run All Tests
```bash
cd tests
./run-all-tests.sh
```

### 3. Or Run Individual Tests

**Check Prerequisites Only:**
```bash
./test-prerequisites.sh
```

**Run Deployment Tests Only:**
```bash
./test-deployments.sh
```

**Cleanup Resources:**
```bash
./cleanup.sh
```

## Test Matrix

The test suite validates **14 deployment scenarios**:

| # | Configuration | Ollama | MinIO | Polybase | Size | Memory |
|---|--------------|--------|-------|----------|------|--------|
| 1 | Minimal | ❌ | ❌ | ❌ | ~3.5GB | ~2GB |
| 2 | +Polybase | ❌ | ❌ | ✅ | ~3.8GB | ~2.5GB |
| 3 | +MinIO | ❌ | ✅ | ❌ | ~3.6GB | ~2.5GB |
| 4 | +Ollama (default) | ✅ | ❌ | ❌ | ~5.5GB | ~4GB |
| 5 | Ollama+Polybase | ✅ | ❌ | ✅ | ~5.8GB | ~4.5GB |
| 6 | Ollama+MinIO | ✅ | ✅ | ❌ | ~5.6GB | ~5GB |
| 7 | Full Stack | ✅ | ✅ | ✅ | ~5.8GB | ~5GB |

Each configuration is tested with **both Ubuntu and RHEL** base images = **14 total tests**

## What Gets Tested

For each scenario:
1. ✅ Docker image builds successfully
2. ✅ Container starts without errors
3. ✅ SQL Server accepts connections within timeout
4. ✅ `SELECT @@VERSION` executes and returns version info
5. ✅ `SELECT @@SERVERNAME` executes and returns server name
6. ✅ Container, image, and volumes are cleaned up

## Expected Output

```
╔══════════════════════════════════════════════════════════════════╗
║              ✅✅✅ ALL TESTS PASSED! ✅✅✅                      ║
╚══════════════════════════════════════════════════════════════════╝

Total Tests Run: 14
Tests Passed: 14
Tests Failed: 0
```

## Troubleshooting

### SA Password Error
```
Error: SA_PASSWORD environment variable not set
```
**Fix:** `export SA_PASSWORD='YourComplexPass@123'`

### Docker Not Running
```
❌ FAIL - Docker daemon running
```
**Fix:** Start Docker Desktop

### sqlcmd Not Found
```
❌ FAIL - sqlcmd installed
```
**Fix:** 
- **macOS:** `brew install mssql-tools`
- **Linux:** Install via package manager

### Timeout Waiting for SQL
```
❌ SQL Server failed to start
```
**Fix:** Increase timeout: `export TEST_TIMEOUT=180`

### Keep Failed Containers for Debugging
```bash
export SKIP_CLEANUP=1
./test-deployments.sh

# Then inspect
docker logs sql-ai-test-ubuntu-minimal
docker exec -it sql-ai-test-ubuntu-minimal /bin/bash
```

## Test Only Specific Scenarios

Edit `test-deployments.sh` and modify:

```bash
# Test only Ubuntu
BASE_IMAGES=("ubuntu")

# Test only RHEL  
BASE_IMAGES=("rhel")

# Test both (default)
BASE_IMAGES=("ubuntu" "rhel")
```

## Environment Variables

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| `SA_PASSWORD` | ✅ Yes | - | SQL Server SA password |
| `TEST_TIMEOUT` | ❌ No | 120 | Seconds to wait for SQL startup |
| `SKIP_CLEANUP` | ❌ No | 0 | Set to 1 to keep resources after tests |

## Estimated Duration

- **Prerequisites check:** ~10 seconds
- **Per deployment test:** ~3-5 minutes
- **Total (14 scenarios):** ~30-60 minutes

**Note:** First run is slower due to base image downloads. Subsequent runs benefit from Docker layer caching.

## CI/CD Example

```yaml
# .github/workflows/test.yml
name: Test Deployment Scenarios

on: [push, pull_request]

jobs:
  test:
    runs-on: ubuntu-latest
    
    steps:
      - uses: actions/checkout@v3
      
      - name: Install SQL Server Tools
        run: |
          curl https://packages.microsoft.com/keys/microsoft.asc | sudo apt-key add -
          sudo add-apt-repository "$(curl https://packages.microsoft.com/config/ubuntu/22.04/prod.list)"
          sudo apt-get update
          sudo apt-get install -y mssql-tools unixodbc-dev
          echo 'export PATH="$PATH:/opt/mssql-tools/bin"' >> ~/.bashrc
      
      - name: Run Tests
        env:
          SA_PASSWORD: ${{ secrets.SA_PASSWORD }}
        run: |
          cd tests
          chmod +x *.sh
          ./run-all-tests.sh
```

## Manual Cleanup

If tests fail and leave resources behind:

```bash
# Clean up everything
./cleanup.sh

# Or manually
docker ps -a | grep sql-ai-test | awk '{print $1}' | xargs docker rm -f
docker images | grep sql-ai-custom | awk '{print $3}' | xargs docker rmi -f
docker volume ls | grep -E 'sqldata|ollama-models|minio-data' | awk '{print $2}' | xargs docker volume rm
```

## Support

For issues or questions:
1. Check test logs in `/tmp/build-*.log`
2. Review container logs: `docker logs <container-name>`
3. Run prerequisites test to verify setup: `./test-prerequisites.sh`
