# Build and Run Script for SQL Server + Ollama + Caddy Container
# Usage: .\build-and-run.ps1 [-ImageName <name>] [-Tag <tag>] [-ContainerName <name>] [-SAPassword <password>] [-Edition <edition>] [-Memory <size>]

param(
    [string]$ImageName = "sqlserver-ollama",
    [string]$Tag = "2025",
    [string]$ContainerName = "sqlserver-ollama",
    [string]$SAPassword = "",
    [ValidateSet("Developer", "Express", "Standard", "Enterprise")]
    [string]$Edition = "Developer",
    [string]$Memory = "8g",
    [string]$BaseImage = "mcr.microsoft.com/mssql/server:2025-latest",
    [switch]$Help
)

if ($Help) {
    Write-Host @"
Usage: .\build-and-run.ps1 [OPTIONS]

Options:
  -ImageName NAME       Docker image name (default: sqlserver-ollama)
  -Tag TAG              Docker image tag (default: 2025)
  -ContainerName NAME   Container name (default: sqlserver-ollama)
  -SAPassword PASSWORD  SQL Server SA password (REQUIRED - must be set)
  -Edition EDITION      SQL Server edition: Developer, Express, Standard, Enterprise (default: Developer)
  -Memory SIZE          Memory limit (default: 8g)
  -BaseImage IMAGE      Base SQL Server image (default: mcr.microsoft.com/mssql/server:2025-latest)
  -Help                 Show this help message

Examples:
  .\build-and-run.ps1
  .\build-and-run.ps1 -SAPassword 'MySecure@Pass123'
  .\build-and-run.ps1 -ImageName myuser/sqlserver-ollama -Tag latest
  .\build-and-run.ps1 -BaseImage mcr.microsoft.com/mssql/server:2025-latest-rhel
"@
    exit 0
}

$FullImageName = "${ImageName}:${Tag}"
$MemoryReservation = "4g"

# Validate SA password
if ([string]::IsNullOrEmpty($SAPassword)) {
    Write-Host "ERROR: SQL Server SA password is required!" -ForegroundColor Red
    Write-Host "You must set a password using -SAPassword parameter."
    Write-Host "Example: .\build-and-run.ps1 -SAPassword 'YourStrong@Pass123'"
    Write-Host ""
    Write-Host "Password requirements:"
    Write-Host "  - At least 8 characters"
    Write-Host "  - Contains uppercase, lowercase, digits, and special characters"
    exit 1
}

Write-Host "==========================================" -ForegroundColor Cyan
Write-Host "SQL Server + Ollama + Caddy Container" -ForegroundColor Cyan
Write-Host "==========================================" -ForegroundColor Cyan
Write-Host "Base Image: $BaseImage"
Write-Host "Output Image: $FullImageName"
Write-Host "Container: $ContainerName"
Write-Host "SQL Edition: $Edition"
Write-Host "Memory: $Memory"
Write-Host "==========================================" -ForegroundColor Cyan
Write-Host ""

# Build the image
Write-Host "Building Docker image..." -ForegroundColor Yellow
docker build --build-arg BASE_IMAGE=$BaseImage -t $FullImageName .

if ($LASTEXITCODE -ne 0) {
    Write-Host "Error: Docker build failed" -ForegroundColor Red
    exit 1
}

Write-Host ""
Write-Host "Build completed successfully!" -ForegroundColor Green
Write-Host ""

# Stop and remove existing container if it exists
$existingContainer = docker ps -a -q -f "name=$ContainerName"
if ($existingContainer) {
    Write-Host "Stopping and removing existing container..." -ForegroundColor Yellow
    docker rm -f $ContainerName
}

# Run the container
Write-Host "Starting container..." -ForegroundColor Yellow
docker run -d `
    --name $ContainerName `
    --memory=$Memory `
    --memory-reservation=$MemoryReservation `
    -e ACCEPT_EULA=Y `
    -e MSSQL_SA_PASSWORD=$SAPassword `
    -e MSSQL_PID=$Edition `
    -p 1433:1433 `
    -p 11435:11435 `
    -v sqlserver_data:/var/opt/mssql `
    -v ollama_data:/root/.ollama `
    -v caddy_data:/root/.local/share/caddy `
    $FullImageName

if ($LASTEXITCODE -ne 0) {
    Write-Host "Error: Failed to start container" -ForegroundColor Red
    exit 1
}

Write-Host ""
Write-Host "==========================================" -ForegroundColor Green
Write-Host "Container started successfully!" -ForegroundColor Green
Write-Host "==========================================" -ForegroundColor Green
Write-Host "Container name: $ContainerName"
Write-Host ""
Write-Host "Services:"
Write-Host "  • SQL Server:      localhost:1433"
Write-Host "  • Ollama (HTTPS):  https://localhost:11435"
Write-Host ""
Write-Host "SQL Server credentials:"
Write-Host "  • Username: sa"
Write-Host "  • Password: $SAPassword"
Write-Host ""
Write-Host "Useful commands:"
Write-Host "  • View logs:        docker logs -f $ContainerName"
Write-Host "  • Stop container:   docker stop $ContainerName"
Write-Host "  • Start container:  docker start $ContainerName"
Write-Host "  • Remove container: docker rm -f $ContainerName"
Write-Host ""
Write-Host "Testing Ollama:"
Write-Host "  curl -k https://localhost:11435/api/tags"
Write-Host ""
Write-Host "Waiting for services to start (this may take 2-3 minutes)..." -ForegroundColor Yellow
Write-Host "==========================================" -ForegroundColor Green

# Wait for container to be ready
Start-Sleep -Seconds 5

# Show logs
Write-Host ""
Write-Host "Container logs (Ctrl+C to exit):" -ForegroundColor Cyan
docker logs -f $ContainerName
