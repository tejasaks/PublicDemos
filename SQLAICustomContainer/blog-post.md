# Building an AI-Powered SQL Server Container: Combining SQL Server 2025 with Optional Ollama, MinIO, and Caddy for Flexible AI and Storage Solutions

> **📖 Quick Reference**: For detailed setup instructions, configuration options, and troubleshooting, see the [README](README.md).

## Introduction

In the rapidly evolving landscape of AI and data management, the need to combine traditional database systems with modern AI capabilities and object storage has never been more critical. Today, I'm excited to share a flexible solution that bridges this gap: a custom Docker container that integrates SQL Server 2025 with optional Ollama (AI model runtime), MinIO (S3-compatible object storage), and Caddy (HTTPS proxy server) components—all configurable based on your specific needs.

This solution enables you to deploy SQL Server in various configurations: from a minimal setup with just Full-Text Search, to a full-stack deployment with AI embeddings, object storage, and external data connectivity through Polybase. All with secure HTTPS communication, proper certificate management, and automatic adaptation to both Ubuntu and RHEL base images.

## The Challenge

Modern applications increasingly require:
- **Relational database capabilities** for structured data storage
- **AI/ML model inference** (optional) for embeddings, semantic search, and natural language processing
- **Object storage** (optional) for unstructured data like documents, images, and model artifacts
- **External data connectivity** (optional) for querying data stored in object storage
- **Secure communication** between services, especially in production environments
- **Flexible deployment** that doesn't require all components when not needed
- **Simple configuration** without complex orchestration

Traditionally, you'd either deploy a monolithic solution with all components (wasting resources) or manage multiple containers with complex networking. What if you could have the best of both worlds—a single container image that adapts to your exact requirements?

## The Solution Architecture

Our custom container provides flexible deployment with optional components:

```
┌────────────────────────────────────────────────────────────┐
│                    Docker Container                         │
│                 (Ubuntu or RHEL Base)                      │
├────────────────────────────────────────────────────────────┤
│                                                             │
│  ┌──────────────┐         ┌────────────────┐             │
│  │ SQL Server   │         │ Ollama Runtime │ (Optional)  │
│  │ 2025 + FTS   │         │ + nomic-embed  │             │
│  │ + Polybase*  │         │ Port: 11434    │             │
│  │ Port: 1433   │         └────────┬───────┘             │
│  └──────────────┘                  │                      │
│         │                           │                      │
│         │                  ┌────────▼───────┐             │
│         │                  │ Caddy Proxy    │ (Always)   │
│         │                  │ HTTPS:11435*   │             │
│         │                  │ HTTPS:9001*    │             │
│         │                  └────────┬───────┘             │
│         │                           │                      │
│    Trusted CA ◄────────────────────┘                      │
│    Certificates                                            │
│                                                             │
│  ┌────────────────────────────────────┐                   │
│  │ MinIO Object Storage               │ (Optional)        │
│  │ S3-Compatible API (Port: 9000)     │                   │
│  │ Web Console (Port: 9002→9001)      │                   │
│  │ Persistent Storage: /minio/data    │                   │
│  └────────────────────────────────────┘                   │
│                                                             │
│  OS Detection: Ubuntu/Debian or RHEL/CentOS               │
│  * Optional components, controlled by build args          │
└────────────────────────────────────────────────────────────┘
          │         │            │            │
          │         │            │            │
     Port 1433  Port 11435* Port 9000*   Port 9001*
   (SQL Server) (Ollama)   (MinIO API)  (MinIO HTTPS)
   
   * Ports only exposed when respective components are installed
```

### Deployment Configurations

The container supports seven different deployment configurations:

1. **Minimal**: SQL Server + FTS only
2. **Data Platform**: SQL + FTS + Polybase
3. **SQL + Storage**: SQL + FTS + MinIO
4. **Full Data Platform**: SQL + FTS + Polybase + MinIO
5. **AI-Enabled (Default)**: SQL + FTS + Ollama
6. **AI + Data**: SQL + FTS + Ollama + Polybase
7. **Full Stack**: SQL + FTS + Polybase + Ollama + MinIO

### Component Breakdown

**1. SQL Server 2025 (Ubuntu or RHEL Base)** - *Always Installed*
- Latest SQL Server running on Ubuntu or RHEL (configurable at build time)
- Full-Text Search (FTS) enabled for advanced text indexing and searching
- Runs as the `mssql` user for security
- Full T-SQL capabilities with AI integration potential
- Trusts Caddy's CA certificates for secure outbound connections (when Ollama or MinIO enabled)

**2. Ollama AI Runtime** - *Optional (Default: Enabled)*
- Runs the `nomic-embed-text` model (pre-pulled during build)
- Provides REST API for embeddings generation
- Can be extended with additional models (llama2, codellama, etc.)
- HTTP endpoint on port 11434
- Controlled by `--install-ollama true|false` parameter

**3. Caddy HTTPS Proxy** - *Always Installed*
- Automatically generates self-signed certificates
- Dynamically configured based on installed components
- Reverse proxies Ollama HTTP → HTTPS (when Ollama enabled)
- Reverse proxies MinIO Console HTTP → HTTPS (when MinIO enabled)
- Certificate authority chain trusted system-wide
- Production-ready TLS configuration
- Lightweight with minimal resource overhead

**4. MinIO Object Storage** - *Optional (Default: Disabled)*
- S3-compatible object storage service
- REST API on port 9000 with TLS encryption
- Web console on port 9001 (HTTPS via Caddy)
- Persistent storage at /minio/data
- Ideal for storing unstructured data, model artifacts, and backups
- Integrates with SQL Server via Polybase for external table access
- Controlled by `--install-minio true|false` parameter

**5. SQL Server Polybase** - *Optional (Default: Disabled)*
- External data connectivity for querying data in object storage
- Requires trace flag 13702 on Linux
- Works with MinIO S3-compatible storage
- Enables hybrid data architectures
- Controlled by `--polybase true|false` parameter

## Key Features

### 🎛️ Flexible Deployment
- Choose exactly which components you need
- Minimal resource footprint for simple deployments
- Full-featured stack available when needed
- Single container image supports all configurations
- Build-time component selection

### 🔒 Security First
- SQL Server runs as unprivileged `mssql` user
- HTTPS encryption for AI model API calls (when Ollama enabled)
- MinIO TLS encryption with automatic certificate generation (when MinIO enabled)
- Automatic certificate management
- System-wide CA trust store integration

### 🎯 Intelligent OS Detection
- Automatically detects Ubuntu/Debian or RHEL/CentOS base
- Uses appropriate package managers (apt-get vs yum/dnf)
- Single Dockerfile works with multiple base images
- Proper repository configuration for each OS

### 🚀 Performance Optimized
- AI model pre-pulled during image build (when Ollama enabled)
- Single container reduces network latency
- Direct in-memory communication between services
- Efficient resource utilization
- Only install what you need

