#!/bin/bash

# Build and Run Script for SQL Server + Ollama + Caddy Container
# Usage: ./build-and-run.sh [OPTIONS]

set -e

# Default values
IMAGE_NAME="sqlserver-ollama"
IMAGE_TAG="2025"
CONTAINER_NAME="sqlserver-ollama"
SA_PASSWORD=""
MSSQL_PID="Developer"
MEMORY_LIMIT="32g"
MEMORY_RESERVATION="4g"
BASE_IMAGE="mcr.microsoft.com/mssql/server:2025-latest"
ENABLE_POLYBASE="false"
INSTALL_OLLAMA="true"
INSTALL_MINIO="false"
NO_FOLLOW="false"

# Parse command line arguments
while [[ $# -gt 0 ]]; do
    case $1 in
        --image-name)
            IMAGE_NAME="$2"
            shift 2
            ;;
        --tag)
            IMAGE_TAG="$2"
            shift 2
            ;;
        --container-name)
            CONTAINER_NAME="$2"
            shift 2
            ;;
        --sa-password)
            SA_PASSWORD="$2"
            shift 2
            ;;
        --edition)
            MSSQL_PID="$2"
            shift 2
            ;;
        --memory)
            MEMORY_LIMIT="$2"
            shift 2
            ;;
        --base-image)
            BASE_IMAGE="$2"
            shift 2
            ;;
        --polybase)
            ENABLE_POLYBASE="$2"
            shift 2
            ;;
        --install-ollama)
            INSTALL_OLLAMA="$2"
            shift 2
            ;;
        --install-minio)
            INSTALL_MINIO="$2"
            shift 2
            ;;
        --no-follow)
            NO_FOLLOW="true"
            shift
            ;;
        --help)
            echo "Usage: ./build-and-run.sh [OPTIONS]"
            echo ""
            echo "Options:"
            echo "  --image-name NAME       Docker image name (default: sqlserver-ollama)"
            echo "  --tag TAG               Docker image tag (default: 2025)"
            echo "  --container-name NAME   Container name (default: sqlserver-ollama)"
            echo "  --sa-password PASSWORD  SQL Server SA password (REQUIRED - must be set)"
            echo "  --edition EDITION       SQL Server edition: Developer, Express, Standard, Enterprise (default: Developer)"
            echo "  --memory SIZE           Memory limit (default: 8g)"
            echo "  --base-image IMAGE      Base SQL Server image (default: mcr.microsoft.com/mssql/server:2025-latest)"
            echo "  --polybase BOOL         Enable SQL Server Polybase: true or false (default: false)"
            echo "  --install-ollama BOOL   Install Ollama AI runtime: true or false (default: true)"
            echo "  --install-minio BOOL    Install MinIO object storage: true or false (default: false)"
            echo "  --no-follow             Skip following container logs (useful for automation/testing)"
            echo "  --help                  Show this help message"
            echo ""
            echo "Examples:"
            echo "  ./build-and-run.sh --sa-password 'MySecure@Pass123'"
            echo "  ./build-and-run.sh --sa-password 'MySecure@Pass123' --polybase true"
            echo "  ./build-and-run.sh --sa-password 'MySecure@Pass123' --install-minio true"
            echo "  ./build-and-run.sh --sa-password 'MySecure@Pass123' --polybase true --install-ollama true --install-minio true"
            echo "  ./build-and-run.sh --sa-password 'MySecure@Pass123' --install-ollama false --install-minio false"
            echo "  ./build-and-run.sh --sa-password 'MySecure@Pass123' --image-name myuser/sqlserver-ollama --tag latest"
            echo "  ./build-and-run.sh --sa-password 'MySecure@Pass123' --base-image mcr.microsoft.com/mssql/rhel/server:2025-latest"
            exit 0
            ;;
        *)
            echo "Unknown option: $1"
            echo "Use --help for usage information"
            exit 1
            ;;
    esac
done

FULL_IMAGE_NAME="${IMAGE_NAME}:${IMAGE_TAG}"

# Validate SA password
if [ -z "${SA_PASSWORD}" ]; then
    echo "ERROR: SQL Server SA password is required!"
    echo "You must set a password using --sa-password flag."
    echo "Example: ./build-and-run.sh --sa-password 'YourStrong@Pass123'"
    echo ""
    echo "Password requirements:"
    echo "  - At least 8 characters"
    echo "  - Contains uppercase, lowercase, digits, and special characters"
    exit 1
fi

