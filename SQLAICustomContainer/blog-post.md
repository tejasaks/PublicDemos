# Building an AI-Powered SQL Server Container: Combining SQL Server 2025, Ollama, and Caddy for Secure Embeddings

## Introduction

In the rapidly evolving landscape of AI and data management, the need to combine traditional database systems with modern AI capabilities has never been more critical. Today, I'm excited to share an example of a solution that bridges this gap: a custom Docker container that seamlessly integrates SQL Server 2025, Ollama (an AI model runtime), and Caddy (a modern HTTPS proxy server) into a single, deployable unit.

This solution enables you to run AI embeddings and large language models directly alongside your SQL Server database, with secure HTTPS communication, Full-Text Search capabilities, and proper certificate management—all within one container. Plus, it intelligently adapts to both Ubuntu and RHEL base images automatically.

## The Challenge

Modern applications increasingly require:
- **Relational database capabilities** for structured data storage
- **AI/ML model inference** for embeddings, semantic search, and natural language processing
- **Secure communication** between services, especially in production environments
- **Simple deployment** that doesn't require complex orchestration

Traditionally, these would require multiple containers, complex networking configurations, and significant DevOps overhead. What if we could package everything into a single, reusable container image?

## The Solution Architecture

Our custom container combines three powerful components:

```
┌─────────────────────────────────────────────────────┐
│                 Docker Container                     │
│              (Ubuntu or RHEL Base)                   │
├─────────────────────────────────────────────────────┤
│                                                       │
│  ┌──────────────┐         ┌────────────────┐       │
│  │ SQL Server   │         │ Ollama Runtime │       │
│  │ 2025 + FTS   │         │ + nomic-embed  │       │
│  │ Port: 1433   │         │ Port: 11434    │       │
│  └──────────────┘         └────────┬───────┘       │
│         │                           │               │
│         │                           │               │
│         │                  ┌────────▼───────┐      │
│         │                  │ Caddy Proxy    │      │
│         │                  │ HTTPS:11435    │      │
│         │                  └────────┬───────┘      │
│         │                           │               │
│    Trusted CA ◄────────────────────┘               │
│    Certificates                                     │
│                                                     │
│  OS Detection: Ubuntu/Debian or RHEL/CentOS       │
└─────────────────────────────────────────────────────┘
          │                           │
          │                           │
     Port 1433                   Port 11435
    (SQL Server)               (Ollama HTTPS)
```

### Component Breakdown

**1. SQL Server 2025 (Ubuntu or RHEL Base)**
- Latest SQL Server running on Ubuntu or RHEL (configurable at build time)
- Full-Text Search (FTS) enabled for advanced text indexing and searching
- Runs as the `mssql` user for security
- Full T-SQL capabilities with AI integration potential
- Trusts Caddy's CA certificates for secure outbound connections

**2. Ollama AI Runtime**
- Runs the `nomic-embed-text` model (pre-pulled during build)
- Provides REST API for embeddings generation
- Can be extended with additional models (llama2, codellama, etc.)
- HTTP endpoint on port 11434

**3. Caddy HTTPS Proxy**
- Automatically generates self-signed certificates
- Reverse proxies Ollama HTTP → HTTPS
- Certificate authority chain trusted system-wide
- Production-ready TLS configuration

## Key Features

### 🔒 Security First
- SQL Server runs as unprivileged `mssql` user
- HTTPS encryption for AI model API calls
- Automatic certificate management
- System-wide CA trust store integration

### 🎯 Intelligent OS Detection
- Automatically detects Ubuntu/Debian or RHEL/CentOS base
- Uses appropriate package managers (apt-get vs yum/dnf)
- Single Dockerfile works with multiple base images
- Proper repository configuration for each OS

### 🚀 Performance Optimized
- AI model pre-pulled during image build
- Single container reduces network latency
- Direct in-memory communication between services
- Efficient resource utilization

### 📦 Production Ready
- Persistent volumes for data, models, and certificates
- Configurable memory limits
- Comprehensive logging
- Health check compatible

### 🔧 Developer Friendly
- Single `docker run` command deployment
- Automated startup orchestration
- Clear documentation and examples
- Easy to extend with additional models

## Implementation Highlights

### Intelligent OS Detection

One of the most powerful features of this solution is its ability to work with both Ubuntu and RHEL base images without any code changes. The Dockerfile intelligently detects the operating system at build time:

