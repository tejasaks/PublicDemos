# SQL Server 2025 + Ollama + Caddy Container

This custom container combines SQL Server 2025, Ollama AI runtime, and Caddy reverse proxy in a single deployable unit.

## Features

- **SQL Server 2025**: Latest SQL Server on Ubuntu or RHEL base (configurable via build argument)
- **SQL Server Full-Text Search (FTS)**: Advanced text searching and indexing capabilities
- **Automatic OS Detection**: Intelligently detects Ubuntu or RHEL base and uses appropriate package managers
- **Ollama**: AI model runtime with nomic-embed-text model pre-installed
- **Caddy**: Automatic HTTPS reverse proxy for Ollama (HTTP→HTTPS)
- **SSL Certificate Management**: Caddy's root certificate automatically trusted by SQL Server
- **Multi-OS Support**: Single Dockerfile works with both Ubuntu and RHEL base images

## Architecture

```
External Request (HTTPS:11435) 
    → Caddy (HTTPS termination)
        → Ollama (HTTP:11434)
            → nomic-embed-text model

SQL Server (1433) + FTS
    → Trusts Caddy CA certificates
    → Full-Text Search enabled

OS Detection Layer:
    ├─ Ubuntu/Debian → apt-get, dpkg
    └─ RHEL/CentOS  → yum/dnf, rpm
```

## Prerequisites

- Docker Engine 20.10+
- Minimum 4GB RAM available
- 20GB disk space for images and models

## Building the Container

> **⚠️ SECURITY NOTICE**: The SA password is mandatory and must be provided either:
> 1. **Via command-line parameter** (recommended): Use `--sa-password` flag when running the script
> 2. **By editing the script**: Manually set `SA_PASSWORD` variable in the build-and-run script
>
> The script will NOT run without a valid password set.

### Build with default Ubuntu base

```bash
# Using command-line parameter (recommended)
docker build -t sqlserver-ollama:2025 .

# OR using the build script with password
./build-and-run.sh --sa-password 'YourStrong@Pass123'
```

### Build with RHEL base image

```bash
docker build --build-arg BASE_IMAGE=mcr.microsoft.com/mssql/rhel/server:2025-latest -t sqlserver-ollama:2025-rhel .
```

### Build with custom tag

```bash
docker build -t yourusername/sqlserver-ollama:2025 .
```

### Using the build script (recommended)

**Option 1: Provide password via command-line (recommended)**
```bash
# Build with default Ubuntu image
./build-and-run.sh --sa-password 'YourStrong@Pass123'

# Build with RHEL image
./build-and-run.sh --base-image mcr.microsoft.com/mssql/rhel/server:2025-latest --sa-password 'YourStrong@Pass123'

# Windows PowerShell
.\build-and-run.ps1 -SAPassword 'YourStrong@Pass123' -BaseImage mcr.microsoft.com/mssql/rhel/server:2025-latest
```

**Option 2: Edit the script and set password**
```bash
# Edit build-and-run.sh (Linux/Mac) or build-and-run.ps1 (Windows)
# Set SA_PASSWORD="YourStrong@Pass123"

# Then run without the parameter
./build-and-run.sh
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
  -v sqlserver_data:/var/opt/mssql \
  -v ollama_data:/root/.ollama \
  -v caddy_data:/root/.local/share/caddy \
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
  -v sqlserver_data:/var/opt/mssql \
  -v ollama_data:/root/.ollama \
  -v caddy_data:/root/.local/share/caddy \
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
  -v sqlserver_data:/var/opt/mssql \
  -v ollama_data:/root/.ollama \
  -v caddy_data:/root/.local/share/caddy \
  yourusername/sqlserver-ollama:2025
```

## Quick Start

1. **Check logs:**
   ```bash
   docker logs -f sqlserver-ollama
   ```

2. **Connect to SQL Server:**
   - Host: `localhost`
   - Port: `1433`
   - Username: `sa`
   - Password: `YourStrong@Passw0rd` (set during docker run)

3. **Test Ollama via HTTPS:**
   ```bash
   curl -k https://localhost:11435/api/tags
   ```

4. **Test embeddings:**
   ```bash
   curl -k https://localhost:11435/api/embeddings -d '{
     "model": "nomic-embed-text",
     "prompt": "Hello, world!"
   }'
   ```

## Configuration

### SQL Server Password (Required)

Set the password via environment variable when running, or via script parameter when building:

**At runtime:**
```bash
-e MSSQL_SA_PASSWORD=YourStrong@Passw0rd
```

**At build time (using build script):**
```bash
--sa-password 'YourStrong@Pass123'  # Linux/Mac
-SAPassword 'YourStrong@Pass123'    # Windows PowerShell
```

**Password requirements:**
- At least 8 characters
- Contains uppercase, lowercase, digits, and special characters
- Cannot be empty or default value

### SQL Server Edition

Change the edition via environment variable:
```bash
-e MSSQL_PID=Developer    # Free developer edition
-e MSSQL_PID=Express      # Free express edition
-e MSSQL_PID=Standard     # Requires license
-e MSSQL_PID=Enterprise   # Requires license
```

### Resource Limits

Set memory limits when running:
```bash
docker run --memory="8g" --memory-reservation="4g" ...
```

### Additional Ollama Models

Add models at runtime:
```bash
docker exec sqlserver-ollama ollama pull llama2
docker exec sqlserver-ollama ollama pull codellama
```

Or modify the Dockerfile before building to include them automatically:
```bash
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

## Troubleshooting

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