### 📦 Production Ready
- Persistent volumes for data (and models/objects when enabled)
- Configurable memory limits
- Comprehensive logging
- Health check compatible
- Conditional service startup

### 🔧 Developer Friendly
- Single build script with clear parameters
- Automated startup orchestration
- Clear documentation and examples
- Easy to extend with additional models or features

## Deployment Configuration Examples

The container's flexibility allows you to deploy exactly what you need:

```bash
# 1. Minimal SQL Server deployment (just FTS)
./build-and-run.sh --sa-password 'YourPass@123' --install-ollama false

# 2. SQL Server with data platform capabilities
./build-and-run.sh --sa-password 'YourPass@123' --install-ollama false --polybase true

# 3. SQL Server with object storage
./build-and-run.sh --sa-password 'YourPass@123' --install-ollama false --install-minio true

# 4. SQL Server with AI capabilities (default)
./build-and-run.sh --sa-password 'YourPass@123'

# 5. Full data + AI platform
./build-and-run.sh --sa-password 'YourPass@123' --polybase true --install-minio true
```

**Resource Impact by Configuration:**

| Configuration | Image Size* | Memory Footprint** | Use Case |
|--------------|-------------|-------------------|----------|
| Minimal (SQL+FTS) | ~3.5 GB | ~2 GB | Traditional SQL workloads |
| +Polybase | ~3.8 GB | ~2.5 GB | External data queries |
| +MinIO | ~3.6 GB | ~2.5 GB | Object storage integration |
| +Ollama (default) | ~5.5 GB | ~4 GB | AI/ML workloads |
| Full Stack | ~5.8 GB | ~5 GB | Complete data+AI platform |

*Approximate compressed image size
**Typical runtime memory usage under light load

## Implementation Highlights

### Conditional Component Installation

The Dockerfile uses build arguments to conditionally install components:

```dockerfile
ARG INSTALL_OLLAMA=true
ARG INSTALL_MINIO=false
ARG ENABLE_POLYBASE=false

# Install Ollama if enabled
RUN if [ "${INSTALL_OLLAMA}" = "true" ]; then \
        curl -fsSL https://ollama.com/install.sh | sh; \
    fi

# Install MinIO if enabled  
RUN if [ "${INSTALL_MINIO}" = "true" ]; then \
        wget https://dl.min.io/server/minio/release/linux-amd64/minio && \
        chmod +x minio && \
        mkdir -p /minio/data /root/.minio/certs; \
    fi
```

Benefits:
- **Reduced image size** when components aren't needed
- **Faster builds** skipping unused software
- **Lower attack surface** with minimal installations
- **Flexible deployment** matching your requirements

### Dynamic Caddy Configuration

Caddyfile generation adapts to installed components:

```dockerfile
RUN echo "{ local_certs }" > /etc/caddy/Caddyfile && \
    if [ "${INSTALL_OLLAMA}" = "true" ]; then \
        # Add Ollama HTTPS proxy configuration
    fi && \
    if [ "${INSTALL_MINIO}" = "true" ]; then \
        # Add MinIO Console HTTPS proxy configuration
    fi
```

This ensures Caddy only proxies services that are actually installed.

### Intelligent OS Detection

One of the most powerful features of this solution is its ability to work with both Ubuntu and RHEL base images without any code changes. The Dockerfile intelligently detects the operating system at build time:

```dockerfile
RUN if [ -f /etc/debian_version ]; then \
        # Ubuntu/Debian detected
        apt-get update && \
        apt-get install -y mssql-server-fts && \
        # Add Ubuntu repositories
        wget -qO- https://packages.microsoft.com/.../ubuntu/22.04/mssql-server-2025.list
    elif [ -f /etc/redhat-release ]; then \
        # RHEL detected
        yum install -y mssql-server-fts && \
        # Add RHEL repositories
        curl -o /etc/yum.repos.d/mssql-server.repo https://packages.microsoft.com/.../rhel/9/mssql-server-2025.repo
    fi
```

This means:
- **Same Dockerfile** works for both Ubuntu and RHEL
- **Automatic package manager selection** (apt-get vs yum/dnf)
- **Correct repository configuration** for SQL Server 2025 FTS
- **Build-time OS detection** with no runtime overhead

### The Startup Orchestration

The startup script dynamically adjusts based on installed components:

```bash
1. Setup directory permissions
2. [If MinIO] Generate TLS certificates and start MinIO
3. [If Ollama] Start Ollama service and pull models
4. Start Caddy with dynamic configuration
5. Copy certificates to SQL Server trust store (if needed)
6. Update system CA certificates
7. Start SQL Server as mssql user
```

This sequence ensures that:
- Only enabled services are started
- Services start in dependency order
- Certificates are generated before SQL Server starts (when needed)
- SQL Server trusts the required CAs
- Each service has proper permissions

### Certificate Trust Chain

A unique aspect of this solution is how we establish trust between SQL Server and the Ollama HTTPS endpoint:

```bash
# Caddy generates certificates
/root/.local/share/caddy/pki/authorities/local/root.crt

# Copy to SQL Server certificate directory
→ /var/opt/mssql/security/ca-certificates/caddy-root.crt

# Update system-wide CA trust
→ update-ca-certificates

# SQL Server can now trust HTTPS calls to Ollama
```

This enables SQL Server to make secure HTTPS calls to Ollama for AI operations without certificate validation errors.

### MinIO Object Storage Integration

MinIO provides enterprise-grade, S3-compatible object storage that seamlessly integrates with SQL Server through Polybase. This integration enables:

**Key Benefits:**
- **Unstructured Data Storage**: Store documents, images, videos, and model artifacts
- **SQL Server Integration**: Query object storage data directly using SQL via Polybase external tables
- **S3 Compatibility**: Works with any S3-compatible client or SDK
- **Persistent Storage**: Data persisted in Docker volume for durability
- **Secure Access**: Web console secured via Caddy HTTPS proxy
- **TLS Encryption**: All MinIO API traffic encrypted with auto-generated certificates

**Important Prerequisites for Polybase Integration:**
- ⚠️ **Trace flag 13702 required**: Polybase on SQL Server for Linux requires trace flag 13702
- ⚠️ **TLS enabled**: MinIO must have TLS enabled for SQL Server Polybase connectivity
- ⚠️ **Certificate trust**: MinIO certificate must be in SQL Server's trusted CA store

**MinIO Architecture:**
```
MinIO Server (Port 9000)  ←→  S3 API Access (HTTPS/TLS Enabled)
     ↓
MinIO Console (Port 9002) → Caddy HTTPS Proxy (Port 9001)
     ↓
Persistent Volume: /minio/data
     ↓
TLS Certificates: /root/.minio/certs/
```

**Default Credentials:**
- Username: `minioadmin`
- Password: `minioadmin`
- Console URL: `https://localhost:9001`
- API Endpoint: `https://localhost:9000` (TLS enabled with self-signed certificate)

