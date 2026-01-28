-- ============================================================================
-- SQL Server Availability Group Setup Script
-- For use with MSSQL Kubernetes Operator
-- ============================================================================
-- 
-- Prerequisites:
--   1. SQL Server pods are running (kubectl get pods -n mssql)
--   2. SA password secret is created
--   3. HADR is enabled on all instances (hadrEnabled: true in spec)
--
-- Usage:
--   # Connect to primary pod and run this script
--   kubectl exec -it sql-ag-prod01-0 -n mssql -- /opt/mssql-tools18/bin/sqlcmd \
--     -S localhost -U sa -P 'YourStrong@Passw0rd!' -C -i /scripts/setup-ag.sql
--
-- ============================================================================

-- Configuration variables (update these for your environment)
DECLARE @AGName NVARCHAR(100) = 'ProductionAG';
DECLARE @PodBaseName NVARCHAR(100) = 'sql-ag-prod01';
DECLARE @Namespace NVARCHAR(100) = 'mssql';
DECLARE @ReplicaCount INT = 3;
DECLARE @EndpointPort INT = 5022;

PRINT 'Starting Availability Group Setup...';
PRINT 'AG Name: ' + @AGName;
PRINT 'Pod Base Name: ' + @PodBaseName;
GO

-- ============================================================================
-- STEP 1: Enable HADR (if not already enabled via mssql.conf)
-- ============================================================================
PRINT '';
PRINT '=== Step 1: Checking HADR Configuration ===';

-- Check if HADR is enabled
IF (SELECT SERVERPROPERTY('IsHadrEnabled')) = 0
BEGIN
    PRINT 'WARNING: HADR is not enabled. Enable it via mssql.conf or operator spec.';
    PRINT 'Set hadrEnabled: true in the SQLServer spec and restart pods.';
END
ELSE
BEGIN
    PRINT 'HADR is enabled.';
END
GO

-- ============================================================================
-- STEP 2: Create Master Key and Certificate for Endpoint Authentication
-- ============================================================================
PRINT '';
PRINT '=== Step 2: Creating Master Key and Certificate ===';

-- Create master key if not exists
IF NOT EXISTS (SELECT * FROM sys.symmetric_keys WHERE name = '##MS_DatabaseMasterKey##')
BEGIN
    CREATE MASTER KEY ENCRYPTION BY PASSWORD = 'MasterKeyP@ssw0rd!';
    PRINT 'Master key created.';
END
ELSE
BEGIN
    PRINT 'Master key already exists.';
END
GO

-- Create certificate for endpoint authentication
IF NOT EXISTS (SELECT * FROM sys.certificates WHERE name = 'AG_Auth_Cert')
BEGIN
    CREATE CERTIFICATE AG_Auth_Cert
        WITH SUBJECT = 'AG Authentication Certificate',
        EXPIRY_DATE = '2030-12-31';
    PRINT 'Certificate AG_Auth_Cert created.';
END
ELSE
BEGIN
    PRINT 'Certificate AG_Auth_Cert already exists.';
END
GO

-- ============================================================================
-- STEP 3: Create Database Mirroring Endpoint
-- ============================================================================
PRINT '';
PRINT '=== Step 3: Creating Database Mirroring Endpoint ===';

IF NOT EXISTS (SELECT * FROM sys.endpoints WHERE name = 'AG_Endpoint')
BEGIN
    CREATE ENDPOINT AG_Endpoint
        STATE = STARTED
        AS TCP (LISTENER_PORT = 5022)
        FOR DATABASE_MIRRORING (
            ROLE = ALL,
            AUTHENTICATION = CERTIFICATE AG_Auth_Cert,
            ENCRYPTION = REQUIRED ALGORITHM AES
        );
    PRINT 'Endpoint AG_Endpoint created on port 5022.';
END
ELSE
BEGIN
    PRINT 'Endpoint AG_Endpoint already exists.';
END
GO

-- ============================================================================
-- STEP 4: Create Sample Databases
-- ============================================================================
PRINT '';
PRINT '=== Step 4: Creating Sample Databases ===';

-- Create ApplicationDB
IF NOT EXISTS (SELECT * FROM sys.databases WHERE name = 'ApplicationDB')
BEGIN
    CREATE DATABASE ApplicationDB;
    PRINT 'Database ApplicationDB created.';
END
ELSE
BEGIN
    PRINT 'Database ApplicationDB already exists.';
END
GO

-- Create ReportingDB
IF NOT EXISTS (SELECT * FROM sys.databases WHERE name = 'ReportingDB')
BEGIN
    CREATE DATABASE ReportingDB;
    PRINT 'Database ReportingDB created.';
END
ELSE
BEGIN
    PRINT 'Database ReportingDB already exists.';
END
GO

-- Set FULL recovery model (required for AG)
ALTER DATABASE ApplicationDB SET RECOVERY FULL;
ALTER DATABASE ReportingDB SET RECOVERY FULL;
PRINT 'Recovery model set to FULL for both databases.';
GO

