# Base OS layer: SQL Server 2025 (Ubuntu or RHEL)
# Can be overridden with --build-arg BASE_IMAGE=<image>
ARG BASE_IMAGE=mcr.microsoft.com/mssql/server:2025-latest
ARG ENABLE_POLYBASE=false
FROM ${BASE_IMAGE}

# Switch to root for installations
USER root

# Set environment to non-interactive to avoid prompts during build
ENV DEBIAN_FRONTEND=noninteractive

# Install prerequisites and SQL Server FTS based on the base OS
RUN if [ -f /etc/debian_version ]; then \
        # Ubuntu/Debian-based image
        echo "Detected Ubuntu/Debian base image" && \
        apt-get update && \
        apt-get install -y \
            software-properties-common \
            wget \
            curl \
            ca-certificates \
            debian-keyring \
            debian-archive-keyring \
            apt-transport-https \
            gnupg \
            zstd && \
        # Add Microsoft SQL Server 2025 repository for Ubuntu
        wget -qO- https://packages.microsoft.com/keys/microsoft.asc | apt-key add - && \
        (add-apt-repository "$(wget -qO- https://packages.microsoft.com/config/ubuntu/22.04/mssql-server-2025.list)" || \
         echo "deb [arch=amd64,arm64] https://packages.microsoft.com/ubuntu/22.04/mssql-server-2025 jammy main" > /etc/apt/sources.list.d/mssql-server.list) && \
        apt-get update && \
        apt-get install -y mssql-server-fts && \
        rm -rf /var/lib/apt/lists/*; \
    elif [ -f /etc/redhat-release ]; then \
        # RHEL-based image
        echo "Detected RHEL base image" && \
        yum install -y --allowerasing wget curl ca-certificates gnupg2 zstd && \
        # Add Microsoft SQL Server 2025 repository for RHEL
        curl -o /etc/yum.repos.d/mssql-server.repo https://packages.microsoft.com/config/rhel/9/mssql-server-2025.repo && \
        yum install -y mssql-server-fts && \
        yum clean all; \
    else \
        echo "Warning: Unknown base OS, skipping FTS installation"; \
    fi

# Install SQL Server Polybase if enabled
ARG ENABLE_POLYBASE=false
RUN if [ "${ENABLE_POLYBASE}" = "true" ]; then \
        echo "Polybase installation enabled" && \
        if [ -f /etc/debian_version ]; then \
            # Ubuntu/Debian-based image
            echo "Installing Polybase for Ubuntu/Debian" && \
            apt-get update && \
            apt-get install -y mssql-server-polybase && \
            rm -rf /var/lib/apt/lists/*; \
        elif [ -f /etc/redhat-release ]; then \
            # RHEL-based image
            echo "Installing Polybase for RHEL" && \
            yum install -y mssql-server-polybase && \
            yum clean all; \
        fi; \
    else \
        echo "Polybase installation skipped"; \
    fi

# Install Caddy (works for both Ubuntu and RHEL)
RUN if [ -f /etc/debian_version ]; then \
        # Ubuntu/Debian - install Caddy via apt
        curl -1sLf 'https://dl.cloudsmith.io/public/caddy/stable/gpg.key' | gpg --dearmor -o /usr/share/keyrings/caddy-stable-archive-keyring.gpg && \
        curl -1sLf 'https://dl.cloudsmith.io/public/caddy/stable/debian.deb.txt' | tee /etc/apt/sources.list.d/caddy-stable.list && \
        apt-get update && \
        apt-get install -y caddy && \
        rm -rf /var/lib/apt/lists/*; \
    elif [ -f /etc/redhat-release ]; then \
        # RHEL - install Caddy via yum/dnf
        yum install -y --allowerasing yum-plugin-copr && \
        yum copr enable -y @caddy/caddy && \
        yum install -y caddy && \
        yum clean all; \
    fi

# Install Ollama
RUN curl -fsSL https://ollama.com/install.sh | sh

# Install MinIO (works for both Ubuntu and RHEL)
RUN if [ -f /etc/debian_version ]; then \
        # Ubuntu/Debian - install MinIO binary
        echo "Installing MinIO for Ubuntu/Debian" && \
        wget https://dl.min.io/server/minio/release/linux-amd64/minio -O /usr/local/bin/minio && \
        chmod +x /usr/local/bin/minio; \
    elif [ -f /etc/redhat-release ]; then \
        # RHEL - install MinIO binary
        echo "Installing MinIO for RHEL" && \
        wget https://dl.min.io/server/minio/release/linux-amd64/minio -O /usr/local/bin/minio && \
        chmod +x /usr/local/bin/minio; \
    fi

# Create MinIO directories
RUN mkdir -p /minio/data && \
    mkdir -p /root/.minio/certs && \
    chmod -R 755 /minio

# Create Caddyfile for reverse proxy configuration
RUN mkdir -p /etc/caddy
RUN cat > /etc/caddy/Caddyfile << 'EOF'
{
    # Global options
    # Enable local HTTPS with self-signed certificates
    local_certs
}

https://localhost:11435 {
    # Use internal CA for self-signed certificates
    tls internal
    
    # Reverse proxy to Ollama
    reverse_proxy localhost:11434
    
    # Enable logging
    log {
        output stdout
        format console
    }
}

https://localhost:9001 {
    # Use internal CA for self-signed certificates
    tls internal
    
    # Reverse proxy to MinIO Console
    reverse_proxy localhost:9002
    
    # Enable logging
    log {
        output stdout
        format console
    }
}
EOF

# Create SQL Server certificate directory
RUN mkdir -p /var/opt/mssql/security/ca-certificates

# Create startup script that will:
# 1. Generate MinIO TLS certificates
# 2. Start MinIO in background with TLS
# 3. Start Ollama in background
# 4. Pull the model
# 5. Start Caddy
# 6. Copy Caddy and MinIO certificates to SQL Server cert directory
# 7. Update CA certificates
# 8. Start SQL Server
RUN cat > /opt/start-services.sh << 'EOF'
#!/bin/bash
set -e

# Ensure proper directory permissions
echo "Setting up directory permissions..."
mkdir -p /root/.ollama
mkdir -p /var/opt/mssql/security/ca-certificates
mkdir -p /minio/data
mkdir -p /root/.minio/certs
chown -R mssql:root /var/opt/mssql

# Generate self-signed certificate for MinIO
echo "Generating TLS certificate for MinIO..."
openssl req -new -newkey rsa:2048 -days 3650 -nodes -x509 \
    -subj "/C=US/ST=State/L=City/O=Organization/CN=localhost" \
    -addext "subjectAltName=DNS:localhost,DNS:minio,IP:127.0.0.1" \
    -keyout /root/.minio/certs/private.key \
    -out /root/.minio/certs/public.crt

chmod 644 /root/.minio/certs/public.crt
chmod 600 /root/.minio/certs/private.key

echo "MinIO TLS certificate generated successfully"

# Copy MinIO certificate to SQL Server certificate directory
echo "Copying MinIO certificate to SQL Server CA store..."
cp /root/.minio/certs/public.crt \
   /var/opt/mssql/security/ca-certificates/minio-cert.crt
chown mssql:root /var/opt/mssql/security/ca-certificates/minio-cert.crt
chmod 644 /var/opt/mssql/security/ca-certificates/minio-cert.crt

# Copy MinIO certificate to system-wide CA store
if [ -f /etc/debian_version ]; then
    # Ubuntu/Debian
    echo "Detected Ubuntu/Debian - adding MinIO cert to system CA store..."
    mkdir -p /usr/local/share/ca-certificates
    cp /root/.minio/certs/public.crt \
       /usr/local/share/ca-certificates/minio-cert.crt
elif [ -f /etc/redhat-release ]; then
    # RHEL/CentOS
    echo "Detected RHEL - adding MinIO cert to system CA store..."
    mkdir -p /etc/pki/ca-trust/source/anchors
    cp /root/.minio/certs/public.crt \
       /etc/pki/ca-trust/source/anchors/minio-cert.crt
fi

# Set default MinIO credentials if not provided
export MINIO_ROOT_USER=${MINIO_ROOT_USER:-minioadmin}
export MINIO_ROOT_PASSWORD=${MINIO_ROOT_PASSWORD:-minioadmin}

echo "Starting MinIO service with TLS..."
# Start MinIO server in background with TLS enabled (console on port 9002, will be proxied to 9001 via HTTPS)
MINIO_ROOT_USER=$MINIO_ROOT_USER MINIO_ROOT_PASSWORD=$MINIO_ROOT_PASSWORD \
minio server /minio/data --address :9000 --console-address :9002 &
MINIO_PID=$!

# Wait for MinIO to be ready
echo "Waiting for MinIO to start..."
for i in {1..30}; do
    if curl -k -s https://localhost:9000/minio/health/live >/dev/null 2>&1; then
        echo "MinIO is ready (TLS enabled)"
        break
    fi
    sleep 2
done

echo "Starting Ollama service..."
# Start Ollama in background as root
OLLAMA_HOST=0.0.0.0:11434 ollama serve &
OLLAMA_PID=$!

# Wait for Ollama to be ready
echo "Waiting for Ollama to start..."
for i in {1..30}; do
    if curl -s http://localhost:11434/api/tags >/dev/null 2>&1; then
        echo "Ollama is ready"
        break
    fi
    sleep 2
done

# Pull the nomic-embed-text model
echo "Pulling nomic-embed-text model..."
ollama pull nomic-embed-text || echo "Note: Model pull will retry if needed"

echo "Starting Caddy server..."
# Start Caddy in background
caddy start --config /etc/caddy/Caddyfile --adapter caddyfile

# Wait for Caddy to generate certificates
echo "Waiting for Caddy to generate certificates..."
sleep 5

# Copy Caddy root certificate to SQL Server certificate directory
if [ -f /root/.local/share/caddy/pki/authorities/local/root.crt ]; then
    echo "Copying Caddy root certificate..."
    cp /root/.local/share/caddy/pki/authorities/local/root.crt \
       /var/opt/mssql/security/ca-certificates/caddy-root.crt
    
    # Detect OS and copy to appropriate system-wide CA certificates location
    if [ -f /etc/debian_version ]; then
        # Ubuntu/Debian
        echo "Detected Ubuntu/Debian - updating CA certificates..."
        mkdir -p /usr/local/share/ca-certificates
        cp /root/.local/share/caddy/pki/authorities/local/root.crt \
           /usr/local/share/ca-certificates/caddy-root.crt
        update-ca-certificates
    elif [ -f /etc/redhat-release ]; then
        # RHEL/CentOS
        echo "Detected RHEL - updating CA trust..."
        mkdir -p /etc/pki/ca-trust/source/anchors
        cp /root/.local/share/caddy/pki/authorities/local/root.crt \
           /etc/pki/ca-trust/source/anchors/caddy-root.crt
        update-ca-trust
    fi
    
    # Ensure SQL Server can read the certificate
    chown mssql:root /var/opt/mssql/security/ca-certificates/caddy-root.crt
    chmod 644 /var/opt/mssql/security/ca-certificates/caddy-root.crt
    
    echo "Certificate setup complete"
else
    echo "Warning: Caddy root certificate not found at expected location"
    # Try alternative location
    if [ -f /data/caddy/pki/authorities/local/root.crt ]; then
        cp /data/caddy/pki/authorities/local/root.crt \
           /var/opt/mssql/security/ca-certificates/caddy-root.crt
        
        if [ -f /etc/debian_version ]; then
            mkdir -p /usr/local/share/ca-certificates
            cp /data/caddy/pki/authorities/local/root.crt \
               /usr/local/share/ca-certificates/caddy-root.crt
            update-ca-certificates
        elif [ -f /etc/redhat-release ]; then
            mkdir -p /etc/pki/ca-trust/source/anchors
            cp /data/caddy/pki/authorities/local/root.crt \
               /etc/pki/ca-trust/source/anchors/caddy-root.crt
            update-ca-trust
        fi
        
        chown mssql:root /var/opt/mssql/security/ca-certificates/caddy-root.crt
        chmod 644 /var/opt/mssql/security/ca-certificates/caddy-root.crt
        echo "Certificate setup complete (alternative location)"
    fi
fi

# Update CA certificates for both Caddy and MinIO certs
echo "Updating system CA certificates..."
if [ -f /etc/debian_version ]; then
    update-ca-certificates
elif [ -f /etc/redhat-release ]; then
    update-ca-trust
fi

echo "All services started. Starting SQL Server..."
# Start SQL Server as mssql user (this will run in foreground)
# Use su to switch to mssql user for SQL Server process only
exec su -s /bin/bash mssql -c "/opt/mssql/bin/sqlservr"
EOF

RUN chmod +x /opt/start-services.sh

# Create Ollama directories with proper permissions
RUN mkdir -p /root/.ollama && \
    mkdir -p /home/mssql/.ollama && \
    chown -R mssql:mssql /home/mssql/.ollama

# Pre-pull the Ollama model during build to speed up container startup
# This is done in a way that doesn't fail the build if it times out
RUN (ollama serve &) && \
    sleep 10 && \
    (timeout 300 ollama pull nomic-embed-text || echo "Model pull will complete at runtime") && \
    pkill ollama || true

# Expose ports
# 1433: SQL Server
# 11435: HTTPS proxy for Ollama
# 9000: MinIO API
# 9001: MinIO Console (HTTPS via Caddy)
EXPOSE 1433 11435 9000 9001

# Keep running as root since we're orchestrating multiple services
# SQL Server will be started with proper user via sqlservr
USER root

# Use the custom startup script
CMD ["/opt/start-services.sh"]