**TLS and Certificate Management:**
- MinIO automatically generates self-signed TLS certificates on container startup
- Certificates stored in `/root/.minio/certs/` (private.key and public.crt)
- Public certificate automatically copied to:
  - SQL Server CA store: `/var/opt/mssql/security/ca-certificates/minio-cert.crt`
  - System CA store (Ubuntu): `/usr/local/share/ca-certificates/minio-cert.crt`
  - System CA store (RHEL): `/etc/pki/ca-trust/source/anchors/minio-cert.crt`
- System CA certificates updated automatically to trust MinIO certificate

**Important Security Note**: Change default credentials in production by setting environment variables `MINIO_ROOT_USER` and `MINIO_ROOT_PASSWORD` in the startup script or via Docker environment variables.

## Real-World Use Cases

### 1. Advanced Text Search with FTS + AI Embeddings

**Note**: This example uses the default container configuration where **Full-Text Search (FTS)** and **Ollama** are installed by default, enabling local AI model inference without external dependencies.

**For Remote AI Models (Azure OpenAI, etc.)**: If you plan to use remote AI services like Azure OpenAI instead of local Ollama models, you can significantly reduce the container size by disabling Ollama during build:

```bash
# Minimal container for FTS + remote AI (no Ollama)
./build-and-run.sh --sa-password "YourPass@123" --install-ollama false

# This reduces the image size from ~5.5GB to ~3.5GB and saves ~2GB of memory
# Then use CREATE EXTERNAL MODEL with Azure OpenAI endpoint instead of localhost:11435
```

**SQL Implementation with Local Ollama (Default Configuration):**

```sql
-- Create and switch to a new database
CREATE DATABASE AIDocuments;
GO

USE AIDocuments;
GO

-- Enable preview features for vector search and indexing
ALTER DATABASE SCOPED CONFIGURATION
SET PREVIEW_FEATURES = ON;
GO

-- Create full-text catalog and index
CREATE FULLTEXT CATALOG DocumentCatalog AS DEFAULT;
GO

CREATE TABLE Documents (
    ID INT CONSTRAINT PK_Documents PRIMARY KEY,
    Title NVARCHAR(200),
    Content NVARCHAR(1024),
    Embedding VECTOR(768)  -- 768 dimensions for nomic-embed-text model
);
GO

CREATE FULLTEXT INDEX ON Documents(Title, Content)
   KEY INDEX PK_Documents;
GO

-- Create external model pointing to Ollama via HTTPS proxy
IF EXISTS (SELECT * FROM sys.external_models WHERE name = 'MyOllamaEmbeddingModel')
DROP EXTERNAL MODEL MyOllamaEmbeddingModel;
GO

CREATE EXTERNAL MODEL MyOllamaEmbeddingModel
WITH (
    LOCATION = 'https://localhost:11435/api/embed',  -- HTTPS proxy to Ollama
    API_FORMAT = 'Ollama',                             -- Provider type
    MODEL_TYPE = EMBEDDINGS,                           -- text embedding model
    MODEL = 'nomic-embed-text'
    --PARAMETERS = '{"temperature":1,"max_tokens":1024}'  -- Optional runtime settings
);
GO

-- Insert sample AI/ML related documents
INSERT INTO Documents (ID, Title, Content) VALUES
(1, 'Introduction to Machine Learning', 'Machine learning is a subset of artificial intelligence that enables systems to learn and improve from experience without being explicitly programmed. It focuses on developing algorithms that can access data and use it to learn.'),
(2, 'Deep Learning Fundamentals', 'Deep neural systems teach computers to learn by example using computational intelligence. Multi-layered networks can identify patterns in images, text, and speech with human-like accuracy in intelligent automation applications.'),
(3, 'Advanced Materials Processing', 'Materials processing involves the transformation of raw materials into finished products through various manufacturing techniques. Heat treatment, forging, and extrusion are critical processes that alter material properties to achieve desired strength and durability characteristics.'),
(4, 'Computer Vision and AI', 'Computer vision is an artificial intelligence field enabling machines to interpret visual information. Using machine learning algorithms, systems can identify objects, faces, and scenes in images and videos with remarkable accuracy.'),
(5, 'Adaptive Learning Systems', 'Adaptive learning represents a paradigm where intelligent agents learn optimal behaviors through trial and error interactions with their environment. This computational approach has revolutionized robotics, game playing, and autonomous systems development.'),
(6, 'Chemical Reaction Kinetics', 'Chemical reaction kinetics studies the rates of chemical processes and the factors affecting them. Temperature, pressure, and catalyst concentration influence reaction rates, enabling chemists to optimize industrial processes and predict reaction outcomes.'),
(7, 'AI Ethics and Responsible ML', 'Ethical considerations in artificial intelligence are crucial for responsible machine learning development. Bias mitigation, transparency, and fairness must guide AI systems to ensure equitable outcomes across diverse populations.'),
(8, 'Predictive Analytics with ML', 'Predictive analytics leverages machine learning algorithms to forecast future outcomes. By analyzing historical data, artificial intelligence models identify trends and patterns enabling data-driven decision making in businesses.'),
(9, 'Polymer Science and Engineering', 'Polymer science explores the synthesis, structure, and properties of macromolecules. Cross-linking density, molecular weight distribution, and crystallinity determine mechanical properties like tensile strength, elasticity, and thermal stability in polymer materials.'),
(10, 'AI in Healthcare Applications', 'Artificial intelligence transforms healthcare through machine learning applications in diagnosis, treatment planning, and drug discovery. AI systems analyze medical imaging and patient data to support clinical decision making.');
GO

-- Generate embeddings via Ollama for semantic similarity
-- Store and query embeddings alongside FTS results
UPDATE d
SET Embedding = AI_GENERATE_EMBEDDINGS(CONCAT(d.Title, ' ', d.Content) USE MODEL MyOllamaEmbeddingModel)
FROM Documents AS d;
GO

-- ⚠️ IMPORTANT: Vector Search and Indexing are PREVIEW FEATURES
-- Creating a vector index currently makes the table READ-ONLY
-- This limitation makes vector indexes suitable for ANALYTICS scenarios only at this time
-- Microsoft is actively working on removing this restriction in future updates
-- For production write-heavy workloads, consider generating embeddings without indexing
-- or wait for GA release with full read/write support

-- Create a vector index on the embedding column for faster similarity searches
CREATE VECTOR INDEX vec_idx ON Documents(Embedding)
WITH (METRIC = 'cosine', TYPE = 'diskann');
GO


-- Combine full-text search with semantic embeddings
-- Traditional keyword search via FTS
SELECT * FROM Documents
WHERE CONTAINS(Content, '"artificial intelligence" OR "machine learning"');

-- This starts faltering if specific contractions like AI and ML don't exist in the dataset. 
SELECT * FROM Documents
WHERE CONTAINS(Content, '"AI" OR "ML"');


-- Semantic vector search using embeddings
DECLARE @query_vector VECTOR(768) = AI_GENERATE_EMBEDDINGS(N'AI or Machine learning' USE MODEL MyOllamaEmbeddingModel);

SELECT TOP (5) 
    r.ID,
    r.Title,
    r.content,
    st.distance
FROM VECTOR_SEARCH(
        TABLE = [dbo].[Documents] AS t,
        COLUMN = [Embedding],
        SIMILAR_TO = @query_vector,
        METRIC = 'cosine',
        TOP_N = 5
    ) AS st
    INNER JOIN Documents r ON t.ID = r.ID
ORDER BY st.distance;

```

