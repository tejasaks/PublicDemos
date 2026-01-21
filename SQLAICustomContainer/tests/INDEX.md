# SQL AI Custom Container - Test Suite Overview

## 📋 What This Test Suite Does

This comprehensive test suite validates all deployment configurations of the SQL Server + AI + Object Storage container across multiple base images and component combinations.

### Key Capabilities

✅ **Validates Prerequisites** - Checks Docker, SQL Server tools, passwords, disk space  
✅ **Tests 14 Deployment Scenarios** - 7 configurations × 2 base images (Ubuntu + RHEL)  
✅ **Automated Build & Deploy** - Fully automated container lifecycle testing  
✅ **SQL Connectivity Testing** - Verifies database queries (@@VERSION, @@SERVERNAME)  
✅ **Intelligent Cleanup** - Removes containers, images, and volumes automatically  
✅ **Failure Reporting** - Detailed error messages and logs  
✅ **Cross-Platform Support** - Linux and macOS  

## 🚀 Quick Start

```bash
cd tests
export SA_PASSWORD='YourComplexPass@123'
./run-all-tests.sh
```

## 📚 Documentation Index

| Document | Purpose | When to Read |
|----------|---------|--------------|
| **[README.md](README.md)** | Complete documentation with all details | First-time setup, troubleshooting |
| **[QUICKSTART.md](QUICKSTART.md)** | Quick reference for common tasks | Quick lookups, copy-paste commands |
| **[FILE-STRUCTURE.md](FILE-STRUCTURE.md)** | Detailed file descriptions and architecture | Understanding test implementation |
| **[EXAMPLE-OUTPUT.md](EXAMPLE-OUTPUT.md)** | Sample test execution output | Know what to expect when running tests |
| **[test-scenarios.conf](test-scenarios.conf)** | Test scenario configurations | Adding/modifying test cases |

## 🗂️ Test Files

| File | Type | Purpose |
|------|------|---------|
| `run-all-tests.sh` | Script | Main test orchestrator - runs everything |

| `test-prerequisites.sh` | Script | Validates system prerequisites (7 checks) |
| `test-deployments.sh` | Script | Tests all 14 deployment scenarios |
| `cleanup.sh` | Script | Removes test containers, images, volumes |
| `test-scenarios.conf` | Config | Defines all test scenarios |

## 🎯 Test Scenarios

The suite tests these 7 configurations with both Ubuntu and RHEL base images:

| # | Name | Components | Image Size | Memory |
|---|------|-----------|-----------|---------|
| 1 | **minimal** | SQL + FTS | ~3.5 GB | ~2 GB |
| 2 | **polybase** | SQL + FTS + Polybase | ~3.8 GB | ~2.5 GB |
| 3 | **minio** | SQL + FTS + MinIO | ~3.6 GB | ~2.5 GB |
| 4 | **ollama** | SQL + FTS + Ollama | ~5.5 GB | ~4 GB |
| 5 | **ollama-polybase** | SQL + FTS + Ollama + Polybase | ~5.8 GB | ~4.5 GB |
| 6 | **ollama-minio** | SQL + FTS + Ollama + MinIO | ~5.6 GB | ~5 GB |
| 7 | **full-stack** | All components | ~5.8 GB | ~5 GB |

**Total Tests: 14** (7 scenarios × 2 base images)

## ✅ What Gets Tested

### Prerequisites (7 checks)
1. Docker installation
2. Docker daemon status
3. Docker permissions
4. sqlcmd installation
5. sqlcmd PATH configuration
6. SA password validity
7. Available disk space

### Per Deployment Scenario (5 steps)
1. Build Docker image with specified components
2. Start container successfully
3. Wait for SQL Server to be ready (timeout: 120s)
4. Execute SQL queries: `SELECT @@VERSION` and `SELECT @@SERVERNAME`
5. Cleanup: remove container, image, and volumes

## ⚙️ Configuration

### Environment Variables

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| `SA_PASSWORD` | ✅ Yes | - | SQL Server SA password (must meet complexity) |
| `TEST_TIMEOUT` | ❌ No | 120 | Seconds to wait for SQL startup |
| `SKIP_CLEANUP` | ❌ No | 0 | Set to `1` to keep resources for debugging |

