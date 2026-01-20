# SQL Server 2025 + Ollama + MinIO + Caddy Container

This custom container combines SQL Server 2025 with optional Ollama AI runtime, MinIO object storage, and Caddy reverse proxy in a single deployable unit. Each component can be enabled or disabled based on your requirements.

## Features

- **SQL Server 2025**: Latest SQL Server on Ubuntu or RHEL base (configurable via build argument)
- **SQL Server Full-Text Search (FTS)**: Advanced text searching and indexing capabilities (always installed)
- **SQL Server Polybase** (Optional): External data connectivity to MinIO and other sources
  - **Important**: Requires trace flag 13702 on Linux
  - **Note**: MinIO must be TLS-enabled for Polybase connectivity
- **Ollama** (Optional, default: enabled): AI model runtime with nomic-embed-text model pre-installed
- **MinIO** (Optional, default: disabled): S3-compatible object storage with web console and TLS encryption
- **Caddy** (Always installed): Automatic HTTPS reverse proxy for enabled services
- **Automatic OS Detection**: Intelligently detects Ubuntu or RHEL base and uses appropriate package managers
- **SSL Certificate Management**: Caddy's root certificate and MinIO TLS certificates automatically trusted by SQL Server
- **Multi-OS Support**: Single Dockerfile works with both Ubuntu and RHEL base images
- **Persistent Storage**: Volumes for SQL Server data and Caddy certificates (Ollama models and MinIO objects only when enabled)
- **Flexible Deployment**: Deploy SQL Server alone or with any combination of Polybase, Ollama, and MinIO

## Deployment Configurations

This container supports multiple deployment configurations:

1. **SQL Server + FTS only** (minimal)
   ```bash
   ./build-and-run.sh --sa-password 'YourPass@123' --install-ollama false
   ```

2. **SQL Server + FTS + Polybase**
   ```bash
   ./build-and-run.sh --sa-password 'YourPass@123' --install-ollama false --polybase true
   ```

3. **SQL Server + FTS + MinIO**
   ```bash
   ./build-and-run.sh --sa-password 'YourPass@123' --install-ollama false --install-minio true
   ```

4. **SQL Server + FTS + Polybase + MinIO**
   ```bash
   ./build-and-run.sh --sa-password 'YourPass@123' --install-ollama false --polybase true --install-minio true
   ```

5. **SQL Server + FTS + Ollama** (default AI-enabled setup)
   ```bash
   ./build-and-run.sh --sa-password 'YourPass@123'
   ```

6. **SQL Server + FTS + Ollama + Polybase**
   ```bash
   ./build-and-run.sh --sa-password 'YourPass@123' --polybase true
   ```

7. **Full Stack** (SQL + FTS + Polybase + Ollama + MinIO)
   ```bash
   ./build-and-run.sh --sa-password 'YourPass@123' --polybase true --install-minio true
   ```

## Architecture

The architecture adapts based on which components are installed:

```
[When Ollama is enabled]
External Request (HTTPS:11435) 
    → Caddy (HTTPS termination)
        → Ollama (HTTP:11434)
            → nomic-embed-text model

[When MinIO is enabled]
External Request (HTTPS:9001)
    → Caddy (HTTPS termination)
        → MinIO Console (HTTP:9002)

MinIO API (HTTPS:9000)
    → S3-compatible object storage (TLS enabled)
    → Persistent storage: /minio/data
    → TLS certificates: /root/.minio/certs/

[Always present]
SQL Server (1433) + FTS + Polybase*
    → Trusts Caddy CA certificates (when Ollama or MinIO enabled)
    → Full-Text Search enabled
    → Polybase for external data access*

Caddy Reverse Proxy (always installed)
    → Dynamically configured based on enabled services

OS Detection Layer:
    ├─ Ubuntu/Debian → apt-get, dpkg
    └─ RHEL/CentOS  → yum/dnf, rpm

* Polybase is optional, enable with --polybase true
```

## Prerequisites

- Docker Engine 20.10+
- Minimum 4GB RAM available
- 20GB disk space for images and models
- Linux, macOS, or WSL2 environment for build script

