-- ============================================================================
-- Verify Availability Group Status
-- Run on any replica to check AG health
-- ============================================================================
--
-- Usage:
--   kubectl exec -it sql-ag-prod01-0 -n mssql -- /opt/mssql-tools18/bin/sqlcmd \
--     -S localhost -U sa -P 'YourStrong@Passw0rd!' -C -i /scripts/verify-ag-status.sql
--
-- ============================================================================

PRINT '=== Availability Group Status ===';
PRINT 'Server: ' + @@SERVERNAME;
PRINT '';

-- ============================================================================
-- AG Overview
-- ============================================================================
SELECT 
    ag.name AS [AG Name],
    ag.automated_backup_preference_desc AS [Backup Preference],
    ag.failure_condition_level AS [Failure Condition Level],
    ag.required_synchronized_secondaries_to_commit AS [Required Sync Secondaries]
FROM sys.availability_groups ag;

-- ============================================================================
-- Replica Status
-- ============================================================================
PRINT '';
PRINT '=== Replica Status ===';

SELECT 
    ar.replica_server_name AS [Replica],
    CASE WHEN ars.is_local = 1 THEN 'YES' ELSE 'NO' END AS [Local],
    ars.role_desc AS [Role],
    ar.availability_mode_desc AS [Availability Mode],
    ar.failover_mode_desc AS [Failover Mode],
    ars.connected_state_desc AS [Connected],
    ars.synchronization_health_desc AS [Sync Health],
    ars.operational_state_desc AS [Operational State]
FROM sys.availability_groups ag
JOIN sys.availability_replicas ar ON ag.group_id = ar.group_id
JOIN sys.dm_hadr_availability_replica_states ars ON ar.replica_id = ars.replica_id
ORDER BY ars.role_desc DESC, ar.replica_server_name;

-- ============================================================================
-- Database Status
-- ============================================================================
PRINT '';
PRINT '=== Database Status ===';

SELECT 
    d.name AS [Database],
    ar.replica_server_name AS [Replica],
    CASE WHEN drs.is_primary_replica = 1 THEN 'PRIMARY' ELSE 'SECONDARY' END AS [Role],
    drs.synchronization_state_desc AS [Sync State],
    drs.synchronization_health_desc AS [Sync Health],
    drs.is_suspended AS [Suspended],
    drs.suspend_reason_desc AS [Suspend Reason],
    drs.log_send_queue_size AS [Log Queue (KB)],
    drs.redo_queue_size AS [Redo Queue (KB)]
FROM sys.dm_hadr_database_replica_states drs
JOIN sys.databases d ON drs.database_id = d.database_id
JOIN sys.availability_replicas ar ON drs.replica_id = ar.replica_id
ORDER BY d.name, ar.replica_server_name;

-- ============================================================================
-- Automatic Seeding Status (if databases are being seeded)
-- ============================================================================
PRINT '';
PRINT '=== Automatic Seeding Status ===';

SELECT 
    ag.name AS [AG Name],
    ar.replica_server_name AS [Replica],
    d.name AS [Database],
    has.current_state AS [Seeding State],
    has.failure_state_desc AS [Failure State],
    has.start_time AS [Start Time],
    has.completion_percentage AS [Progress %]
FROM sys.dm_hadr_automatic_seeding has
JOIN sys.availability_groups ag ON has.ag_id = ag.group_id
JOIN sys.availability_replicas ar ON has.replica_id = ar.replica_id
JOIN sys.databases d ON has.local_database_id = d.database_id
WHERE has.is_source = 0; -- Show destination replicas

-- ============================================================================
-- Cluster Endpoint Status
-- ============================================================================
PRINT '';
PRINT '=== Endpoint Status ===';

SELECT 
    name AS [Endpoint Name],
    protocol_desc AS [Protocol],
    port AS [Port],
    state_desc AS [State],
    role_desc AS [Role]
FROM sys.database_mirroring_endpoints;

-- ============================================================================
-- Summary
-- ============================================================================
PRINT '';
PRINT '=== Summary ===';

DECLARE @PrimaryCount INT, @SyncedCount INT, @TotalReplicas INT;

SELECT @TotalReplicas = COUNT(*),
       @PrimaryCount = SUM(CASE WHEN ars.role_desc = 'PRIMARY' THEN 1 ELSE 0 END),
       @SyncedCount = SUM(CASE WHEN ars.synchronization_health_desc = 'HEALTHY' THEN 1 ELSE 0 END)
FROM sys.dm_hadr_availability_replica_states ars;

PRINT 'Total Replicas: ' + CAST(@TotalReplicas AS VARCHAR);
PRINT 'Primary Replicas: ' + CAST(@PrimaryCount AS VARCHAR);
PRINT 'Synchronized Replicas: ' + CAST(@SyncedCount AS VARCHAR);

IF @SyncedCount = @TotalReplicas
    PRINT 'STATUS: All replicas are synchronized - AG is healthy!';
ELSE
    PRINT 'WARNING: Not all replicas are synchronized!';
GO
