-- ============================================================================
-- Join Secondary Replica to Availability Group
-- Run this script on EACH secondary replica
-- ============================================================================
--
-- Usage:
--   # Connect to secondary pod and run this script
--   kubectl exec -it sql-ag-prod01-1 -n mssql -- /opt/mssql-tools18/bin/sqlcmd \
--     -S localhost -U sa -P 'YourStrong@Passw0rd!' -C -i /scripts/join-secondary.sql
--
--   kubectl exec -it sql-ag-prod01-2 -n mssql -- /opt/mssql-tools18/bin/sqlcmd \
--     -S localhost -U sa -P 'YourStrong@Passw0rd!' -C -i /scripts/join-secondary.sql
--
-- ============================================================================

PRINT 'Joining this replica to the Availability Group...';
PRINT 'Server: ' + @@SERVERNAME;
GO

-- ============================================================================
-- STEP 1: Create Master Key and Certificate (same as primary)
-- ============================================================================
PRINT '';
PRINT '=== Step 1: Creating Master Key and Certificate ===';

IF NOT EXISTS (SELECT * FROM sys.symmetric_keys WHERE name = '##MS_DatabaseMasterKey##')
BEGIN
    CREATE MASTER KEY ENCRYPTION BY PASSWORD = 'MasterKeyP@ssw0rd!';
    PRINT 'Master key created.';
END
GO

IF NOT EXISTS (SELECT * FROM sys.certificates WHERE name = 'AG_Auth_Cert')
BEGIN
    CREATE CERTIFICATE AG_Auth_Cert
        WITH SUBJECT = 'AG Authentication Certificate',
        EXPIRY_DATE = '2030-12-31';
    PRINT 'Certificate AG_Auth_Cert created.';
END
GO

-- ============================================================================
-- STEP 2: Create Database Mirroring Endpoint
-- ============================================================================
PRINT '';
PRINT '=== Step 2: Creating Database Mirroring Endpoint ===';

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
    PRINT 'Endpoint AG_Endpoint created.';
END
GO

-- ============================================================================
-- STEP 3: Join the Availability Group
-- ============================================================================
PRINT '';
PRINT '=== Step 3: Joining Availability Group ===';

-- Join this replica to the AG
ALTER AVAILABILITY GROUP ProductionAG JOIN WITH (CLUSTER_TYPE = EXTERNAL);
PRINT 'Joined Availability Group ProductionAG.';
GO

-- Grant permission to create databases (for automatic seeding)
ALTER AVAILABILITY GROUP ProductionAG GRANT CREATE ANY DATABASE;
PRINT 'Granted CREATE ANY DATABASE permission for automatic seeding.';
GO

-- ============================================================================
-- STEP 4: Verify Join Status
-- ============================================================================
PRINT '';
PRINT '=== Step 4: Verifying Join Status ===';

-- Wait a few seconds for seeding to start
WAITFOR DELAY '00:00:05';

-- Check replica status
SELECT 
    ag.name AS ag_name,
    @@SERVERNAME AS this_server,
    ars.role_desc AS role,
    ars.connected_state_desc AS connected_state,
    ars.synchronization_health_desc AS sync_health
FROM sys.availability_groups ag
JOIN sys.dm_hadr_availability_replica_states ars ON ag.group_id = ars.group_id
WHERE ars.is_local = 1;

-- Check database seeding progress
SELECT 
    d.name AS database_name,
    drs.synchronization_state_desc AS sync_state,
    drs.synchronization_health_desc AS sync_health,
    drs.log_send_queue_size AS log_queue_kb,
    drs.redo_queue_size AS redo_queue_kb
FROM sys.dm_hadr_database_replica_states drs
JOIN sys.databases d ON drs.database_id = d.database_id
WHERE drs.is_local = 1;
GO

PRINT '';
PRINT '=== Secondary Replica Join Complete ===';
PRINT 'Databases are being seeded automatically.';
PRINT 'Monitor progress with: SELECT * FROM sys.dm_hadr_automatic_seeding;';
GO
