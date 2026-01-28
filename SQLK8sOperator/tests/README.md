# MSSQL Operator Tests

This directory contains end-to-end tests for the MSSQL Kubernetes Operator.

## Test Categories

| Test | Description | Duration |
|------|-------------|----------|
| `test-operator-deployment.sh` | Validates operator installation | ~2 min |
| `test-crd-validation.sh` | Validates CRD schema and validation rules | ~1 min |
| `test-sqlserver-standalone.sh` | Tests standalone SQL Server deployment | ~5 min |
| `test-sqlserver-ag.sh` | Tests Availability Group deployment | ~10 min |
| `run-all-tests.sh` | Runs all tests in sequence | ~20 min |

## Prerequisites

- Kubernetes cluster (minikube, kind, or remote cluster)
- kubectl configured and connected
- Operator images built and available

## Setting Execute Permissions

Before running any test scripts, ensure they have execute permissions:

```bash
# From project root, set permissions on all shell scripts
chmod +x tests/*.sh
chmod +x scripts/*.sh

# Or set permissions on individual files
chmod +x tests/run-all-tests.sh
chmod +x tests/test-operator-deployment.sh
chmod +x tests/test-crd-validation.sh
chmod +x tests/test-sqlserver-standalone.sh
chmod +x tests/test-sqlserver-ag.sh
```

> **Note**: This step is required on Linux/macOS. Windows users running via WSL or Git Bash may also need to run this.

## Running Tests

### Run All Tests

```bash
./tests/run-all-tests.sh
```

### Run Individual Tests

```bash
./tests/test-operator-deployment.sh
./tests/test-crd-validation.sh
./tests/test-sqlserver-standalone.sh
./tests/test-sqlserver-ag.sh
```

### Run with Custom Namespace

```bash
TEST_NAMESPACE=my-test-ns ./tests/test-sqlserver-standalone.sh
```

## Test Behavior

- Each test creates its own namespace for isolation
- Tests clean up all resources after completion (pass or fail)
- Exit code 0 indicates success, non-zero indicates failure
- Verbose output is written to `tests/logs/` directory

## Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `TEST_NAMESPACE` | `mssql-test-*` | Namespace for test resources |
| `OPERATOR_NAMESPACE` | `mssql-system` | Namespace for operator |
| `OPERATOR_IMAGE` | `mssql-operator:dev` | Operator image to test |
| `SKIP_CLEANUP` | `false` | Set to `true` to preserve resources after test |
| `TIMEOUT` | `300` | Default timeout in seconds |

## Writing New Tests

Use the `test-helpers.sh` library for common functions:

```bash
#!/bin/bash
source "$(dirname "$0")/test-helpers.sh"

test_init "my-test-name"

# Your test logic here

test_cleanup
test_result
```
