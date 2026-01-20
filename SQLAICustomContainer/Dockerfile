# Base OS layer: SQL Server 2025 (Ubuntu or RHEL)
# Can be overridden with --build-arg BASE_IMAGE=<image>
ARG BASE_IMAGE=mcr.microsoft.com/mssql/server:2025-latest
ARG ENABLE_POLYBASE=false
ARG INSTALL_OLLAMA=true
ARG INSTALL_MINIO=false
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

# Install Ollama if enabled
ARG INSTALL_OLLAMA=true
RUN if [ "${INSTALL_OLLAMA}" = "true" ]; then \
        echo "Ollama installation enabled" && \
        curl -fsSL https://ollama.com/install.sh | sh; \
    else \
        echo "Ollama installation skipped"; \
    fi

# Install MinIO if enabled (works for both Ubuntu and RHEL)
ARG INSTALL_MINIO=false
RUN if [ "${INSTALL_MINIO}" = "true" ]; then \
        echo "MinIO installation enabled" && \
        if [ -f /etc/debian_version ]; then \
            echo "Installing MinIO for Ubuntu/Debian" && \
            wget https://dl.min.io/server/minio/release/linux-amd64/minio -O /usr/local/bin/minio && \
            chmod +x /usr/local/bin/minio; \
        elif [ -f /etc/redhat-release ]; then \
            echo "Installing MinIO for RHEL" && \
            wget https://dl.min.io/server/minio/release/linux-amd64/minio -O /usr/local/bin/minio && \
            chmod +x /usr/local/bin/minio; \
        fi && \
        mkdir -p /minio/data && \
        mkdir -p /root/.minio/certs && \
        chmod -R 755 /minio; \
    else \
        echo "MinIO installation skipped"; \
    fi

# Create Caddyfile for reverse proxy configuration
RUN mkdir -p /etc/caddy

# Generate Caddyfile dynamically based on which services are installed
ARG INSTALL_OLLAMA=true
ARG INSTALL_MINIO=false
RUN printf '{\n    local_certs\n}\n' > /etc/caddy/Caddyfile && \
    if [ "${INSTALL_OLLAMA}" = "true" ]; then \
        printf '\nhttps://localhost:11435 {\n' >> /etc/caddy/Caddyfile && \
        printf '    tls internal\n' >> /etc/caddy/Caddyfile && \
        printf '    reverse_proxy localhost:11434\n' >> /etc/caddy/Caddyfile && \
        printf '    log {\n' >> /etc/caddy/Caddyfile && \
        printf '        output stdout\n' >> /etc/caddy/Caddyfile && \
        printf '        format console\n' >> /etc/caddy/Caddyfile && \
        printf '    }\n' >> /etc/caddy/Caddyfile && \
        printf '}\n' >> /etc/caddy/Caddyfile; \
    fi && \
    if [ "${INSTALL_MINIO}" = "true" ]; then \
        printf '\nhttps://localhost:9001 {\n' >> /etc/caddy/Caddyfile && \
        printf '    tls internal\n' >> /etc/caddy/Caddyfile && \
        printf '    reverse_proxy localhost:9002\n' >> /etc/caddy/Caddyfile && \
        printf '    log {\n' >> /etc/caddy/Caddyfile && \
        printf '        output stdout\n' >> /etc/caddy/Caddyfile && \
        printf '        format console\n' >> /etc/caddy/Caddyfile && \
        printf '    }\n' >> /etc/caddy/Caddyfile && \
        printf '}\n' >> /etc/caddy/Caddyfile; \
    fi

# Create SQL Server certificate directory
RUN mkdir -p /var/opt/mssql/security/ca-certificates

# Create startup script that will:
# 1. Generate MinIO TLS certificates (if MinIO is installed)
# 2. Start MinIO in background with TLS (if MinIO is installed)
# 3. Start Ollama in background (if Ollama is installed)
# 4. Pull the model (if Ollama is installed)
# 5. Start Caddy (always)
# 6. Copy Caddy and MinIO certificates to SQL Server cert directory
# 7. Update CA certificates
# 8. Start SQL Server
ARG INSTALL_OLLAMA=true
ARG INSTALL_MINIO=false
RUN cat > /opt/start-services.sh << EOF_SCRIPT
#!/bin/bash
set -e

# Build args passed as environment for runtime decision
INSTALL_OLLAMA="${INSTALL_OLLAMA}"
INSTALL_MINIO="${INSTALL_MINIO}"

# Ensure proper directory permissions
echo "Setting up directory permissions..."
mkdir -p /var/opt/mssql/security/ca-certificates
chown -R mssql:root /var/opt/mssql

if [ "\${INSTALL_OLLAMA}" = "true" ]; then
    echo "Ollama is installed - creating directories..."
    mkdir -p /root/.ollama
fi