> **Base Image Options**: 
> - **Ubuntu** (default): `mcr.microsoft.com/mssql/server:2025-latest`
> - **RHEL**: `mcr.microsoft.com/mssql/rhel/server:2025-latest` 
> 
> Both images are published by Microsoft and support the same SQL Server features.
> To verify available tags: https://mcr.microsoft.com/en-us/product/mssql/rhel/server/tags

## Building the Container

> **⚠️ SECURITY NOTICE**: The SA password is mandatory and must be provided via command-line parameter when using the build script: `--sa-password 'YourStrong@Pass123'`
>
> The script will NOT run without a valid password set.

### Build with default configuration (Ubuntu base, Ollama enabled)

```bash
# Using Docker directly
docker build -t sqlserver-ollama:2025 .

# OR using the build script (recommended)
./build-and-run.sh --sa-password 'YourStrong@Pass123'
```

### Build with optional components

```bash
# Minimal: SQL Server + FTS only (no Ollama, no MinIO)
./build-and-run.sh --sa-password 'YourPass@123' --install-ollama false

# SQL Server + FTS + Polybase (no Ollama, no MinIO)
./build-and-run.sh --sa-password 'YourPass@123' --install-ollama false --polybase true

# SQL Server + FTS + MinIO (no Ollama)
./build-and-run.sh --sa-password 'YourPass@123' --install-ollama false --install-minio true

# Full stack: SQL + FTS + Polybase + Ollama + MinIO
./build-and-run.sh --sa-password 'YourPass@123' --polybase true --install-minio true
```

### Build with RHEL base image

```bash
# Default RHEL with Ollama
./build-and-run.sh --base-image mcr.microsoft.com/mssql/rhel/server:2025-latest --sa-password 'YourPass@123'

# RHEL with Polybase and MinIO
./build-and-run.sh --base-image mcr.microsoft.com/mssql/rhel/server:2025-latest --sa-password 'YourPass@123' --polybase true --install-minio true

# Minimal RHEL (no Ollama)
./build-and-run.sh --base-image mcr.microsoft.com/mssql/rhel/server:2025-latest --sa-password 'YourPass@123' --install-ollama false
```

### Manual Docker build with build args

```bash
# Full control over components
docker build \
  --build-arg BASE_IMAGE=mcr.microsoft.com/mssql/server:2025-latest \
  --build-arg ENABLE_POLYBASE=true \
  --build-arg INSTALL_OLLAMA=true \
  --build-arg INSTALL_MINIO=true \
  -t sqlserver-ollama:2025 .

# Minimal build (no optional components)
docker build \
  --build-arg ENABLE_POLYBASE=false \
  --build-arg INSTALL_OLLAMA=false \
  --build-arg INSTALL_MINIO=false \
  -t sqlserver-minimal:2025 .
```

### Push to Docker Hub (for sharing)

```bash
# Login to Docker Hub
docker login

# Tag the image
docker tag sqlserver-ollama:2025 yourusername/sqlserver-ollama:2025

# Push to registry
docker push yourusername/sqlserver-ollama:2025
```

## Running the Container

> **Note**: SQL Server SA password must be set. See security requirements above.

### Basic run command

```bash
docker run -d \
  --name sqlserver-ollama \
  -e ACCEPT_EULA=Y \
  -e MSSQL_SA_PASSWORD=YourStrong@Passw0rd \
  -e MSSQL_PID=Developer \
  -p 1433:1433 \
  -p 11435:11435 \
  -p 9000:9000 \
  -p 9001:9001 \
  -v sqlserver_data:/var/opt/mssql \
  -v ollama_data:/root/.ollama \
  -v caddy_data:/root/.local/share/caddy \
  -v minio_data:/minio/data \
  sqlserver-ollama:2025
```

**Important**: Replace `YourStrong@Passw0rd` with your own secure password.

### Run with custom memory limits