### 2. Document Processing Pipeline

**Note**: This use case uses the default configuration with Ollama enabled for local AI processing.

Common workflow:
- Store documents in SQL Server
- Generate embeddings via Ollama HTTPS API
- Perform similarity searches
- All within the same container

**For minimal footprint**: If using external embedding services (Azure OpenAI, etc.), build with `--install-ollama false` to reduce container size.

### 3. MinIO + SQL Server Polybase Integration

**Note**: This use case requires enabling both Polybase and MinIO during build with `--polybase true` and `--install-minio true` flags.

MinIO provides S3-compatible object storage that integrates seamlessly with SQL Server via Polybase, enabling SQL queries over unstructured data stored in MinIO.

**Prerequisites:**
```bash
# Build with Polybase and MinIO enabled
./build-and-run.sh --sa-password "YourStrong@Pass123" --polybase true --install-minio true
```

**Step 1: Create sample data files and upload to MinIO**

Use this Python program to generate sample CSV and Parquet files with technical book data:

```python
import pandas as pd
from minio import Minio
from minio.error import S3Error
import io

# Sample technical books data (10 rows)
books_data = {
    'ID': [1, 2, 3, 4, 5, 6, 7, 8, 9, 10],
    'Title': [
        'Introduction to Machine Learning',
        'Deep Learning Fundamentals',
        'Advanced Materials Processing',
        'Computer Vision and AI',
        'Adaptive Learning Systems',
        'Chemical Reaction Kinetics',
        'AI Ethics and Responsible ML',
        'Predictive Analytics with ML',
        'Polymer Science and Engineering',
        'AI in Healthcare Applications'
    ],
    'Content': [
        'Machine learning is a subset of artificial intelligence that enables systems to learn and improve from experience without being explicitly programmed. It focuses on developing algorithms that can access data and use it to learn.',
        'Deep neural systems teach computers to learn by example using computational intelligence. Multi-layered networks can identify patterns in images, text, and speech with human-like accuracy in intelligent automation applications.',
        'Materials processing involves the transformation of raw materials into finished products through various manufacturing techniques. Heat treatment, forging, and extrusion are critical processes that alter material properties to achieve desired strength and durability characteristics.',
        'Computer vision is an artificial intelligence field enabling machines to interpret visual information. Using machine learning algorithms, systems can identify objects, faces, and scenes in images and videos with remarkable accuracy.',
        'Adaptive learning represents a paradigm where intelligent agents learn optimal behaviors through trial and error interactions with their environment. This computational approach has revolutionized robotics, game playing, and autonomous systems development.',
        'Chemical reaction kinetics studies the rates of chemical processes and the factors affecting them. Temperature, pressure, and catalyst concentration influence reaction rates, enabling chemists to optimize industrial processes and predict reaction outcomes.',
        'Ethical considerations in artificial intelligence are crucial for responsible machine learning development. Bias mitigation, transparency, and fairness must guide AI systems to ensure equitable outcomes across diverse populations.',
        'Predictive analytics leverages machine learning algorithms to forecast future outcomes. By analyzing historical data, artificial intelligence models identify trends and patterns enabling data-driven decision making in businesses.',
        'Polymer science explores the synthesis, structure, and properties of macromolecules. Cross-linking density, molecular weight distribution, and crystallinity determine mechanical properties like tensile strength, elasticity, and thermal stability in polymer materials.',
        'Artificial intelligence transforms healthcare through machine learning applications in diagnosis, treatment planning, and drug discovery. AI systems analyze medical imaging and patient data to support clinical decision making.'
    ]
}

# Create DataFrame
df = pd.DataFrame(books_data)

# Create MinIO client
client = Minio(
    "localhost:9000",
    access_key="minioadmin",
    secret_key="minioadmin",
    secure=True,  # MinIO uses HTTPS with self-signed certificate
    cert_check=False  # Disable certificate verification for self-signed cert
)

# Create bucket if it doesn't exist
bucket_name = "sqltest"
try:
    if not client.bucket_exists(bucket_name):
        client.make_bucket(bucket_name)
        print(f"Bucket '{bucket_name}' created successfully")
    else:
        print(f"Bucket '{bucket_name}' already exists")
except S3Error as e:
    print(f"Error creating bucket: {e}")

# Save as CSV (with proper escaping for commas in content)
csv_buffer = io.BytesIO()
df.to_csv(csv_buffer, index=False, quoting=1)  # quoting=1 quotes all fields
csv_buffer.seek(0)

try:
    client.put_object(
        bucket_name,
        "technical_books.csv",
        csv_buffer,
        length=csv_buffer.getbuffer().nbytes,
        content_type="text/csv"
    )
    print("CSV file uploaded successfully to MinIO")
except S3Error as e:
    print(f"Error uploading CSV: {e}")

# Save as Parquet
parquet_buffer = io.BytesIO()
df.to_parquet(parquet_buffer, index=False, engine='pyarrow')
parquet_buffer.seek(0)

try:
    client.put_object(
        bucket_name,
        "technical_books.parquet",
        parquet_buffer,
        length=parquet_buffer.getbuffer().nbytes,
        content_type="application/octet-stream"
    )
    print("Parquet file uploaded successfully to MinIO")
except S3Error as e:
    print(f"Error uploading Parquet: {e}")

# List files in bucket to verify
print("\nFiles in bucket:")
objects = client.list_objects(bucket_name)
for obj in objects:
    print(f"  - {obj.object_name} ({obj.size} bytes)")
```

**Required Python packages:**
```bash
pip install pandas pyarrow minio
```

**Step 2: Configure SQL Server Polybase to query MinIO data**

**Important**: Polybase on SQL Server for Linux requires trace flag 13702. Configure it before enabling Polybase:

```bash
# Enable trace flag 13702 globally for Polybase support on Linux
docker exec -it sqlserver-ollama /opt/mssql/bin/mssql-conf traceflag 13702 on

# Restart the container for the trace flag to take effect
docker restart sqlserver-ollama

# Wait for SQL Server to start (check logs)
docker logs -f sqlserver-ollama
```

After the container restarts and SQL Server is ready, connect and run the following SQL commands:

```sql
-- Verify the Polybase feature is installed
SELECT SERVERPROPERTY('IsPolyBaseInstalled') AS IsPolyBaseInstalled;

-- Enable polybase feature using the commands below
EXEC sp_configure @configname = 'polybase enabled', @configvalue = 1;
RECONFIGURE WITH OVERRIDE;
EXEC sp_configure @configname = 'polybase enabled';
GO

-- Next, let's create a database and database scoped credential to access the object storage
CREATE DATABASE pb_demo;
GO

USE pb_demo;
GO

CREATE MASTER KEY ENCRYPTION BY PASSWORD = 'mypass123@';  
GO

-- Create database scoped credential with MinIO access credentials
-- Format: 'username:password' in the SECRET
CREATE DATABASE SCOPED CREDENTIAL s3_dc 
WITH IDENTITY = 'S3 Access Key', 
     SECRET = 'minioadmin:minioadmin';
GO

-- To verify the credential is created, run the below command
SELECT * FROM sys.database_scoped_credentials;
GO

-- Now create the External data source pointing to MinIO
-- Note: Use the container's internal IP or 'localhost' depending on network setup
-- If you get an error, try using the container's IP address instead of localhost
CREATE EXTERNAL DATA SOURCE s3_ds 
WITH (
    LOCATION = 's3://localhost:9000/',
    CREDENTIAL = s3_dc
);
GO

-- When creating the external data source, if you see the below error, 
-- restart the container and run the same command again
-- Msg 46530, Level 16, State 11, Line 20
-- External data sources are not supported with type GENERIC.

-- Query the CSV file from MinIO using OPENROWSET
-- This queries the technical_books.csv file we uploaded
SELECT * 
FROM OPENROWSET(
    BULK '/sqltest/technical_books.csv', 
    FORMAT = 'CSV',
    DATA_SOURCE = 's3_ds',
    FIRSTROW = 2  -- Skip header row
) WITH (
    ID INT,
    Title VARCHAR(200),
    Content VARCHAR(MAX)
) AS TechnicalBooks;
GO

-- Query the Parquet file from MinIO
-- Parquet format automatically handles schema
SELECT * 
FROM OPENROWSET(
    BULK '/sqltest/technical_books.parquet',
    FORMAT = 'PARQUET',
    DATA_SOURCE = 's3_ds'
) AS TechnicalBooksParquet;
GO

-- Example: Filter and analyze data from object storage
SELECT 
    ID,
    Title,
    LEFT(Content, 100) + '...' AS ContentPreview
FROM OPENROWSET(
    BULK '/sqltest/technical_books.parquet',
    FORMAT = 'PARQUET',
    DATA_SOURCE = 's3_ds'
) AS Books
WHERE Title LIKE '%Machine Learning%' OR Title LIKE '%AI%'
ORDER BY ID;
GO

-- Example: Join MinIO data with SQL Server tables
-- First, create a local table with additional metadata
CREATE TABLE BookMetadata (
    BookID INT PRIMARY KEY,
    ISBN VARCHAR(20),
    PublicationYear INT,
    Publisher VARCHAR(100)
);
GO

INSERT INTO BookMetadata VALUES
(1, '978-1-234-56789-0', 2023, 'Tech Publishers'),
(2, '978-1-234-56789-1', 2024, 'AI Press'),
(4, '978-1-234-56789-3', 2023, 'Vision Books'),
(7, '978-1-234-56789-6', 2024, 'Ethics Press');
GO

-- Join external MinIO data with internal SQL Server table
SELECT 
    b.ID,
    b.Title,
    m.ISBN,
    m.PublicationYear,
    m.Publisher
FROM OPENROWSET(
    BULK '/sqltest/technical_books.parquet',
    FORMAT = 'PARQUET',
    DATA_SOURCE = 's3_ds'
) AS b
INNER JOIN BookMetadata m ON b.ID = m.BookID
ORDER BY b.ID;
GO
```

**Benefits of MinIO + Polybase Integration:**
- **Unified Queries**: Query object storage and relational data together
- **Cost-Effective Storage**: Store large files in object storage, metadata in SQL Server
- **Data Lake Integration**: Build data lake solutions with SQL Server as the query engine
- **Hybrid Workloads**: Combine structured (SQL Server) and unstructured (MinIO) data
- **Format Flexibility**: Support for CSV, Parquet, and other data formats

### 4. Database Backup to S3-Compatible Object Storage (MinIO)

**Note**: This use case requires MinIO to be installed during build with `--install-minio true` flag.

SQL Server 2025 supports native backup to S3-compatible object storage using the BACKUP TO URL feature. This allows you to store database backups directly in MinIO.

**Prerequisites:**
```bash
# Build with MinIO enabled (Polybase optional but useful for data examples)
./build-and-run.sh --sa-password "YourPass@123" --install-minio true
```

