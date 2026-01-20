# Test Suite - Example Execution Output

This document shows what you can expect when running the test suite.

## Running All Tests

```bash
cd tests
export SA_PASSWORD='YourComplexPass@123'
./run-all-tests.sh
```

## Expected Output

### Phase 1: Prerequisites Check

```
╔══════════════════════════════════════════════════════════════════╗
║                                                                  ║
║      SQL AI Custom Container - Complete Test Suite              ║
║                                                                  ║
║  This test suite validates all deployment configurations        ║
║  across Ubuntu and RHEL base images.                            ║
║                                                                  ║
╚══════════════════════════════════════════════════════════════════╝

Test Configuration:
  Working Directory: /path/to/tests
  SA Password: ****** (configured)
  Test Timeout: 120s
  Skip Cleanup: 0

This test suite will:
  1. Validate prerequisites (Docker, SQL tools)
  2. Build and test 14 deployment scenarios (7 × 2 base images)
  3. Test SQL connectivity for each deployment
  4. Clean up containers, images, and volumes

Estimated duration: 30-60 minutes depending on your system

Continue? [y/N] y

════════════════════════════════════════════════════════════════
Phase 1: Prerequisites Check
════════════════════════════════════════════════════════════════

╔════════════════════════════════════════════════════════════╗
║  SQL AI Custom Container - Prerequisites Test Suite       ║
╔════════════════════════════════════════════════════════════╗

[1/7] Checking Docker installation...
✅ PASS - Docker installed
   Version: Docker version 24.0.7, build afdd53b

[2/7] Checking Docker daemon status...
✅ PASS - Docker daemon running

[3/7] Checking Docker permissions...
✅ PASS - Docker permissions
   User has permission to run Docker commands

[4/7] Checking SQL Server command-line tools...
✅ PASS - sqlcmd installed
   Microsoft (R) SQL Server Command Line Tool Version 18.3.1.1

[5/7] Checking sqlcmd PATH configuration...
✅ PASS - sqlcmd in PATH
   Location: /opt/mssql-tools/bin/sqlcmd

[6/7] Checking SA password environment variable...
✅ PASS - SA_PASSWORD set and valid
   Password meets SQL Server complexity requirements

[7/7] Checking available disk space...
✅ PASS - Disk space available
   45GB available (required: 20GB)

════════════════════════════════════════════════════════════
                    Test Summary
════════════════════════════════════════════════════════════
Tests Passed: 7
Tests Failed: 0

✅ All prerequisite tests passed! Ready to run deployment tests.
```

### Phase 2: Deployment Tests

```
════════════════════════════════════════════════════════════
Phase 2: Deployment Tests
════════════════════════════════════════════════════════════

Reading test scenarios from: /path/to/tests/test-scenarios.conf

════════════════════════════════════════════════════════════
Testing with base image: ubuntu
════════════════════════════════════════════════════════════

╔════════════════════════════════════════════════════════════╗
║ Test: minimal (ubuntu)
║ SQL Server + FTS only (minimal)
╚════════════════════════════════════════════════════════════╝

Build command: ./build-and-run.sh --sa-password "***" --install-ollama false

[1/5] Building Docker image...
✅ Build succeeded

Container name: sqlserver-ollama

[2/5] Waiting for SQL Server to start...
Waiting for SQL Server to be ready (timeout: 120s)...
.....✅ SQL Server is ready (25s)
✅ SQL Server started successfully

[3/5] Testing SQL connectivity...
Testing SQL connectivity...
  [1/2] Testing SELECT @@VERSION...
  ✅ @@VERSION query succeeded
  Version: Microsoft SQL Server 2025 (RTM) - 16.0.1000.6 (X64)
  [2/2] Testing SELECT @@SERVERNAME...
  ✅ @@SERVERNAME query succeeded
  Server Name: a1b2c3d4e5f6
✅ SQL connectivity tests passed

[4/5] Cleaning up resources...
✅ Cleanup complete

[5/5] Test completed
✅✅✅ Scenario PASSED: ubuntu/minimal

╔════════════════════════════════════════════════════════════╗
║ Test: polybase (ubuntu)
║ SQL Server + FTS + Polybase
╚════════════════════════════════════════════════════════════╝

Build command: ./build-and-run.sh --sa-password "***" --install-ollama false --polybase true

[1/5] Building Docker image...
✅ Build succeeded

[... continues for all scenarios ...]

════════════════════════════════════════════════════════════
Testing with base image: rhel
════════════════════════════════════════════════════════════

[... tests all RHEL scenarios ...]

════════════════════════════════════════════════════════════
                    Final Test Summary
════════════════════════════════════════════════════════════
Total Tests Run: 14
Tests Passed: 14
Tests Failed: 0

✅✅✅ All tests passed successfully! ✅✅✅
```