```bash
docker run -d \
  --name sqlserver-ollama \
  --memory="8g" \
  --memory-reservation="4g" \
  -e ACCEPT_EULA=Y \
  -e MSSQL_SA_PASSWORD=YourStrong@Passw0rd \
  -e MSSQL_PID=Developer \
  -p 1433:1433 \
  -p 11435:11435 \
  -p 9000:9000 \
  -p 9001:9001 \
  -v sqlserver_data:/var/opt/mssql \
  -v ollama_data:/root/.ollama \
  -v caddy_data:/root/.local/share/caddy \
  -v minio_data:/minio/data \
  sqlserver-ollama:2025
```

### Run from Docker Hub (after pushing)

```bash
docker run -d \
  --name sqlserver-ollama \
  -e ACCEPT_EULA=Y \
  -e MSSQL_SA_PASSWORD=YourStrong@Passw0rd \
  -e MSSQL_PID=Developer \
  -p 1433:1433 \
  -p 11435:11435 \
  -p 9000:9000 \
  -p 9001:9001 \
  -p 11435:11435 \
  -v sqlserver_data:/var/opt/mssql \
  -v ollama_data:/root/.ollama \
  -v caddy_data:/root/.local/share/caddy \
  yourusername/sqlserver-ollama:2025
```

## Quick Start

### Running the Container

1. **Check logs:**
   ```bash
   docker logs -f sqlserver-ollama
   ```

2. **Connect to SQL Server:**
   - Host: `localhost`
   - Port: `1433`
   - Username: `sa`
   - Password: `YourStrong@Passw0rd` (set during build)

3. **Test Ollama via HTTPS (if Ollama is installed):**
   ```bash
   curl -k https://localhost:11435/api/tags
   ```

4. **Test embeddings (if Ollama is installed):**
   ```bash
   curl -k https://localhost:11435/api/embeddings -d '{
     "model": "nomic-embed-text",
     "prompt": "Hello, world!"
   }'
   ```

5. **Test MinIO API (if MinIO is installed):**
   ```bash
   curl -k https://localhost:9000/minio/health/live
   ```

6. **Access MinIO Console (if MinIO is installed):**
   - URL: `https://localhost:9001` (accept self-signed certificate warning)
   - Username: `minioadmin`
   - Password: `minioadmin`
   - **Important**: Change default credentials in production!

## Configuration

### Component Selection

Control which components are installed using build parameters:

```bash
# Minimal: SQL Server + FTS only
./build-and-run.sh --sa-password 'YourPass@123' --install-ollama false

# Add Polybase support
./build-and-run.sh --sa-password 'YourPass@123' --install-ollama false --polybase true

# Add MinIO support
./build-and-run.sh --sa-password 'YourPass@123' --install-minio true

# Full stack with all components
./build-and-run.sh --sa-password 'YourPass@123' --polybase true --install-minio true
```

**Component parameters:**
- `--install-ollama`: `true` (default) or `false` - Install Ollama AI runtime
- `--install-minio`: `true` or `false` (default) - Install MinIO object storage
- `--polybase`: `true` or `false` (default) - Install SQL Server Polybase

### SQL Server Password (Required)

Set the password via script parameter when building:

```bash
./build-and-run.sh --sa-password 'YourStrong@Pass123'
```

**Password requirements:**
- At least 8 characters
- Contains uppercase, lowercase, digits, and special characters
- Cannot be empty

### SQL Server Edition

Change the edition via parameter:
```bash
./build-and-run.sh --sa-password 'YourPass@123' --edition Developer    # Free developer edition
./build-and-run.sh --sa-password 'YourPass@123' --edition Express      # Free express edition
./build-and-run.sh --sa-password 'YourPass@123' --edition Standard     # Requires license
./build-and-run.sh --sa-password 'YourPass@123' --edition Enterprise   # Requires license
```

### Resource Limits

Set memory limits via parameter:
```bash
./build-and-run.sh --sa-password 'YourPass@123' --memory 8g
```

### Additional Ollama Models (if Ollama is installed)

Add models at runtime:
```bash
docker exec sqlserver-ollama ollama pull llama2
docker exec sqlserver-ollama ollama pull codellama
```

Or modify the Dockerfile before building to include them automatically:
```dockerfile
ollama pull llama2
ollama pull codellama
```

### SQL Server Full-Text Search

FTS is automatically installed and configured. To use it:

```sql
-- Enable FTS on a database
USE YourDatabase;
GO

-- Create a full-text catalog
CREATE FULLTEXT CATALOG ftCatalog AS DEFAULT;
GO

-- Create a full-text index
CREATE FULLTEXT INDEX ON YourTable(TextColumn)
   KEY INDEX PK_YourTable;
GO

-- Search using full-text
SELECT * FROM YourTable
WHERE CONTAINS(TextColumn, 'search term');
```

### SQL Server Polybase Configuration

**Important**: Polybase on SQL Server for Linux requires trace flag 13702 to be enabled.

#### Enable Polybase (if built with --polybase true)

1. **Enable trace flag 13702:**
   ```bash
   # Enable trace flag 13702 globally
   docker exec -it sqlserver-ollama /opt/mssql/bin/mssql-conf traceflag 13702 on
   
   # Restart container for changes to take effect
   docker restart sqlserver-ollama
   ```

2. **Verify trace flag is enabled:**
   ```sql
   -- Check if trace flag 13702 is enabled
   DBCC TRACESTATUS(13702);
   ```

3. **Enable Polybase feature:**
   ```sql
   EXEC sp_configure @configname = 'polybase enabled', @configvalue = 1;
   RECONFIGURE WITH OVERRIDE;
   ```

4. **Create database and credentials:**
   ```sql
   CREATE DATABASE polybase_demo;
   GO
   
   USE polybase_demo;
   GO
   
   CREATE MASTER KEY ENCRYPTION BY PASSWORD = 'YourStrong@Pass123';
   GO
   
   -- Create credential for MinIO (format: 'username:password')
   CREATE DATABASE SCOPED CREDENTIAL minio_cred
   WITH IDENTITY = 'S3 Access Key',
        SECRET = 'minioadmin:minioadmin';
   GO
   
   -- Create external data source pointing to MinIO
   CREATE EXTERNAL DATA SOURCE minio_ds
   WITH (
       LOCATION = 's3://localhost:9000/',
       CREDENTIAL = minio_cred
   );
   GO
   ```

### MinIO Configuration and Usage

**Default Credentials:**
- Username: `minioadmin`
- Password: `minioadmin`
- **⚠️ IMPORTANT**: Change these in production!

**Access Points:**
- **API Endpoint**: `https://localhost:9000` (TLS enabled)
- **Web Console**: `https://localhost:9001` (proxied via Caddy)

**TLS Configuration:**
- MinIO automatically generates self-signed TLS certificates on startup
- Certificates are stored in `/root/.minio/certs/` inside the container
- Public certificate is automatically copied to:
  - SQL Server CA store: `/var/opt/mssql/security/ca-certificates/minio-cert.crt`
  - System CA store (Ubuntu): `/usr/local/share/ca-certificates/minio-cert.crt`
  - System CA store (RHEL): `/etc/pki/ca-trust/source/anchors/minio-cert.crt`

**Using MinIO with Python:**
```python
from minio import Minio

client = Minio(
    "localhost:9000",
    access_key="minioadmin",
    secret_key="minioadmin",
    secure=True,  # HTTPS enabled
    cert_check=False  # Accept self-signed certificate
)

# Create bucket
client.make_bucket("my-bucket")

# Upload file
client.fput_object("my-bucket", "file.txt", "/path/to/file.txt")
```

**Changing MinIO Credentials:**

Method 1 - Environment variables in docker run:
```bash
docker run -d \
  -e MINIO_ROOT_USER=myadmin \
  -e MINIO_ROOT_PASSWORD=MySecurePass123 \
  ...
```

Method 2 - Modify Dockerfile before building:
```bash
export MINIO_ROOT_USER=${MINIO_ROOT_USER:-mynewuser}
export MINIO_ROOT_PASSWORD=${MINIO_ROOT_PASSWORD:-mynewpassword}
```

## Ports

| Port  | Service                    | Protocol |
|-------|----------------------------|----------|
| 1433  | SQL Server                 | TCP      |
| 11435 | Ollama (via Caddy HTTPS)  | HTTPS    |

## Volumes

