# Test Suite File Structure

```
tests/
├── README.md                  # Complete test documentation
├── QUICKSTART.md              # Quick reference guide
├── run-all-tests.sh          # Main test orchestrator
├── test-prerequisites.sh     # Validates Docker and SQL tools
├── test-deployments.sh       # Tests all deployment scenarios
├── test-scenarios.conf       # Configuration of test scenarios
└── cleanup.sh                # Resource cleanup helper
```

## File Descriptions

### Core Test Scripts

#### `run-all-tests.sh` (Main Orchestrator)
- **Purpose**: Runs complete test suite from prerequisites to deployments
- **Platform**: Linux, macOS
- **Usage**: `./run-all-tests.sh`
- **Features**:
  - Interactive prompts with confirmations
  - Two-phase execution (prerequisites → deployments)
  - Comprehensive summary with timing
  - Colored output for easy reading

#### `test-prerequisites.sh`
- **Purpose**: Validates system prerequisites
- **Tests**:
  1. Docker installation
  2. Docker daemon status
  3. Docker permissions
  4. sqlcmd installation
  5. sqlcmd PATH configuration
  6. SA password validation
  7. Available disk space
- **Exit Codes**: 0 = all passed, 1 = some failed

#### `test-deployments.sh`
- **Purpose**: Tests all deployment configurations
- **Tests**: 14 scenarios (7 configs × 2 base images)
- **Per Scenario**:
  1. Build Docker image
  2. Start container
  3. Wait for SQL Server (configurable timeout)
  4. Test SQL connectivity (@@VERSION, @@SERVERNAME)
  5. Cleanup resources (unless SKIP_CLEANUP=1)
- **Features**:
  - Parallel-ready architecture
  - Detailed logging to `/tmp/build-*.log`
  - Failure tracking with summary
  - Optional resource retention for debugging

#### `cleanup.sh`
- **Purpose**: Remove test containers, images, and volumes
- **Usage**: 
  - `./cleanup.sh` (clean test resources only)
  - `./cleanup.sh --all` (clean test resources AND default sqlserver-ollama)
- **Removes**:
  - Test containers (sql-ai-test-*)
  - Test images (sql-ai-custom:*)
  - Default container/image (when --all flag used)
  - Volumes (sqlserver_data, caddy_data, ollama_data, minio_data)
  - Optional: System-wide Docker prune

### Configuration

#### `test-scenarios.conf`
- **Format**: `NAME|OLLAMA|MINIO|POLYBASE|DESCRIPTION`
- **Scenarios**:
  1. minimal - SQL+FTS only
  2. polybase - SQL+FTS+Polybase
  3. minio - SQL+FTS+MinIO
  4. ollama - SQL+FTS+Ollama (default)
  5. ollama-polybase - SQL+FTS+Ollama+Polybase
  6. ollama-minio - SQL+FTS+Ollama+MinIO
  7. full-stack - All components

### Documentation

#### `README.md`
- **Complete test documentation**
- Sections:
  - Test structure and files
  - Prerequisites
  - Running tests (all options)
  - Test scenarios matrix
  - What gets tested
  - Output format
  - Environment variables
  - Troubleshooting guide
  - Advanced usage
  - CI/CD integration examples

#### `QUICKSTART.md`
- **Quick reference guide**
- Sections:
  - One-line test execution
  - Step-by-step instructions
  - Test matrix table
  - Expected output
  - Common troubleshooting
  - Environment variables
  - Duration estimates
  - CI/CD example

## Usage Patterns

### Scenario 1: First-Time Setup
```bash
cd tests
export SA_PASSWORD='YourComplexPass@123'
./test-prerequisites.sh           # Verify setup
./run-all-tests.sh               # Run full suite
```

### Scenario 2: Quick Validation
```bash
cd tests
export SA_PASSWORD='YourComplexPass@123'
./test-deployments.sh            # Skip prerequisites
```

### Scenario 3: Debug Failed Test
```bash
export SKIP_CLEANUP=1
./test-deployments.sh
# Container remains for inspection
docker logs sql-ai-test-ubuntu-minimal
docker exec -it sql-ai-test-ubuntu-minimal /bin/bash
# Manual cleanup when done
./cleanup.sh
```

### Scenario 4: CI/CD Pipeline
```yaml
steps:
  - name: Run Tests
    env:
      SA_PASSWORD: ${{ secrets.SA_PASSWORD }}
    run: |
      cd tests
      chmod +x *.sh
      ./run-all-tests.sh
```

## Environment Variables

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| `SA_PASSWORD` | ✅ | - | SQL Server SA password (complexity requirements apply) |
| `TEST_TIMEOUT` | ❌ | 120 | Seconds to wait for SQL Server startup |
| `SKIP_CLEANUP` | ❌ | 0 | Set to 1 to keep containers/images/volumes after tests |

## Test Output Format

### Success
```
╔══════════════════════════════════════════════════════════════════╗
║              ✅✅✅ ALL TESTS PASSED! ✅✅✅                      ║
╚══════════════════════════════════════════════════════════════════╝

Total Tests Run: 14
Tests Passed: 14
Tests Failed: 0
```

### Failure
```
╔══════════════════════════════════════════════════════════════════╗
║                  ❌ SOME TESTS FAILED ❌                         ║
╚══════════════════════════════════════════════════════════════════╝

Total Tests Run: 14
Tests Passed: 12
Tests Failed: 2

Failed Scenarios:
  ❌ ubuntu/minimal - Build failed
  ❌ rhel/full-stack - SQL connectivity failed
```

## Dependencies

### Required
- Docker (18.06+)
- SQL Server command-line tools (sqlcmd)
- Bash shell (Linux/macOS)

## Exit Codes

| Code | Meaning |
|------|---------|
| 0 | All tests passed |
| 1 | Prerequisites failed or deployment tests failed |

## Log Files

Test logs are written to `/tmp/`:
- `/tmp/build-<scenario>-<base-image>.log` - Build output for each scenario

## Best Practices

1. **Always run prerequisites first** on new systems
2. **Set appropriate timeout** for slower systems (`TEST_TIMEOUT=180`)
3. **Use SKIP_CLEANUP=1** when debugging failures
4. **Check disk space** before running full suite (~20GB recommended)
5. **Clean up** after debugging with `./cleanup.sh`
6. **Review logs** in `/tmp/` for build failures

## Extending Tests

To add new test scenarios:

1. Add line to `test-scenarios.conf`:
   ```
   new-scenario|true|true|true|Description here
   ```

2. Update documentation in `README.md` and `QUICKSTART.md`

3. Test with both base images:
   ```bash
   ./test-deployments.sh
   ```

## Maintenance

### Regular Tasks
- Update SQL Server base image versions in test scenarios
- Review and update timeout values for new hardware
- Add new deployment configurations as features are added
- Keep documentation in sync with script changes

### Monitoring
- Check `/tmp/build-*.log` files for build warnings
- Monitor test duration trends
- Review failure patterns in CI/CD

## Support

For issues or questions:
1. Check `README.md` for detailed documentation
2. Review `QUICKSTART.md` for common solutions
3. Run `./test-prerequisites.sh` to verify setup
4. Check logs in `/tmp/build-*.log`
5. Inspect containers: `docker logs <container-name>`