### Password Requirements
- Minimum 8 characters
- Uppercase letters
- Lowercase letters
- Digits
- Special characters (!@#$%^&*)

## 📊 Expected Results

### Success Output
```
╔══════════════════════════════════════════════════════════════════╗
║              ✅✅✅ ALL TESTS PASSED! ✅✅✅                      ║
╚══════════════════════════════════════════════════════════════════╝

Total Tests Run: 14
Tests Passed: 14
Tests Failed: 0

Test Duration: 42m 18s
```

### Failure Output
```
╔══════════════════════════════════════════════════════════════════╗
║                  ❌ SOME TESTS FAILED ❌                         ║
╚══════════════════════════════════════════════════════════════════╝

Total Tests Run: 14
Tests Passed: 12
Tests Failed: 2

Failed Scenarios:
  ❌ ubuntu/full-stack - SQL Server startup failed
  ❌ rhel/minimal - Build failed
```

## ⏱️ Test Duration

| Phase | Duration | Notes |
|-------|----------|-------|
| Prerequisites | ~5-10s | One-time checks |
| Per Scenario (cold) | ~4-5 min | First run with downloads |
| Per Scenario (warm) | ~2-3 min | Cached layers |
| **Total (14 scenarios)** | **30-60 min** | Full suite |

**First run is slower** due to:
- SQL Server base image download (~1.5 GB)
- Ollama model download (~274 MB)
- Package installations

## 🔍 Debugging

### Keep Failed Resources
```bash
export SKIP_CLEANUP=1
./test-deployments.sh
```

### Inspect Failed Container
```bash
docker logs sql-ai-test-ubuntu-minimal
docker exec -it sql-ai-test-ubuntu-minimal /bin/bash
```

### Check Build Logs
```bash
cat /tmp/build-minimal-ubuntu.log
```

### Manual Cleanup
```bash
./cleanup.sh
```

## 🛠️ Common Issues

| Problem | Solution |
|---------|----------|
| Docker not installed | Install Docker Desktop or Docker Engine |
| sqlcmd not found | Install SQL Server command-line tools |
| SA password error | Use complex password (8+ chars, mixed case, digits, special) |
| Timeout waiting for SQL | Increase `TEST_TIMEOUT=180` |
| Permission denied | Add user to `docker` group or use sudo |
| Disk space error | Free up at least 20 GB, 30GB+ recommended |

## 📖 Documentation Quick Links

- **Getting Started**: [README.md](README.md#running-tests)
- **Quick Commands**: [QUICKSTART.md](QUICKSTART.md#one-line-test-execution)
- **Troubleshooting**: [README.md](README.md#troubleshooting)
- **File Details**: [FILE-STRUCTURE.md](FILE-STRUCTURE.md#file-descriptions)
- **Example Output**: [EXAMPLE-OUTPUT.md](EXAMPLE-OUTPUT.md#expected-output)
- **CI/CD Integration**: [README.md](README.md#cicd-integration)

## 🔗 Integration Options

### GitHub Actions
```yaml
- name: Run Tests
  env:
    SA_PASSWORD: ${{ secrets.SA_PASSWORD }}
  run: |
    cd tests
    ./run-all-tests.sh
```

### Azure Pipelines
```yaml
- script: |
    cd tests
    export SA_PASSWORD=$(SA_PASSWORD)
    ./run-all-tests.sh
  displayName: 'Run Container Tests'
```

### GitLab CI
```yaml
test:
  script:
    - cd tests
    - export SA_PASSWORD=$SA_PASSWORD
    - ./run-all-tests.sh
```

## 📝 Test Maintenance

### Adding New Scenarios
1. Edit `test-scenarios.conf`
2. Add line: `name|ollama|minio|polybase|description`
3. Update documentation
4. Run tests to validate

### Modifying Timeouts
```bash
export TEST_TIMEOUT=180  # 3 minutes
./test-deployments.sh
```

### Testing Specific Base Image
Edit `test-deployments.sh`:
```bash
# BASE_IMAGES=("ubuntu" "rhel")
BASE_IMAGES=("ubuntu")  # Test Ubuntu only
```

## 🎓 Learning Resources

- [Docker Testing Best Practices](https://docs.docker.com/develop/dev-best-practices/)
- [SQL Server on Linux](https://docs.microsoft.com/sql/linux/)
- [Bash Testing Frameworks](https://github.com/sstephenson/bats)

## 📞 Support

For issues or questions:
1. Check [README.md](README.md) for detailed documentation
2. Review [EXAMPLE-OUTPUT.md](EXAMPLE-OUTPUT.md) for expected behavior
3. Run `./test-prerequisites.sh` to verify setup
4. Check logs in `/tmp/build-*.log`
5. Inspect container logs: `docker logs <container-name>`

## 🏆 Test Suite Features

- ✅ **Comprehensive** - Tests all deployment configurations
- ✅ **Automated** - No manual intervention required
- ✅ **Cross-Platform** - Linux, macOS, Windows support
- ✅ **Detailed Reporting** - Clear pass/fail with error details
- ✅ **Fast Cleanup** - Automatic resource removal
- ✅ **Debug Friendly** - Optional resource retention
- ✅ **CI/CD Ready** - Easy integration with pipelines
- ✅ **Well Documented** - Multiple guides for different needs

---

**Ready to test?** Start with the [Quick Start](#-quick-start) above!