| Volume           | Purpose                           |
|------------------|-----------------------------------|
| sqlserver_data   | SQL Server databases and logs     |
| ollama_data      | Ollama models                     |
| caddy_data       | Caddy certificates and config     |

## Certificate Management

Caddy automatically generates self-signed certificates during first startup. The root CA certificate is:

1. Generated by Caddy at `/root/.local/share/caddy/pki/authorities/local/root.crt`
2. Copied to `/var/opt/mssql/security/ca-certificates/caddy-root.crt`
3. Added to system-wide CA trust store via `update-ca-certificates`

### Trusting Caddy Certificates on Your Machine

For client applications to trust the HTTPS endpoint, export the certificate:

```bash
docker cp sqlserver-ollama:/usr/local/share/ca-certificates/caddy-root.crt ./caddy-root.crt
```

Then install it on your system:
- **Windows**: Double-click → Install Certificate → Trusted Root Certification Authorities
- **Linux**: Copy to `/usr/local/share/ca-certificates/` and run `sudo update-ca-certificates`
- **macOS**: Add to Keychain Access → System → Certificates

## Testing

### Automated Test Suite

A comprehensive test suite is available to validate all deployment configurations:

```bash
cd tests
export SA_PASSWORD='YourComplexPass@123'
./run-all-tests.sh
```

The test suite validates:
- ✅ Docker and SQL Server tools installation
- ✅ All 14 deployment scenarios (7 configurations × 2 base images)
- ✅ SQL Server connectivity and basic queries
- ✅ Automatic cleanup of test resources

**Quick test options:**
```bash
# Prerequisites only
./test-prerequisites.sh

# Deployments only
./test-deployments.sh

# Cleanup test resources
./cleanup.sh
```

📖 **See [tests/README.md](tests/README.md) for complete testing documentation**

## Troubleshooting

### Build Issues

**Error: "failed to resolve source metadata" or "not found" for base image**

This occurs when the specified base image doesn't exist in the registry or you don't have network connectivity.

Solutions:
1. **Verify network connectivity**:
   ```bash
   docker pull mcr.microsoft.com/mssql/server:2025-latest
   ```

2. **Check available tags** (if specific version not found):
   ```bash
   # List available SQL Server RHEL tags
   curl -s https://mcr.microsoft.com/v2/mssql/rhel/server/tags/list | jq -r '.tags[]' | sort
   ```

3. **Verify image path format**:
   - ✅ **Correct Ubuntu**: `mcr.microsoft.com/mssql/server:2025-latest`
   - ✅ **Correct RHEL**: `mcr.microsoft.com/mssql/rhel/server:2025-latest`
   - ❌ Wrong: `mcr.microsoft.com/mssql/server:2025-latest-rhel`

**Error: Build fails during package installation**

Check if the base image requires different package names or repositories. SQL Server 2025 packages may not be available for all base OS versions.

### Polybase Issues

**Error: "External data sources are not supported with type GENERIC"**

Solution:
```bash
# Restart the container
docker restart sqlserver-ollama

# Wait for SQL Server to start, then retry the CREATE EXTERNAL DATA SOURCE command
```

**Error: Polybase queries fail or timeout**

1. Verify trace flag 13702 is enabled:
   ```sql
   DBCC TRACESTATUS(13702);
   ```

2. If not enabled, set it:
   ```bash
   docker exec -it sqlserver-ollama /opt/mssql/bin/mssql-conf traceflag 13702 on
   docker restart sqlserver-ollama
   ```

**Error: Cannot connect to MinIO from Polybase**

1. Verify MinIO is running with TLS:
   ```bash
   curl -k https://localhost:9000/minio/health/live
   ```

2. Check if MinIO certificate is in SQL Server CA store:
   ```bash
   docker exec sqlserver-ollama ls -la /var/opt/mssql/security/ca-certificates/
   # Should show minio-cert.crt
   ```

3. Verify certificate is trusted:
   ```bash
   docker exec sqlserver-ollama cat /etc/ssl/certs/ca-certificates.crt | grep -A 10 "MinIO"
   ```

### MinIO Issues

**Cannot access MinIO console at https://localhost:9001**