-- ============================================================================
-- STEP 5: Take Full Backups (Required for AG)
-- ============================================================================
PRINT '';
PRINT '=== Step 5: Taking Full Backups ===';

-- Backup ApplicationDB
BACKUP DATABASE ApplicationDB 
    TO DISK = '/var/opt/mssql/backup/ApplicationDB_full.bak'
    WITH FORMAT, INIT, NAME = 'ApplicationDB Full Backup';
PRINT 'ApplicationDB backed up.';

-- Backup ReportingDB
BACKUP DATABASE ReportingDB 
    TO DISK = '/var/opt/mssql/backup/ReportingDB_full.bak'
    WITH FORMAT, INIT, NAME = 'ReportingDB Full Backup';
PRINT 'ReportingDB backed up.';
GO

-- ============================================================================
-- STEP 6: Create the Availability Group (Run on PRIMARY only)
-- ============================================================================
PRINT '';
PRINT '=== Step 6: Creating Availability Group ===';

-- Check if this is intended to be the primary
IF NOT EXISTS (SELECT * FROM sys.availability_groups WHERE name = 'ProductionAG')
BEGIN
    -- Create the AG with 3 replicas
    -- Note: Adjust the endpoint URLs based on your pod naming convention
    CREATE AVAILABILITY GROUP ProductionAG
        WITH (
            CLUSTER_TYPE = EXTERNAL,
            DB_FAILOVER = ON,
            REQUIRED_SYNCHRONIZED_SECONDARIES_TO_COMMIT = 1
        )
        FOR DATABASE ApplicationDB, ReportingDB
        REPLICA ON
            N'sql-ag-prod01-0' WITH (
                ENDPOINT_URL = N'TCP://sql-ag-prod01-0.sql-ag-prod01-pods.mssql.svc.cluster.local:5022',
                AVAILABILITY_MODE = SYNCHRONOUS_COMMIT,
                FAILOVER_MODE = EXTERNAL,
                SEEDING_MODE = AUTOMATIC,
                SECONDARY_ROLE (ALLOW_CONNECTIONS = READ_ONLY)
            ),
            N'sql-ag-prod01-1' WITH (
                ENDPOINT_URL = N'TCP://sql-ag-prod01-1.sql-ag-prod01-pods.mssql.svc.cluster.local:5022',
                AVAILABILITY_MODE = SYNCHRONOUS_COMMIT,
                FAILOVER_MODE = EXTERNAL,
                SEEDING_MODE = AUTOMATIC,
                SECONDARY_ROLE (ALLOW_CONNECTIONS = READ_ONLY)
            ),
            N'sql-ag-prod01-2' WITH (
                ENDPOINT_URL = N'TCP://sql-ag-prod01-2.sql-ag-prod01-pods.mssql.svc.cluster.local:5022',
                AVAILABILITY_MODE = SYNCHRONOUS_COMMIT,
                FAILOVER_MODE = EXTERNAL,
                SEEDING_MODE = AUTOMATIC,
                SECONDARY_ROLE (ALLOW_CONNECTIONS = READ_ONLY)
            );
    
    PRINT 'Availability Group ProductionAG created.';
    PRINT '';
    PRINT '*** IMPORTANT: Now run join-secondary.sql on each secondary replica ***';
END
ELSE
BEGIN
    PRINT 'Availability Group ProductionAG already exists.';
END
GO

-- ============================================================================
-- STEP 7: Verify AG Status
-- ============================================================================
PRINT '';
PRINT '=== Step 7: Verifying AG Status ===';

SELECT 
    ag.name AS ag_name,
    ar.replica_server_name,
    ISNULL(ars.role_desc, 'NOT JOINED') AS role,
    ISNULL(ars.synchronization_health_desc, 'NOT JOINED') AS sync_health
FROM sys.availability_groups ag
JOIN sys.availability_replicas ar ON ag.group_id = ar.group_id
LEFT JOIN sys.dm_hadr_availability_replica_states ars ON ar.replica_id = ars.replica_id
ORDER BY ar.replica_server_name;

SELECT 
    d.name AS database_name,
    CASE WHEN drs.is_primary_replica = 1 THEN 'PRIMARY' ELSE 'SECONDARY' END AS replica_role,
    drs.synchronization_state_desc AS sync_state,
    drs.synchronization_health_desc AS sync_health
FROM sys.dm_hadr_database_replica_states drs
JOIN sys.databases d ON drs.database_id = d.database_id
WHERE drs.is_local = 1;
GO

PRINT '';
PRINT '=== AG Setup Complete on Primary ===';
PRINT 'Next steps:';
PRINT '1. Run join-secondary.sql on sql-ag-prod01-1';
PRINT '2. Run join-secondary.sql on sql-ag-prod01-2';
PRINT '3. Verify all replicas are synchronized';
GO