if [ "\${INSTALL_MINIO}" = "true" ]; then
    echo "MinIO is installed - setting up..."
    mkdir -p /minio/data
    mkdir -p /root/.minio/certs
    
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
        echo "Detected Ubuntu/Debian - adding MinIO cert to system CA store..."
        mkdir -p /usr/local/share/ca-certificates
        cp /root/.minio/certs/public.crt \
           /usr/local/share/ca-certificates/minio-cert.crt
    elif [ -f /etc/redhat-release ]; then
        echo "Detected RHEL - adding MinIO cert to system CA store..."
        mkdir -p /etc/pki/ca-trust/source/anchors
        cp /root/.minio/certs/public.crt \
           /etc/pki/ca-trust/source/anchors/minio-cert.crt
    fi
    
    # Set default MinIO credentials if not provided
    export MINIO_ROOT_USER=\${MINIO_ROOT_USER:-minioadmin}
    export MINIO_ROOT_PASSWORD=\${MINIO_ROOT_PASSWORD:-minioadmin}
    
    echo "Starting MinIO service with TLS..."
    # Start MinIO server in background with TLS enabled
    MINIO_ROOT_USER=\$MINIO_ROOT_USER MINIO_ROOT_PASSWORD=\$MINIO_ROOT_PASSWORD \
    minio server /minio/data --address :9000 --console-address :9002 &
    MINIO_PID=\$!
    
    # Wait for MinIO to be ready
    echo "Waiting for MinIO to start..."
    for i in {1..30}; do
        if curl -k -s https://localhost:9000/minio/health/live >/dev/null 2>&1; then
            echo "MinIO is ready (TLS enabled)"
            break
        fi
        sleep 2
    done
fi

if [ "\${INSTALL_OLLAMA}" = "true" ]; then
    echo "Starting Ollama service..."
    # Start Ollama in background as root
    OLLAMA_HOST=0.0.0.0:11434 ollama serve &
    OLLAMA_PID=\$!
    
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
fi

echo "Starting Caddy server..."
# Start Caddy in background
caddy start --config /etc/caddy/Caddyfile --adapter caddyfile

# Wait for Caddy to generate certificates
echo "Waiting for Caddy to generate certificates..."
sleep 5

# Copy Caddy root certificate to SQL Server certificate directory (if Ollama or MinIO is installed)
if [ "\${INSTALL_OLLAMA}" = "true" ] || [ "\${INSTALL_MINIO}" = "true" ]; then
    if [ -f /root/.local/share/caddy/pki/authorities/local/root.crt ]; then
        echo "Copying Caddy root certificate..."
        cp /root/.local/share/caddy/pki/authorities/local/root.crt \
           /var/opt/mssql/security/ca-certificates/caddy-root.crt
        
        # Detect OS and copy to appropriate system-wide CA certificates location
        if [ -f /etc/debian_version ]; then
            echo "Detected Ubuntu/Debian - updating CA certificates..."
            mkdir -p /usr/local/share/ca-certificates
            cp /root/.local/share/caddy/pki/authorities/local/root.crt \
               /usr/local/share/ca-certificates/caddy-root.crt
            update-ca-certificates
        elif [ -f /etc/redhat-release ]; then
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
fi

# Update CA certificates
echo "Updating system CA certificates..."
if [ -f /etc/debian_version ]; then
    update-ca-certificates
elif [ -f /etc/redhat-release ]; then
    update-ca-trust
fi

echo "All services started. Starting SQL Server..."
# Start SQL Server as mssql user (this will run in foreground)
exec su -s /bin/bash mssql -c "/opt/mssql/bin/sqlservr"
EOF_SCRIPT

RUN chmod +x /opt/start-services.sh

# Create Ollama directories with proper permissions (conditional)
ARG INSTALL_OLLAMA=true
RUN if [ "${INSTALL_OLLAMA}" = "true" ]; then \
        mkdir -p /root/.ollama && \
        mkdir -p /home/mssql/.ollama && \
        chown -R mssql:mssql /home/mssql/.ollama; \
    fi

# Pre-pull the Ollama model during build to speed up container startup (conditional)
ARG INSTALL_OLLAMA=true
RUN if [ "${INSTALL_OLLAMA}" = "true" ]; then \
        (ollama serve &) && \
        sleep 10 && \
        (timeout 300 ollama pull nomic-embed-text || echo "Model pull will complete at runtime") && \
        pkill ollama || true; \
    fi

# Expose ports - always expose SQL Server (1433)
# Other ports depend on what's installed:
# 11435: HTTPS proxy for Ollama (if INSTALL_OLLAMA=true)
# 9000: MinIO API (if INSTALL_MINIO=true)
# 9001: MinIO Console HTTPS via Caddy (if INSTALL_MINIO=true)
# Note: Docker EXPOSE is documentation only, actual port mapping happens at runtime
# SQL Server will be started with proper user via sqlservr
USER root

# Use the custom startup script
CMD ["/opt/start-services.sh"]