1. Check if MinIO is running:
   ```bash
   docker exec sqlserver-ollama ps aux | grep minio
   ```

2. Check MinIO logs:
   ```bash
   docker logs sqlserver-ollama | grep -i minio
   ```

3. Test MinIO API directly:
   ```bash
   curl -k https://localhost:9000/minio/health/live
   ```

**TLS/Certificate errors with MinIO**

1. Verify MinIO certificates exist:
   ```bash
   docker exec sqlserver-ollama ls -la /root/.minio/certs/
   # Should show private.key and public.crt
   ```

2. Regenerate certificates if needed:
   ```bash
   docker restart sqlserver-ollama
   # Certificates are auto-generated on startup
   ```

3. Check certificate validity:
   ```bash
   docker exec sqlserver-ollama openssl x509 -in /root/.minio/certs/public.crt -text -noout
   ```

**MinIO bucket or file access issues**

1. List buckets via API:
   ```bash
   # Install mc (MinIO client) locally if needed
   mc alias set myminio https://localhost:9000 minioadmin minioadmin --insecure
   mc ls myminio
   ```

2. Check MinIO Console logs in container:
   ```bash
   docker logs sqlserver-ollama 2>&1 | grep "MinIO"
   ```

3. Verify storage permissions:
   ```bash
   docker exec sqlserver-ollama ls -la /minio/data/
   ```

### General Issues

### Container fails to start

Check logs:
```bash
docker logs sqlserver-ollama
```

### Ollama not responding

Verify Ollama is running inside the container:
```bash
docker exec sqlserver-ollama curl http://localhost:11434/api/tags
```

### Model not available

Pull the model manually:
```bash
docker exec sqlserver-ollama ollama pull nomic-embed-text
```

### SQL Server connection issues

1. Verify SQL Server is running:
   ```bash
   docker exec sqlserver-ollama /opt/mssql-tools/bin/sqlcmd -S localhost -U sa -P 'YourStrong@Passw0rd' -Q "SELECT @@VERSION"
   ```

2. Check if port 1433 is accessible:
   ```bash
   telnet localhost 1433
   ```

### Certificate issues

Check if certificates are properly installed:
```bash
docker exec sqlserver-ollama ls -la /var/opt/mssql/security/ca-certificates/
docker exec sqlserver-ollama update-ca-certificates --fresh
```

### Base image OS detection

Verify which OS was detected during build:
```bash
# Check OS type
docker exec sqlserver-ollama cat /etc/os-release

# Check installed packages (Ubuntu)
docker exec sqlserver-ollama dpkg -l | grep mssql-server-fts

# Check installed packages (RHEL)
docker exec sqlserver-ollama rpm -qa | grep mssql-server-fts
```

## Maintenance

### Updating models

```bash
docker exec sqlserver-ollama ollama pull nomic-embed-text
```

### Backup SQL Server database

```bash
docker exec sqlserver-ollama /opt/mssql-tools/bin/sqlcmd -S localhost -U sa -P 'YourStrong@Passw0rd' -Q "BACKUP DATABASE [YourDB] TO DISK='/var/opt/mssql/backup/YourDB.bak'"
```

### Stop container

```bash
docker stop sqlserver-ollama
```

### Remove container

```bash
docker rm sqlserver-ollama
```

### Remove container and volumes

```bash
docker rm -f sqlserver-ollama
docker volume rm sqlserver_data ollama_data caddy_data
```

## Security Considerations

1. **Change default SA password** immediately
2. **Use secrets management** for production (Docker secrets or environment files)
3. **Limit network exposure** - only expose necessary ports
4. **Regular updates** - rebuild container with latest base images
5. **Certificate management** - consider using proper CA certificates for production

## License

This configuration uses:
- SQL Server 2025 (Microsoft EULA)
- Ollama (MIT License)
- Caddy (Apache 2.0)

Ensure compliance with all applicable licenses.

---

## Development

This solution was created using:
- **Visual Studio Code** - Development environment
- **GitHub Copilot** - AI-powered coding assistant
- **Claude Sonnet 4.5** - Advanced AI model for architecture and implementation