- MinIO running with TLS enabled (automatically configured in this container)
- Database to backup (we'll use the `pb_demo` database from the Polybase example, or create a test database)

**Step 1: Create SQL Server credential for MinIO S3 backup**

```sql
-- Create credential for S3 backup to MinIO
-- Note: Use the format 'S3 Access Key' for IDENTITY and 'username:password' for SECRET
CREATE CREDENTIAL [s3://localhost:9000/sqlbackups]
WITH
    IDENTITY = 'S3 Access Key',
    SECRET = 'minioadmin:minioadmin';
GO

-- Verify the credential was created
SELECT * FROM sys.credentials WHERE name LIKE 's3://%';
GO
```

**Step 2: Create a bucket in MinIO for backups**

You can create the bucket via the MinIO Console (`https://localhost:9001`) or using Python:

```python
from minio import Minio

client = Minio(
    "localhost:9000",
    access_key="minioadmin",
    secret_key="minioadmin",
    secure=True,
    cert_check=False
)

# Create bucket for SQL Server backups
if not client.bucket_exists("sqlbackups"):
    client.make_bucket("sqlbackups")
    print("Bucket 'sqlbackups' created successfully")
```

**Step 3: Perform backup to MinIO**

```sql
-- Backup the pb_demo database to MinIO S3 storage
BACKUP DATABASE pb_demo
TO URL = 's3://localhost:9000/sqlbackups/pb_demo_full.bak'
WITH 
    STATS = 10,
    COMPRESSION,
    CHECKSUM;
GO

-- Example: Differential backup
BACKUP DATABASE pb_demo
TO URL = 's3://localhost:9000/sqlbackups/pb_demo_diff.bak'
WITH 
    DIFFERENTIAL,
    STATS = 10,
    COMPRESSION;
GO

-- Example: Transaction log backup
BACKUP LOG pb_demo
TO URL = 's3://localhost:9000/sqlbackups/pb_demo_log.trn'
WITH 
    STATS = 10,
    COMPRESSION;
GO
```

**Step 4: Verify backup in MinIO**

Using MinIO Console:
1. Navigate to `https://localhost:9001`
2. Login with `minioadmin/minioadmin`
3. Browse to the `sqlbackups` bucket
4. You should see your backup files: `pb_demo_full.bak`, `pb_demo_diff.bak`, `pb_demo_log.trn`

Using Python:
```python
from minio import Minio

client = Minio(
    "localhost:9000",
    access_key="minioadmin",
    secret_key="minioadmin",
    secure=True,
    cert_check=False
)

# List all backup files
print("Backup files in MinIO:")
objects = client.list_objects("sqlbackups")
for obj in objects:
    print(f"  - {obj.object_name} ({obj.size} bytes, {obj.last_modified})")
```

**Step 5: Restore database from S3 backup**

```sql
-- Restore database from MinIO S3 storage
RESTORE DATABASE pb_demo_restored
FROM URL = 's3://localhost:9000/sqlbackups/pb_demo_full.bak'
WITH 
    MOVE 'pb_demo' TO '/var/opt/mssql/data/pb_demo_restored.mdf',
    MOVE 'pb_demo_log' TO '/var/opt/mssql/data/pb_demo_restored_log.ldf',
    STATS = 10,
    REPLACE;
GO

-- Verify the restored database
SELECT name, state_desc, recovery_model_desc
FROM sys.databases
WHERE name = 'pb_demo_restored';
GO
```

**Benefits of Backup to MinIO:**
- **Cost-Effective**: Store backups in object storage instead of expensive disk arrays
- **Scalable**: MinIO can scale to petabytes of storage
- **Offsite Storage**: Separate backup storage from production databases
- **S3 Compatible**: Can migrate backups to AWS S3, Azure Blob, or other S3-compatible storage
- **Automated Retention**: Use MinIO lifecycle policies for automatic backup retention
- **Compression**: SQL Server compression reduces backup size and transfer time

**Troubleshooting Backup to URL:**

If backup fails with certificate or connectivity errors:

1. Verify MinIO TLS certificate is trusted:
   ```bash
   docker exec sqlserver-ollama ls -la /var/opt/mssql/security/ca-certificates/minio-cert.crt
   ```

2. Test MinIO connectivity:
   ```bash
   docker exec sqlserver-ollama curl -k https://localhost:9000/minio/health/live
   ```

3. Verify credential is correct:
   ```sql
   SELECT * FROM sys.credentials WHERE name = 's3://localhost:9000/sqlbackups';
   ```

4. Check SQL Server error log for details:
   ```sql
   EXEC xp_readerrorlog 0, 1, N'URL';
   ```

### 5. Hybrid AI/SQL Workloads

Build the appropriate configuration based on your needs:
- **AI workloads**: Default configuration includes Ollama (local AI inference)
- **Data platform**: Add `--polybase true` for external data connectivity
- **Object storage**: Add `--install-minio true` for S3-compatible storage
- **Minimal SQL**: Use `--install-ollama false` to exclude AI components

Traditional OLTP operations in SQL Server combined with:
- Real-time AI inference via Ollama (when installed)
- Object storage for model artifacts and data files via MinIO (when installed)
- Secure communication between components
- Single deployment unit with flexible configuration

### 6. Development and Testing

Choose your stack based on testing requirements:
- **Minimal**: `--install-ollama false` for pure SQL testing (~3.5GB)
- **AI-enabled**: Default with Ollama for AI/ML testing (~5.5GB)
- **Full stack**: Add `--polybase true --install-minio true` for complete platform testing (~5.8GB)

Benefits:
- No external dependencies
- Reproducible environments
- Quick setup for proof-of-concepts
- Deploy and destroy in minutes

## Building and Deploying

### Quick Start

**With Ubuntu base (default):**
```bash
# Minimal: SQL Server + FTS only (no Ollama, smallest image)
docker build --build-arg INSTALL_OLLAMA=false -t sqlserver-minimal:2025 .

# Default: SQL Server + FTS + Ollama (AI-enabled)
docker build -t sqlserver-ollama:2025 .

# With Polybase for external data access
docker build --build-arg ENABLE_POLYBASE=true -t sqlserver-ollama:2025 .

# Full stack: All components
docker build --build-arg ENABLE_POLYBASE=true --build-arg INSTALL_MINIO=true -t sqlserver-full:2025 .

# Or use the build script (SA password is MANDATORY)
# Default configuration (FTS + Ollama)
./build-and-run.sh --sa-password "YourStrong@Passw0rd"

# Minimal (no Ollama)
./build-and-run.sh --sa-password "YourStrong@Passw0rd" --install-ollama false

# With Polybase and MinIO
./build-and-run.sh --sa-password "YourStrong@Passw0rd" --polybase true --install-minio true
```

**Note**: The `build-and-run.sh` script requires a SQL Server SA password for security compliance. All components (Ollama, MinIO, Polybase) are optional and controlled via build parameters.

**With RHEL base:**

```bash
# Default RHEL build (FTS + Ollama)
./build-and-run.sh --sa-password "YourStrong@Passw0rd" --base-image mcr.microsoft.com/mssql/rhel/server:2025-latest

# Full stack RHEL (all components)
./build-and-run.sh --sa-password "YourStrong@Passw0rd" \
  --base-image mcr.microsoft.com/mssql/rhel/server:2025-latest \
  --polybase true --install-minio true
```

**Verified Image Paths:**
- Ubuntu: `mcr.microsoft.com/mssql/server:2025-latest`
- RHEL: `mcr.microsoft.com/mssql/rhel/server:2025-latest`

**Run the container:**

The `build-and-run.sh` script automatically handles port mappings and volumes based on installed components. For manual docker run:

```bash
# Full stack example (adjust ports/volumes based on your build configuration)
docker run -d \
  --name sqlserver-ollama \
  --memory="8g" \
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

### Testing the Setup

```bash
# Test SQL Server
sqlcmd -S localhost -U sa -P 'YourStrong@Passw0rd' -Q "SELECT @@VERSION"

# Test Ollama via HTTPS
curl -k https://localhost:11435/api/embeddings -d '{
  "model": "nomic-embed-text",
  "prompt": "Hello, world!"
}'

# Test MinIO API (now uses HTTPS)
curl -k https://localhost:9000/minio/health/live

# Access MinIO Console
# Open browser to https://localhost:9001
# Login with minioadmin/minioadmin
```

### Sharing Your Container

```bash
# Tag for Docker Hub
docker tag sqlserver-ollama:2025 yourusername/sqlserver-ollama:2025

# Push to registry
docker push yourusername/sqlserver-ollama:2025

# Others can now pull and run
docker run -d yourusername/sqlserver-ollama:2025
```

## Performance Considerations

### Resource Requirements
- **Minimum**: 4GB RAM, 20GB disk
- **Recommended**: 8GB RAM, 50GB disk
- **Production**: 16GB+ RAM, SSD storage

### Optimization Tips
1. **Pre-pull models during build** - Reduces startup time from minutes to seconds
2. **Use volume mounts** - Persist data and avoid re-downloading models
3. **Set memory limits** - Prevent container from consuming all host resources
4. **Enable resource quotas** - Use Docker's memory reservation feature

### Scaling Considerations
For production workloads:
- Consider separating services into individual containers for horizontal scaling
- Use container orchestration (Kubernetes, Docker Swarm)
- Implement external load balancers
- Use managed SQL Server and AI services for enterprise scale

## Security Best Practices

### ✅ What We Did Right
1. **User Isolation**: SQL Server runs as `mssql` user, not root
2. **HTTPS by Default**: All AI API calls use encrypted channels
3. **Certificate Management**: Automatic generation and rotation
4. **Principle of Least Privilege**: Each service has minimal necessary permissions

### 🔐 Production Hardening
For production deployments, consider:

```bash
# Use Docker secrets for passwords
docker secret create sa_password ./password.txt