### Complete Test Suite Summary

```
╔══════════════════════════════════════════════════════════════════╗
║                    Complete Test Suite Summary                  ║
╚══════════════════════════════════════════════════════════════════╝

Test Duration: 42m 18s

Prerequisites: ✅ PASSED
Deployment Tests: ✅ PASSED

╔══════════════════════════════════════════════════════════════════╗
║              ✅✅✅ ALL TESTS PASSED! ✅✅✅                      ║
╚══════════════════════════════════════════════════════════════════╝

Your SQL AI Custom Container is working correctly!
```

## Example with Failures

If a test fails, you'll see detailed error information:

```
╔════════════════════════════════════════════════════════════╗
║ Test: full-stack (ubuntu)
║ All components enabled
╚════════════════════════════════════════════════════════════╝

[1/5] Building Docker image...
✅ Build succeeded

[2/5] Waiting for SQL Server to start...
Waiting for SQL Server to be ready (timeout: 120s)...
...........................
❌ Timeout waiting for SQL Server to start
❌ SQL Server failed to start

Container logs:
2024-01-20 10:15:23.45 spid51      Starting up database 'master'.
2024-01-20 10:15:23.48 spid51      ERROR: Could not allocate memory
2024-01-20 10:15:23.49 Server      SQL Server is terminating

[4/5] Cleaning up resources...
✅ Cleanup complete

════════════════════════════════════════════════════════════
                    Final Test Summary
════════════════════════════════════════════════════════════
Total Tests Run: 14
Tests Passed: 13
Tests Failed: 1

Failed Scenarios:
  ❌ ubuntu/full-stack - SQL Server startup failed

❌ Some tests failed. See details above.
```

## Individual Test Execution

### Prerequisites Only

```bash
./test-prerequisites.sh
```

Output shows just the 7 prerequisite checks.

### Deployments Only

```bash
./test-deployments.sh
```

Skips prerequisites and goes straight to building/testing deployments.

### Cleanup Only

```bash
./cleanup.sh
```

```
Starting cleanup of test resources...

[1/3] Cleaning up containers...
Removing container: sql-ai-test-ubuntu-minimal
✅ Container removed: sql-ai-test-ubuntu-minimal
[... continues for all containers ...]

[2/3] Cleaning up images...
Removing image: sql-ai-custom:test-ubuntu-minimal
✅ Image removed: sql-ai-custom:test-ubuntu-minimal
[... continues for all images ...]

[3/3] Cleaning up volumes...
Removing volumes matching: sqldata*
✅ Volume removed: sqldata
✅ Volume removed: ollama-models
✅ Volume removed: minio-data

Do you want to prune unused Docker resources (dangling images, stopped containers)? [y/N]
n

✅ Cleanup complete!
```

## Debugging Failed Tests

When running with `SKIP_CLEANUP=1`:

```bash
export SKIP_CLEANUP=1
./test-deployments.sh
```

Failed containers remain for inspection:

```
[4/5] Cleaning up resources...
⏭️  Skipping cleanup (SKIP_CLEANUP=1)

Note: Container 'sql-ai-test-ubuntu-full-stack' is still running for debugging.

To inspect:
  docker logs sql-ai-test-ubuntu-full-stack
  docker exec -it sql-ai-test-ubuntu-full-stack /bin/bash

To cleanup manually:
  ./cleanup.sh
```

## Test Timing Examples

Based on system specs:

| System | Prerequisites | Per Scenario | Total (14 scenarios) |
|--------|--------------|--------------|---------------------|
| High-end workstation | ~5s | ~2-3 min | ~30-45 min |
| Mid-range laptop | ~10s | ~3-4 min | ~45-60 min |
| CI/CD runner | ~5s | ~3-5 min | ~45-70 min |

**Note:** First run is slower due to:
- Base image downloads (~1.5GB for SQL Server)
- Ollama model downloads (~274MB for nomic-embed-text)
- Package installations

Subsequent runs are much faster with Docker layer caching.