```dockerfile
RUN if [ -f /etc/debian_version ]; then \
        # Ubuntu/Debian detected
        apt-get update && \
        apt-get install -y mssql-server-fts caddy && \
        # Add Ubuntu repositories
        wget -qO- https://packages.microsoft.com/.../ubuntu/22.04/mssql-server-2025.list
    elif [ -f /etc/redhat-release ]; then \
        # RHEL detected
        yum install -y mssql-server-fts caddy && \
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

One of the most critical aspects of this solution is the startup script that orchestrates all three services in the correct order:

```bash
1. Setup directory permissions
2. Start Ollama service
3. Wait for Ollama readiness
4. Pull AI models
5. Start Caddy with HTTPS
6. Copy Caddy certificates to SQL Server trust store
7. Update system CA certificates
8. Start SQL Server as mssql user
```

This sequence ensures that:
- Services start in dependency order
- Certificates are generated before SQL Server starts
- SQL Server trusts the Caddy CA from the beginning
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

## Real-World Use Cases

### 1. Advanced Text Search with FTS + AI Embeddings
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
- Store documents in SQL Server
- Generate embeddings via Ollama HTTPS API
- Perform similarity searches
- All within the same container

### 3. Hybrid AI/SQL Workloads
- Traditional OLTP operations in SQL Server
- Real-time AI inference via Ollama
- Secure communication between components
- Single deployment unit

### 4. Development and Testing
- Complete AI-powered database stack
- No external dependencies
- Reproducible environments
- Quick setup for proof-of-concepts

## Building and Deploying

### Quick Start

**With Ubuntu base (default):**
```bash
# Build the image
docker build -t sqlserver-ollama:2025 .

# Or use the build script (SA password is MANDATORY)
# Option 1: Pass password as parameter
./build-and-run.sh --sa-password "YourStrong@Passw0rd"

# Option 2: Edit build-and-run.sh and set SA_PASSWORD variable to your desired password
# Then run: ./build-and-run.sh
```

**Note**: The `build-and-run.sh` script requires a SQL Server SA password for security compliance. You must either pass it via `--sa-password` parameter or modify the script to set the `SA_PASSWORD` variable directly.

**With RHEL base:**
```bash
# Build with RHEL base image
docker build --build-arg BASE_IMAGE=mcr.microsoft.com/mssql/rhel/server:2025-latest -t sqlserver-ollama:2025-rhel .

# Or use the build script with password (MANDATORY)
./build-and-run.sh --sa-password "YourStrong@Passw0rd" --base-image mcr.microsoft.com/mssql/rhel/server:2025-latest
```

**Run the container:**
```bash
docker run -d \
  --name sqlserver-ollama \
  --memory="8g" \
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

### Testing the Setup

```bash
# Test SQL Server
sqlcmd -S localhost -U sa -P 'YourStrong@Passw0rd' -Q "SELECT @@VERSION"

# Test Ollama via HTTPS
curl -k https://localhost:11435/api/embeddings -d '{
  "model": "nomic-embed-text",
  "prompt": "Hello, world!"
}'
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
  - Rebuild: 2-3 minutes with cached base image
  - Cached build: Very quick (<1 minute) when base image and models are already pulled
- **Startup Time**: 30-60 seconds (with pre-pulled model)
- **Embedding Generation**: ~50ms per request (nomic-embed-text)
- **SQL Query Performance**: Native SQL Server performance
- **Memory Footprint**: ~6GB with one model loaded
- **Disk Usage**: ~15GB with SQL Server + Ollama + one model

**Note**: The first-time build can be lengthy due to downloading the SQL Server 2025 base image (~1.5GB) and pulling the Ollama `nomic-embed-text` model. Subsequent builds benefit from Docker's layer caching and pre-pulled models.

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

### A Learning Sample, Not a Production Solution
**Important Note**: This is a sample solution designed for learning and experimentation purposes. It demonstrates integration patterns and architectural concepts but is **not production-ready**. Use this as a starting point to understand the technologies and build a more robust, enterprise-grade solution tailored to your specific requirements.

While I appreciate ideas and feedback from the community, please note that I may or may not incorporate suggestions into this sample. Some interesting areas for exploration could include:
- Additional AI model configurations
- Performance tuning strategies
- Integration patterns with different frameworks
- Containerization best practices
- Azure deployment examples

Feel free to fork this solution and adapt it for your needs!

## Conclusion

Building this all-in-one SQL Server + AI container has been an exciting journey into the intersection of traditional databases and modern AI capabilities. By combining SQL Server 2025, Ollama, and Caddy into a single container, we've created a solution that is:

- **Secure**: HTTPS everywhere, proper user isolation, trusted certificates
- **Simple**: One container, one command, ready to go
- **Powerful**: Full SQL Server + AI embeddings + secure proxy
- **Flexible**: Easy to extend with more models and configurations

Whether you're building a semantic search engine, processing documents with AI, or experimenting with hybrid SQL/AI workloads, this container provides a solid foundation.

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

# Start building AI-powered database applications!
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
  -v sqlserver_data:/var/opt/mssql \
  -v ollama_data:/root/.ollama \
  -v caddy_data:/root/.local/share/caddy \
  sqlserver-ollama:2025
```

---

## About the Author

**Tejas Shah**  
*Principal Group Program Manager, Microsoft*

## Questions or Feedback?

Have you built something similar? Found a better approach? I'd love to hear from you! Drop a comment below or reach out on [your preferred platform].

## Tags

`#SQLServer` `#AI` `#Docker` `#Ollama` `#Embeddings` `#MachineLearning` `#DevOps` `#Containers` `#HTTPS` `#Caddy` `#DatabaseEngineering` `#InfrastructureAsCode`

---

*Last updated: January 13, 2026*