# Run with read-only root filesystem where possible
docker run --read-only ...

# Limit capabilities
docker run --cap-drop=ALL --cap-add=NET_BIND_SERVICE ...

# Use security scanning
docker scan sqlserver-ollama:2025
```

### 🛡️ Network Security
```bash
# Don't expose all ports in production
# Use internal networks
docker network create --internal ai-network

# Only expose necessary ports
-p 1433:1433  # Only if external SQL access needed
```

## Extending the Solution

### Adding More AI Models

Modify the Dockerfile to include additional models:

```dockerfile
RUN cat >> /opt/start-services.sh <<'EOF'
ollama pull llama2
ollama pull codellama
ollama pull mistral
EOF
```

### Integrating with Applications

**Python Example: Working with the Documents Table**

**Prerequisites:** This example assumes you have already executed the T-SQL code from the "Advanced Text Search with FTS + AI Embeddings" section above, which creates the `AIDocuments` database, `Documents` table, `MyOllamaEmbeddingModel` external model, and populates the initial 10 sample documents with embeddings.

First, install the required dependencies:

```bash
# Install Python 3 (if not already installed)
# Ubuntu/Debian
sudo apt-get update && sudo apt-get install -y python3 python3-pip

# RHEL/CentOS
sudo yum install -y python3 python3-pip

# Install pymssql (Python SQL Server driver) and requests
pip3 install pymssql requests
```

**Python Code:**
```python
import pymssql
import requests
import json

# Connect to SQL Server using pymssql
# IMPORTANT: Update connection parameters for your environment
# - Change server, user, password, database as needed
# - pymssql doesn't validate certificates by default (suitable for self-signed certs)
conn = pymssql.connect(
    server='localhost',
    user='sa',
    password='YourStrong@Passw0rd',
    database='AIDocuments',
    port=1433
)

cursor = conn.cursor()

# Drop vector index if it exists (required for write operations)
cursor.execute("""
    IF EXISTS (SELECT * FROM sys.indexes WHERE name = 'vec_idx' AND object_id = OBJECT_ID('Documents'))
    DROP INDEX vec_idx ON Documents;
""")
conn.commit()
print("Vector index dropped (if it existed)")

# METHOD 1: Python generates embedding via Ollama, then inserts all data
# New document about quantum computing
title1 = 'Quantum Computing Fundamentals'
content1 = 'Quantum computing leverages quantum mechanical phenomena like superposition and entanglement to perform computations. Quantum bits or qubits can exist in multiple states simultaneously, enabling parallel processing capabilities that surpass classical computers for specific problem domains.'

# Generate embedding via Ollama HTTPS endpoint
response = requests.post(
    'https://localhost:11435/api/embed',
    json={
        'model': 'nomic-embed-text',
        'input': f'{title1} {content1}'
    },
    verify=False  # Self-signed certificate; use verify='/path/to/ca-cert.pem' in production
)

if response.status_code == 200:
    embedding_data = response.json()
    embedding1 = json.dumps(embedding_data['embeddings'][0])  # Convert list to JSON string
    
    # Insert with Python-generated embedding
    cursor.execute("""
        INSERT INTO Documents (ID, Title, Content, Embedding)
        VALUES (%s, %s, %s, CAST(%s AS VECTOR(768)))
    """, (11, title1, content1, embedding1))
    conn.commit()
    print(f"Row 1 inserted: {title1} (Python-generated embedding)")
else:
    print(f"Failed to generate embedding: {response.status_code}")

# METHOD 2: Direct SQL INSERT with AI_GENERATE_EMBEDDINGS function
# New document about blockchain technology
title2 = 'Blockchain and Distributed Ledgers'
content2 = 'Blockchain technology provides a decentralized, immutable ledger for recording transactions across a distributed network. Cryptographic hashing and consensus mechanisms ensure data integrity and security without requiring a central authority, enabling trustless peer-to-peer transactions.'

cursor.execute("""
    INSERT INTO Documents (ID, Title, Content, Embedding)
    VALUES (
        %s, 
        %s, 
        %s,
        AI_GENERATE_EMBEDDINGS(CONCAT(%s, ' ', %s) USE MODEL MyOllamaEmbeddingModel)
    )
""", (12, title2, content2, title2, content2))
conn.commit()
print(f"Row 2 inserted: {title2} (SQL-generated embedding)")

# Query the newly inserted documents
cursor.execute("SELECT ID, Title FROM Documents WHERE ID IN (11, 12)")
rows = cursor.fetchall()
print("\nNewly inserted documents:")
for row in rows:
    print(f"  ID: {row[0]}, Title: {row[1]}")

# Create vector index for faster similarity searches (makes table read-only)
# NOTE: CREATE VECTOR INDEX cannot run inside a transaction
print("\nCreating vector index...")
conn.autocommit(True)  # Enable autocommit mode for vector index creation
cursor.execute("""
    CREATE VECTOR INDEX vec_idx ON Documents(Embedding)
    WITH (METRIC = 'cosine', TYPE = 'diskann');
""")
conn.autocommit(False)  # Restore default transaction mode
print("Vector index created successfully")

# Perform semantic search for "immutable or verifiable transactions"
# Using SQL Server's AI_GENERATE_EMBEDDINGS function directly in the query
search_query = "immutable or verifiable transactions"
print(f"\nPerforming semantic search for: '{search_query}'")

# Execute vector search with embedding generation done by SQL Server
cursor.execute("""
    DECLARE @query_vector VECTOR(768) = AI_GENERATE_EMBEDDINGS(N'immutable or verifiable transactions' USE MODEL MyOllamaEmbeddingModel);
    
    SELECT TOP (5) 
        r.ID,
        r.Title,
        r.Content,
        st.distance
    FROM VECTOR_SEARCH(
            TABLE = [dbo].[Documents] AS t,
            COLUMN = [Embedding],
            SIMILAR_TO = @query_vector,
            METRIC = 'cosine',
            TOP_N = 5
        ) AS st
        INNER JOIN Documents r ON t.ID = r.ID
    ORDER BY st.distance;
""")

search_results = cursor.fetchall()
print("\nTop 5 semantically similar documents:")
print("-" * 80)
for idx, row in enumerate(search_results, 1):
    doc_id, title, content, distance = row
    print(f"{idx}. ID: {doc_id} | Distance: {distance:.4f}")
    print(f"   Title: {title}")
    print(f"   Content: {content[:100]}...")
    print()

