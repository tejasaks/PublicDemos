# Test Suite - Pre-Flight Checklist

Use this checklist before running the test suite to ensure everything is configured correctly.

## ✅ Prerequisites Checklist

### System Requirements

- [ ] **Docker installed** (version 18.06 or higher)
  ```bash
  docker --version
  ```

- [ ] **Docker daemon running**
  ```bash
  docker ps
  ```

- [ ] **User has Docker permissions** (member of `docker` group or admin)
  ```bash
  docker info
  ```

- [ ] **SQL Server command-line tools installed** (sqlcmd)
  ```bash
  sqlcmd -?
  ```

- [ ] **sqlcmd in PATH**
  ```bash
  which sqlcmd
  ```

- [ ] **At least 20GB free disk space**
  ```bash
  df -h .
  ```

### Configuration

- [ ] **SA Password set** with complexity requirements met:
  - ✓ Minimum 8 characters
  - ✓ Contains uppercase letters (A-Z)
  - ✓ Contains lowercase letters (a-z)
  - ✓ Contains digits (0-9)
  - ✓ Contains special characters (!@#$%^&*)
  
  ```bash
  export SA_PASSWORD='YourComplexPass@123'
  ```

- [ ] **Test timeout configured** (optional, default: 120s)
  ```bash
  export TEST_TIMEOUT=120
  ```

- [ ] **Cleanup behavior set** (optional, default: cleanup enabled)
  ```bash
  export SKIP_CLEANUP=0  # 0=cleanup, 1=keep resources
  ```

### Environment Setup

- [ ] **Navigate to tests directory**
  ```bash
  cd tests
  ```

- [ ] **Scripts are executable** (Linux/macOS)
  ```bash
  chmod +x *.sh
  ```
  
  > **Note:** After cloning from Git, shell scripts need to be made executable. See main README for details.

- [ ] **Network connectivity available** (for downloading base images and packages)
  ```bash
  ping mcr.microsoft.com
  ```

## 🚀 Running Tests

### Full Test Suite

```bash
# Set password
export SA_PASSWORD='YourComplexPass@123'

# Run all tests
./run-all-tests.sh
```

**Expected duration:** 30-60 minutes for full suite

### Individual Test Components

#### 1. Prerequisites Only
```bash
./test-prerequisites.sh
```
**Duration:** ~10 seconds

#### 2. Deployments Only
```bash
./test-deployments.sh
```
**Duration:** ~30-50 minutes

#### 3. Cleanup Only
```bash
./cleanup.sh
```
**Duration:** ~30 seconds


## 🐛 Debugging Checklist

If tests fail, work through this checklist:

### Container Won't Build

- [ ] Check Docker logs: `docker system df`
- [ ] Verify base image exists: `docker pull mcr.microsoft.com/mssql/server:2025-latest`
- [ ] Review build logs: `cat /tmp/build-<scenario>-<base>.log`
- [ ] Check network connectivity
- [ ] Verify disk space available

### Container Won't Start

- [ ] Check container logs: `docker logs <container-name>`
- [ ] Verify SA password complexity
- [ ] Check memory limits: `docker stats`
- [ ] Verify ports not already in use: `netstat -an | grep 1433`
- [ ] Review startup script errors in container logs

### SQL Server Won't Connect

- [ ] Verify SQL Server started: `docker logs <container-name> | grep "SQL Server is now ready"`
- [ ] Check sqlcmd connectivity: `sqlcmd -S localhost -U sa -P "$SA_PASSWORD" -Q "SELECT 1"`
- [ ] Increase timeout: `export TEST_TIMEOUT=180`
- [ ] Verify password is correct

### Tests Timeout

- [ ] Increase TEST_TIMEOUT: `export TEST_TIMEOUT=180`
- [ ] Check system resources: `docker stats`
- [ ] Verify no resource constraints
- [ ] Review container logs for errors

### Cleanup Issues

- [ ] Run manual cleanup: `./cleanup.sh`
- [ ] Force remove containers: `docker rm -f $(docker ps -aq)`
- [ ] Remove all test images: `docker images | grep sql-ai-custom | awk '{print $3}' | xargs docker rmi -f`
- [ ] Remove all volumes: `docker volume ls | grep -E 'sqldata|ollama-models|minio-data' | awk '{print $2}' | xargs docker volume rm`

## 📊 Post-Test Verification

After successful test run:

- [ ] **All 14 tests passed**
  ```
  Total Tests Run: 14
  Tests Passed: 14
  Tests Failed: 0
  ```

- [ ] **Resources cleaned up** (unless SKIP_CLEANUP=1)
  ```bash
  docker ps -a | grep sql-ai-test  # Should return nothing
  docker images | grep sql-ai-custom  # Should return nothing
  docker volume ls | grep -E 'sqldata|ollama-models|minio-data'  # Should return nothing
  ```

- [ ] **Test logs available** for review
  ```bash
  ls -la /tmp/build-*.log
  ```

## 🔄 Regular Maintenance

For ongoing test suite usage:

### Weekly
- [ ] Update Docker to latest version
- [ ] Pull latest base images
- [ ] Review and clean up old logs in `/tmp/`

### Monthly
- [ ] Update SQL Server command-line tools
- [ ] Review test scenarios for new configurations
- [ ] Check for Dockerfile updates

### After Code Changes
- [ ] Run full test suite
- [ ] Update test-scenarios.conf if new configurations added
- [ ] Update documentation to reflect changes

## 🎯 Success Criteria

All tests should meet these criteria:

### Prerequisites Tests (7 checks)
- ✅ Docker installed and running
- ✅ sqlcmd available and in PATH
- ✅ SA password valid
- ✅ Sufficient disk space

### Deployment Tests (14 scenarios)
- ✅ Images build successfully
- ✅ Containers start without errors
- ✅ SQL Server accepts connections
- ✅ Queries execute correctly
- ✅ Resources cleanup properly

## 📝 Troubleshooting Quick Reference

| Symptom | Check | Fix |
|---------|-------|-----|
| "Docker not found" | Docker installation | Install Docker Desktop |
| "sqlcmd not found" | SQL tools installation | Install mssql-tools |
| "SA password invalid" | Password complexity | Use strong password (8+ chars, mixed) |
| "Timeout waiting for SQL" | System performance | Increase TEST_TIMEOUT |
| "Permission denied" | Docker permissions | Add user to docker group |
| "No space left on device" | Disk space | Free up space, clean Docker |
| "Container stopped unexpectedly" | Container logs | Review: `docker logs <container>` |
| "Port already in use" | Port conflicts | Stop conflicting services |

## 🆘 Getting Help

If you're still stuck after working through this checklist:

1. **Review documentation**
   - [README.md](README.md) - Complete documentation
   - [QUICKSTART.md](QUICKSTART.md) - Quick reference
   - [EXAMPLE-OUTPUT.md](EXAMPLE-OUTPUT.md) - Expected behavior

2. **Check logs**
   - Build logs: `/tmp/build-*.log`
   - Container logs: `docker logs <container-name>`
   - Test output: Review terminal output

3. **Run individual tests**
   - Prerequisites: `./test-prerequisites.sh`
   - Single scenario: Edit `test-deployments.sh` to test one scenario

4. **Keep resources for debugging**
   ```bash
   export SKIP_CLEANUP=1
   ./test-deployments.sh
   # Inspect failed container
   docker logs <container-name>
   docker exec -it <container-name> /bin/bash
   ```

## ✨ Best Practices

- ✅ Always run prerequisites test first on new systems
- ✅ Use appropriate timeout for your system
- ✅ Review logs for warnings even if tests pass
- ✅ Clean up after debugging sessions
- ✅ Keep SA password secure (use environment variable, not command-line)
- ✅ Test on target platform before deploying
- ✅ Run full suite before merging code changes
- ✅ Document any custom configurations

---

**Ready to start?** Work through this checklist, then run:

```bash
cd tests
export SA_PASSWORD='YourComplexPass@123'
./run-all-tests.sh
```

Good luck! 🚀