echo "=========================================="
echo "SQL Server + Ollama + Caddy + MinIO Container"
echo "=========================================="
echo "Base Image: ${BASE_IMAGE}"
echo "Output Image: ${FULL_IMAGE_NAME}"
echo "Container: ${CONTAINER_NAME}"
echo "SQL Edition: ${MSSQL_PID}"
echo "Polybase Enabled: ${ENABLE_POLYBASE}"
echo "Ollama Installed: ${INSTALL_OLLAMA}"
echo "MinIO Installed: ${INSTALL_MINIO}"
echo "Memory: ${MEMORY_LIMIT}"
echo "=========================================="
echo ""

# Build the image
echo "Building Docker image..."
docker build --build-arg BASE_IMAGE="${BASE_IMAGE}" --build-arg ENABLE_POLYBASE="${ENABLE_POLYBASE}" --build-arg INSTALL_OLLAMA="${INSTALL_OLLAMA}" --build-arg INSTALL_MINIO="${INSTALL_MINIO}" -t "${FULL_IMAGE_NAME}" .

if [ $? -ne 0 ]; then
    echo "Error: Docker build failed"
    exit 1
fi

echo ""
echo "Build completed successfully!"
echo ""

# Stop and remove existing container if it exists
if [ "$(docker ps -a -q -f name=${CONTAINER_NAME})" ]; then
    echo "Stopping and removing existing container..."
    docker rm -f "${CONTAINER_NAME}"
fi

# Run the container
echo "Starting container..."

# Build port mappings based on installed components
PORT_ARGS="-p 1433:1433"
if [ "${INSTALL_OLLAMA}" = "true" ]; then
    PORT_ARGS="${PORT_ARGS} -p 11435:11435"
fi
if [ "${INSTALL_MINIO}" = "true" ]; then
    PORT_ARGS="${PORT_ARGS} -p 9001:9001 -p 9000:9000"
fi

# Build volume mappings based on installed components
VOLUME_ARGS="-v sqlserver_data:/var/opt/mssql -v caddy_data:/root/.local/share/caddy"
if [ "${INSTALL_OLLAMA}" = "true" ]; then
    VOLUME_ARGS="${VOLUME_ARGS} -v ollama_data:/root/.ollama"
fi
if [ "${INSTALL_MINIO}" = "true" ]; then
    VOLUME_ARGS="${VOLUME_ARGS} -v minio_data:/minio/data"
fi

docker run -d \
    --name "${CONTAINER_NAME}" \
    --memory="${MEMORY_LIMIT}" \
    --memory-reservation="${MEMORY_RESERVATION}" \
    -e ACCEPT_EULA=Y \
    -e MSSQL_SA_PASSWORD="${SA_PASSWORD}" \
    -e MSSQL_PID="${MSSQL_PID}" \
    ${PORT_ARGS} \
    ${VOLUME_ARGS} \
    "${FULL_IMAGE_NAME}"

if [ $? -ne 0 ]; then
    echo "Error: Failed to start container"
    exit 1
fi

echo ""
echo "=========================================="
echo "Container started successfully!"
echo "=========================================="
echo "Container name: ${CONTAINER_NAME}"
echo ""
echo "Services:"
echo "  • SQL Server:       localhost:1433"
if [ "${INSTALL_OLLAMA}" = "true" ]; then
    echo "  • Ollama (HTTPS):   https://localhost:11435"
fi
if [ "${INSTALL_MINIO}" = "true" ]; then
    echo "  • MinIO API:        https://localhost:9000 (TLS enabled)"
    echo "  • MinIO Console:    https://localhost:9001"
fi
echo ""
echo "SQL Server credentials:"
echo "  • Username: sa"
echo "  • Password: ${SA_PASSWORD}"
echo ""
echo "Useful commands:"
echo "  • View logs:       docker logs -f ${CONTAINER_NAME}"
echo "  • Stop container:  docker stop ${CONTAINER_NAME}"
echo "  • Start container: docker start ${CONTAINER_NAME}"
echo "  • Remove container: docker rm -f ${CONTAINER_NAME}"
echo ""
if [ "${INSTALL_OLLAMA}" = "true" ]; then
    echo "Testing Ollama:"
    echo "  curl -k https://localhost:11435/api/tags"
    echo ""
fi
echo "Waiting for services to start (this may take 2-3 minutes)..."
echo "=========================================="

# Wait for container to be ready
sleep 5

if [ "${NO_FOLLOW}" = "true" ]; then
    echo ""
    echo "Container started successfully. Skipping log follow."
    echo "To view logs: docker logs -f ${CONTAINER_NAME}"
    echo ""
else
    # Show logs
    docker logs -f "${CONTAINER_NAME}"
fi