# Clean up
cursor.close()
conn.close()
print("Connection closed successfully")
```

**Key Points:**
- **Method 1** demonstrates calling Ollama API from Python, giving you full control over the embedding process
- **Method 2** leverages SQL Server's `AI_GENERATE_EMBEDDINGS` function, offloading embedding generation to the database
- Vector index must be dropped before write operations (preview limitation)
- Use `pymssql` for native Python SQL Server connectivity without ODBC dependencies
- Self-signed certificates require `verify=False` or providing the CA certificate path

### Custom Caddy Configuration

Need specific TLS settings or additional routes? Modify the Caddyfile:

```dockerfile
RUN cat > /etc/caddy/Caddyfile << 'EOF'
{
    local_certs
}

https://localhost:11435 {
    tls internal
    reverse_proxy localhost:11434
    
    # Add custom headers
    header {
        X-Custom-Header "value"
        -Server
    }
    
    # Add rate limiting
    rate_limit {
        zone dynamic_zone {
            key {remote_host}
            events 100
            window 1m
        }
    }
}
EOF
```

## Lessons Learned

### Challenges We Overcame

**1. Permission Issues**
Initially, Ollama failed to start because the container switched to the `mssql` user too early. Solution: Keep the orchestration script running as root, but explicitly switch to `mssql` only for the SQL Server process.

**2. Certificate Trust**
Getting SQL Server to trust Caddy's self-signed certificates required copying the CA root to multiple locations and running `update-ca-certificates`.

**3. Caddy Configuration**
The TLS configuration required using `local_certs` with `tls internal` and the full `https://` URL format rather than just a port number.

## Benchmarks and Results

In our testing, this integrated container provides:

- **Build Time**: 
  - First build (cold cache): Up to 10 minutes when base container image and Ollama models need to be downloaded
  - With Polybase: Add 1-2 minutes for additional package installation
  - Rebuild: 2-3 minutes with cached base image
  - Cached build: Very quick (<1 minute) when base image and models are already pulled
- **Startup Time**: 45-75 seconds (with pre-pulled model and MinIO initialization)
- **Embedding Generation**: ~50ms per request (nomic-embed-text)
- **SQL Query Performance**: Native SQL Server performance
- **MinIO Throughput**: Depends on disk I/O; typically 100+ MB/s for local volumes
- **Memory Footprint**: ~6-7GB with one model loaded and MinIO running
- **Disk Usage**: ~15-16GB with SQL Server + Ollama + MinIO + one model

**Note**: The first-time build can be lengthy due to downloading the SQL Server 2025 base image (~1.5GB), MinIO binary, and pulling the Ollama `nomic-embed-text` model. Subsequent builds benefit from Docker's layer caching and pre-pulled models.

## Future Enhancements

### Roadmap Ideas
1. **Health Checks**: Add proper Docker health checks for each service
2. **Graceful Shutdown**: Implement signal handlers for clean container stops
3. **Metrics**: Add Prometheus exporters for monitoring
4. **Logging**: Centralize logs to a volume mount for easier debugging
5. **Multiple Ollama Instances**: Load balancing for parallel inference
6. **SQL Server AI Integration**: Direct T-SQL functions calling Ollama
7. **Vector Index Support**: Native vector similarity search in SQL Server
8. **Monitoring Dashboard**: Web UI showing service status and metrics
9. **Auto-scaling**: Dynamic model loading based on demand
10. **MinIO Replication**: Multi-node MinIO setup for high availability
11. **Advanced Polybase**: Additional connectors for various data sources

### A Learning Sample, Not a Production Solution
**Important Note**: This is a sample solution designed for learning and experimentation purposes. It demonstrates integration patterns and architectural concepts but is **not production-ready**. Use this as a starting point to understand the technologies and build a more robust, enterprise-grade solution tailored to your specific requirements.

While I appreciate ideas and feedback from the community, please note that I may or may not incorporate suggestions into this sample. Some interesting areas for exploration could include:
- Additional AI model configurations
- Performance tuning strategies
- Integration patterns with different frameworks
- Containerization best practices
- Azure deployment examples
- MinIO bucket policies and access control
- Polybase performance optimization

Feel free to fork this solution and adapt it for your needs!

## Conclusion

Building this all-in-one SQL Server + AI + Object Storage container has been an exciting journey into the intersection of traditional databases, modern AI capabilities, and cloud-native storage. By combining SQL Server 2025, Ollama, MinIO, and Caddy into a single container, we've created a solution that is:

- **Secure**: HTTPS everywhere, proper user isolation, trusted certificates
- **Simple**: One container, one command, ready to go
- **Powerful**: Full SQL Server + AI embeddings + S3-compatible storage + secure proxy
- **Flexible**: Easy to extend with more models, Polybase connectors, and configurations
- **Complete**: Structured data (SQL), unstructured data (MinIO), and AI (Ollama) in one place

Whether you're building a semantic search engine, processing documents with AI, storing model artifacts in object storage, or experimenting with hybrid SQL/AI/Storage workloads, this container provides a solid foundation.

## Get Started Today

All the code, scripts, and documentation are available:

📦 **Dockerfile**: Complete container definition  
🚀 **Build Script**: Automated Bash script for building and running  
📖 **README**: Comprehensive setup and usage guide  
🔧 **Examples**: Sample queries and API calls    

### Resources

- **SQL Server 2025 Documentation**: https://docs.microsoft.com/sql/
- **Ollama GitHub**: https://github.com/ollama/ollama
- **Caddy Documentation**: https://caddyserver.com/docs/
- **Docker Best Practices**: https://docs.docker.com/develop/dev-best-practices/

### Try It Yourself

**Option 1: Automated (Recommended)**
```bash
# Clone or download the files (Dockerfile, build-and-run.sh, etc.)
# The script will build the image AND run the container
./build-and-run.sh --sa-password "YourSecure@Pass123"

# With Polybase for MinIO integration
./build-and-run.sh --sa-password "YourSecure@Pass123" --polybase true

# Start building AI-powered database applications with object storage!
```

**Option 2: Manual**
```bash
# Build the image manually
docker build -t sqlserver-ollama:2025 .

# Run the container manually
docker run -d \
  --name sqlserver-ollama \
  --memory="8g" \
  -e ACCEPT_EULA=Y \
  -e MSSQL_SA_PASSWORD=YourSecure@Pass123 \
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

---

## About the Author

**Tejas Shah**  
*Principal Group Program Manager, Microsoft*

## Questions or Feedback?

Have you built something similar? Found a better approach? I'd love to hear from you! Drop a comment below or reach out on [your preferred platform].

## Tags

`#SQLServer` `#AI` `#Docker` `#Ollama` `#Embeddings` `#MachineLearning` `#DevOps` `#Containers` `#HTTPS` `#Caddy` `#MinIO` `#ObjectStorage` `#S3` `#Polybase` `#DatabaseEngineering` `#InfrastructureAsCode`

---

*Last updated: January 13, 2026*
