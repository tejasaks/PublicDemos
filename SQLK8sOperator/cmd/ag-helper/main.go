/*
Copyright 2026 Microsoft Corporation.

Licensed under the MIT License.
See LICENSE file in the project root for full license information.

AG Helper Sidecar - Ported from mssql-server-ha pacemaker logic
This component runs as a sidecar container alongside SQL Server
and manages Availability Group health monitoring and failover.
*/

package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"regexp"
	"strings"
	"sync"
	"syscall"
	"time"

	_ "github.com/denisenkom/go-mssqldb"
	"k8s.io/klog/v2"
)

// AGRole represents the role of a replica in the AG
type AGRole string

const (
	RolePrimary      AGRole = "PRIMARY"
	RoleSecondary    AGRole = "SECONDARY"
	RoleResolving    AGRole = "RESOLVING"
	RoleConfigOnly   AGRole = "CONFIGURATION_ONLY"
	RoleNotAvailable AGRole = "NOT_AVAILABLE"
)

// SyncState represents the synchronization state
type SyncState string

const (
	StateSynchronized     SyncState = "SYNCHRONIZED"
	StateSynchronizing    SyncState = "SYNCHRONIZING"
	StateNotSynchronizing SyncState = "NOT_SYNCHRONIZING"
	StateSuspended        SyncState = "SUSPENDED"
)

// SQL Identifier validation pattern
// SQL Server allows identifiers that start with letter, _, @, or #
// and contain letters, digits, @, $, #, or _
var sqlIdentifierPattern = regexp.MustCompile(`^[a-zA-Z_@#][a-zA-Z0-9_@#$]{0,127}$`)

// validateSQLIdentifier validates a SQL Server identifier
func validateSQLIdentifier(name string, fieldName string) error {
	if name == "" {
		return fmt.Errorf("%s is required", fieldName)
	}
	if len(name) > 128 {
		return fmt.Errorf("%s '%s' exceeds maximum length of 128 characters", fieldName, truncateString(name, 20))
	}
	if !sqlIdentifierPattern.MatchString(name) {
		return fmt.Errorf("%s '%s' contains invalid characters", fieldName, truncateString(name, 20))
	}
	return nil
}

// sanitizeSQLIdentifier safely escapes a SQL identifier for use in dynamic SQL
// Uses bracket notation with proper escaping
func sanitizeSQLIdentifier(name string) string {
	if name == "" {
		return ""
	}
	// Validate first - if invalid, return empty to prevent injection
	if !sqlIdentifierPattern.MatchString(name) {
		klog.Errorf("Invalid SQL identifier rejected: %s", truncateString(name, 20))
		return ""
	}
	// Escape any existing brackets (shouldn't exist per pattern, but defense in depth)
	escaped := strings.ReplaceAll(name, "]", "]]")
	return "[" + escaped + "]"
}

// truncateString shortens a string for display in logs
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

// ConnectedState represents the connection state
type ConnectedState string

const (
	Connected    ConnectedState = "CONNECTED"
	Disconnected ConnectedState = "DISCONNECTED"
)

// AGState represents the complete state of the AG from this instance's perspective
type AGState struct {
	AGName           string          `json:"agName"`
	LocalReplicaName string          `json:"localReplicaName"`
	Role             AGRole          `json:"role"`
	SyncState        SyncState       `json:"syncState"`
	IsLocalPrimary   bool            `json:"isLocalPrimary"`
	SequenceNumber   int64           `json:"sequenceNumber"`
	Instances        []InstanceState `json:"instances"`
	Databases        []DatabaseState `json:"databases"`
	LastUpdated      time.Time       `json:"lastUpdated"`
	Health           string          `json:"health"`
	mu               sync.RWMutex
}

// InstanceState represents the state of a SQL Server instance in an AG
// Note: SQL Server documentation refers to these as "availability replicas", hence
// the 'ReplicaName' field which maps to SQL Server's replica_server_name column.
type InstanceState struct {
	ReplicaName          string         `json:"replicaName"`
	Role                 AGRole         `json:"role"`
	AvailabilityMode     string         `json:"availabilityMode"`
	FailoverMode         string         `json:"failoverMode"`
	SynchronizationState SyncState      `json:"synchronizationState"`
	ConnectedState       ConnectedState `json:"connectedState"`
	SequenceNumber       int64          `json:"sequenceNumber"`
	IsLocal              bool           `json:"isLocal"`
}

// DatabaseState represents the state of a database in the AG
type DatabaseState struct {
	DatabaseName         string    `json:"databaseName"`
	ReplicaName          string    `json:"replicaName"`
	IsPrimaryReplica     bool      `json:"isPrimaryReplica"`
	SynchronizationState SyncState `json:"synchronizationState"`
	IsSuspended          bool      `json:"isSuspended"`
	SuspendReason        string    `json:"suspendReason,omitempty"`
	LastHardenedLSN      int64     `json:"lastHardenedLsn"`
	LastCommitLSN        int64     `json:"lastCommitLsn"`
}

// FailoverRequest represents a request to failover the AG
type FailoverRequest struct {
	TargetReplica string `json:"targetReplica"`
	Force         bool   `json:"force"`
	AllowDataLoss bool   `json:"allowDataLoss"`
}

// AGHelper is the main struct for the AG helper sidecar
type AGHelper struct {
	agName            string // Name of the AG to monitor (required)
	connectionString  string
	db                *sql.DB
	state             *AGState
	monitorInterval   time.Duration
	connectionTimeout time.Duration
	httpPort          int
	stopCh            chan struct{}
}

// NewAGHelper creates a new AGHelper instance
func NewAGHelper(agName, connStr string, monitorInterval, connTimeout time.Duration, httpPort int) *AGHelper {
	return &AGHelper{
		agName:            agName,
		connectionString:  connStr,
		monitorInterval:   monitorInterval,
		connectionTimeout: connTimeout,
		httpPort:          httpPort,
		state: &AGState{
			AGName:      agName,
			LastUpdated: time.Now(),
		},
		stopCh: make(chan struct{}),
	}
}

// Connect establishes connection to SQL Server
func (h *AGHelper) Connect(ctx context.Context) error {
	db, err := sql.Open("sqlserver", h.connectionString)
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}

	db.SetConnMaxLifetime(5 * time.Minute)
	db.SetMaxIdleConns(2)
	db.SetMaxOpenConns(5)

	// Test connection
	ctx, cancel := context.WithTimeout(ctx, h.connectionTimeout)
	defer cancel()

	if err := db.PingContext(ctx); err != nil {
		return fmt.Errorf("failed to ping database: %w", err)
	}

	h.db = db
	klog.Info("Connected to SQL Server")
	return nil
}

// GetAGStateByName retrieves the state for a specific AG
func (h *AGHelper) GetAGStateByName(ctx context.Context, agName string) (*AGState, error) {
	state := &AGState{
		AGName:      agName,
		LastUpdated: time.Now(),
	}

	// Get local replica info for this specific AG
	role, err := h.getRoleForAG(ctx, agName)
	if err != nil {
		return nil, fmt.Errorf("failed to get role for AG %s: %w", agName, err)
	}
	state.Role = role
	state.IsLocalPrimary = role == RolePrimary

	// Get sequence number
	seqNum, err := h.getSequenceNumberForAG(ctx, agName)
	if err != nil {
		klog.V(4).Infof("Failed to get sequence number for AG %s: %v", agName, err)
	}
	state.SequenceNumber = seqNum

	// Get all instances for this AG
	instances, err := h.getInstanceStatesForAG(ctx, agName)
	if err != nil {
		klog.V(4).Infof("Failed to get instance states for AG %s: %v", agName, err)
	}
	state.Instances = instances

	// Get database states for this AG
	databases, err := h.getDatabaseStatesForAG(ctx, agName)
	if err != nil {
		klog.V(4).Infof("Failed to get database states for AG %s: %v", agName, err)
	}
	state.Databases = databases

	// Determine overall health
	state.Health = h.determineHealth(state)

	// Find local instance name
	for _, inst := range state.Instances {
		if inst.IsLocal {
			state.LocalReplicaName = inst.ReplicaName
			state.SyncState = inst.SynchronizationState
			break
		}
	}

	return state, nil
}

// getRoleForAG returns the role for a specific AG
func (h *AGHelper) getRoleForAG(ctx context.Context, agName string) (AGRole, error) {
	query := `
		SELECT role_desc
		FROM sys.dm_hadr_availability_replica_states ars
		JOIN sys.availability_replicas ar ON ars.replica_id = ar.replica_id
		WHERE is_local = 1 AND ar.group_id = (
			SELECT group_id FROM sys.availability_groups WHERE name = @agName
		)
	`

	var roleDesc string
	err := h.db.QueryRowContext(ctx, query, sql.Named("agName", agName)).Scan(&roleDesc)
	if err != nil {
		if err == sql.ErrNoRows {
			return RoleNotAvailable, nil
		}
		return RoleNotAvailable, err
	}

	return AGRole(roleDesc), nil
}

// getSequenceNumberForAG returns the sequence number for a specific AG
func (h *AGHelper) getSequenceNumberForAG(ctx context.Context, agName string) (int64, error) {
	query := `
		SELECT MAX(last_hardened_lsn)
		FROM sys.dm_hadr_database_replica_states drs
		JOIN sys.availability_groups ag ON drs.group_id = ag.group_id
		WHERE ag.name = @agName AND is_local = 1
	`

	var seqNum sql.NullInt64
	err := h.db.QueryRowContext(ctx, query, sql.Named("agName", agName)).Scan(&seqNum)
	if err != nil {
		return 0, err
	}

	if seqNum.Valid {
		return seqNum.Int64, nil
	}
	return 0, nil
}

// getInstanceStatesForAG retrieves states of all instances for a specific AG
func (h *AGHelper) getInstanceStatesForAG(ctx context.Context, agName string) ([]InstanceState, error) {
	query := `
		SELECT 
			ar.replica_server_name,
			ars.role_desc,
			ar.availability_mode_desc,
			ar.failover_mode_desc,
			ars.synchronization_health_desc,
			ars.connected_state_desc,
			ars.is_local,
			ISNULL(MAX(drs.last_hardened_lsn), 0) as seq_num
		FROM sys.availability_replicas ar
		JOIN sys.dm_hadr_availability_replica_states ars ON ar.replica_id = ars.replica_id
		LEFT JOIN sys.dm_hadr_database_replica_states drs ON ar.replica_id = drs.replica_id
		WHERE ar.group_id = (SELECT group_id FROM sys.availability_groups WHERE name = @agName)
		GROUP BY ar.replica_server_name, ars.role_desc, ar.availability_mode_desc, 
		         ar.failover_mode_desc, ars.synchronization_health_desc, ars.connected_state_desc, ars.is_local
	`

	rows, err := h.db.QueryContext(ctx, query, sql.Named("agName", agName))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var instances []InstanceState
	for rows.Next() {
		var inst InstanceState
		var syncHealth, connState string
		err := rows.Scan(
			&inst.ReplicaName,
			&inst.Role,
			&inst.AvailabilityMode,
			&inst.FailoverMode,
			&syncHealth,
			&connState,
			&inst.IsLocal,
			&inst.SequenceNumber,
		)
		if err != nil {
			klog.Warningf("Failed to scan instance row: %v", err)
			continue
		}

		switch syncHealth {
		case "HEALTHY":
			inst.SynchronizationState = StateSynchronized
		case "PARTIALLY_HEALTHY":
			inst.SynchronizationState = StateSynchronizing
		default:
			inst.SynchronizationState = StateNotSynchronizing
		}

		inst.ConnectedState = ConnectedState(connState)
		instances = append(instances, inst)
	}

	return instances, nil
}

// getDatabaseStatesForAG retrieves states of all databases for a specific AG
func (h *AGHelper) getDatabaseStatesForAG(ctx context.Context, agName string) ([]DatabaseState, error) {
	query := `
		SELECT 
			db.name,
			ar.replica_server_name,
			drs.is_primary_replica,
			drs.synchronization_state_desc,
			drs.is_suspended,
			ISNULL(drs.suspend_reason_desc, ''),
			ISNULL(drs.last_hardened_lsn, 0),
			ISNULL(drs.last_commit_lsn, 0)
		FROM sys.dm_hadr_database_replica_states drs
		JOIN sys.databases db ON drs.database_id = db.database_id
		JOIN sys.availability_replicas ar ON drs.replica_id = ar.replica_id
		WHERE drs.group_id = (SELECT group_id FROM sys.availability_groups WHERE name = @agName)
	`

	rows, err := h.db.QueryContext(ctx, query, sql.Named("agName", agName))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var databases []DatabaseState
	for rows.Next() {
		var d DatabaseState
		var syncState string
		err := rows.Scan(
			&d.DatabaseName,
			&d.ReplicaName,
			&d.IsPrimaryReplica,
			&syncState,
			&d.IsSuspended,
			&d.SuspendReason,
			&d.LastHardenedLSN,
			&d.LastCommitLSN,
		)
		if err != nil {
			klog.Warningf("Failed to scan database row: %v", err)
			continue
		}

		d.SynchronizationState = SyncState(syncState)
		databases = append(databases, d)
	}

	return databases, nil
}

// GetRole returns the current role of this replica
// Ported from mssql-server-ha: GetRole()
func (h *AGHelper) GetRole(ctx context.Context) (AGRole, error) {
	query := `
		SELECT role_desc
		FROM sys.dm_hadr_availability_replica_states ars
		JOIN sys.availability_replicas ar ON ars.replica_id = ar.replica_id
		WHERE is_local = 1 AND ar.group_id = (
			SELECT group_id FROM sys.availability_groups WHERE name = @agName
		)
	`

	var roleDesc string
	err := h.db.QueryRowContext(ctx, query, sql.Named("agName", h.agName)).Scan(&roleDesc)
	if err != nil {
		if err == sql.ErrNoRows {
			return RoleNotAvailable, nil
		}
		return RoleNotAvailable, err
	}

	return AGRole(roleDesc), nil
}

// GetSequenceNumber returns the sequence number for this replica
// Used to determine the most up-to-date replica for failover
// Ported from mssql-server-ha
func (h *AGHelper) GetSequenceNumber(ctx context.Context) (int64, error) {
	query := `
		SELECT MAX(last_hardened_lsn)
		FROM sys.dm_hadr_database_replica_states drs
		JOIN sys.availability_groups ag ON drs.group_id = ag.group_id
		WHERE ag.name = @agName AND is_local = 1
	`

	var seqNum sql.NullInt64
	err := h.db.QueryRowContext(ctx, query, sql.Named("agName", h.agName)).Scan(&seqNum)
	if err != nil {
		return 0, err
	}

	if seqNum.Valid {
		return seqNum.Int64, nil
	}
	return 0, nil
}

// GetAGState retrieves the complete AG state
func (h *AGHelper) GetAGState(ctx context.Context) (*AGState, error) {
	state := &AGState{
		AGName:      h.agName,
		LastUpdated: time.Now(),
	}

	// Get local replica info
	role, err := h.GetRole(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get role: %w", err)
	}
	state.Role = role
	state.IsLocalPrimary = role == RolePrimary

	// Get sequence number
	seqNum, err := h.GetSequenceNumber(ctx)
	if err != nil {
		klog.Warningf("Failed to get sequence number: %v", err)
	}
	state.SequenceNumber = seqNum

	// Get all instances
	instances, err := h.getInstanceStates(ctx)
	if err != nil {
		klog.Warningf("Failed to get instance states: %v", err)
	}
	state.Instances = instances

	// Get database states
	databases, err := h.getDatabaseStates(ctx)
	if err != nil {
		klog.Warningf("Failed to get database states: %v", err)
	}
	state.Databases = databases

	// Determine overall health
	state.Health = h.determineHealth(state)

	// Find local instance name
	for _, inst := range state.Instances {
		if inst.IsLocal {
			state.LocalReplicaName = inst.ReplicaName
			state.SyncState = inst.SynchronizationState
			break
		}
	}

	return state, nil
}

// getInstanceStates retrieves states of all instances
func (h *AGHelper) getInstanceStates(ctx context.Context) ([]InstanceState, error) {
	query := `
		SELECT 
			ar.replica_server_name,
			ars.role_desc,
			ar.availability_mode_desc,
			ar.failover_mode_desc,
			ars.synchronization_health_desc,
			ars.connected_state_desc,
			ars.is_local,
			ISNULL(MAX(drs.last_hardened_lsn), 0) as seq_num
		FROM sys.availability_replicas ar
		JOIN sys.dm_hadr_availability_replica_states ars ON ar.replica_id = ars.replica_id
		LEFT JOIN sys.dm_hadr_database_replica_states drs ON ar.replica_id = drs.replica_id
		WHERE ar.group_id = (SELECT group_id FROM sys.availability_groups WHERE name = @agName)
		GROUP BY ar.replica_server_name, ars.role_desc, ar.availability_mode_desc, 
		         ar.failover_mode_desc, ars.synchronization_health_desc, ars.connected_state_desc, ars.is_local
	`

	rows, err := h.db.QueryContext(ctx, query, sql.Named("agName", h.agName))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var instances []InstanceState
	for rows.Next() {
		var inst InstanceState
		var syncHealth, connState string
		err := rows.Scan(
			&inst.ReplicaName,
			&inst.Role,
			&inst.AvailabilityMode,
			&inst.FailoverMode,
			&syncHealth,
			&connState,
			&inst.IsLocal,
			&inst.SequenceNumber,
		)
		if err != nil {
			klog.Warningf("Failed to scan instance row: %v", err)
			continue
		}

		// Map sync health to sync state
		switch syncHealth {
		case "HEALTHY":
			inst.SynchronizationState = StateSynchronized
		case "PARTIALLY_HEALTHY":
			inst.SynchronizationState = StateSynchronizing
		default:
			inst.SynchronizationState = StateNotSynchronizing
		}

		inst.ConnectedState = ConnectedState(connState)
		instances = append(instances, inst)
	}

	return instances, nil
}

// getDatabaseStates retrieves states of all databases in the AG
func (h *AGHelper) getDatabaseStates(ctx context.Context) ([]DatabaseState, error) {
	query := `
		SELECT 
			db.name,
			ar.replica_server_name,
			drs.is_primary_replica,
			drs.synchronization_state_desc,
			drs.is_suspended,
			ISNULL(drs.suspend_reason_desc, ''),
			ISNULL(drs.last_hardened_lsn, 0),
			ISNULL(drs.last_commit_lsn, 0)
		FROM sys.dm_hadr_database_replica_states drs
		JOIN sys.databases db ON drs.database_id = db.database_id
		JOIN sys.availability_replicas ar ON drs.replica_id = ar.replica_id
		WHERE drs.group_id = (SELECT group_id FROM sys.availability_groups WHERE name = @agName)
	`

	rows, err := h.db.QueryContext(ctx, query, sql.Named("agName", h.agName))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var databases []DatabaseState
	for rows.Next() {
		var d DatabaseState
		var syncState string
		err := rows.Scan(
			&d.DatabaseName,
			&d.ReplicaName,
			&d.IsPrimaryReplica,
			&syncState,
			&d.IsSuspended,
			&d.SuspendReason,
			&d.LastHardenedLSN,
			&d.LastCommitLSN,
		)
		if err != nil {
			klog.Warningf("Failed to scan database row: %v", err)
			continue
		}

		d.SynchronizationState = SyncState(syncState)
		databases = append(databases, d)
	}

	return databases, nil
}

// determineHealth calculates overall health based on state
func (h *AGHelper) determineHealth(state *AGState) string {
	// If AG doesn't exist yet, report as "Waiting" not "Critical"
	// This allows the sidecar to run before AG is configured
	if state.Role == RoleNotAvailable {
		// Check if AG actually exists in sys.availability_groups
		if len(state.Instances) == 0 {
			return "Waiting" // AG not configured yet - this is OK during initial setup
		}
		return "Critical" // AG exists but this instance can't see it - real problem
	}

	syncedCount := 0
	for _, inst := range state.Instances {
		if inst.SynchronizationState == StateSynchronized {
			syncedCount++
		}
	}

	if syncedCount == 0 {
		// If we're primary with no synced secondaries, could be initial seeding
		if state.IsLocalPrimary && len(state.Databases) == 0 {
			return "Waiting" // No databases in AG yet
		}
		return "Critical"
	}
	if syncedCount < len(state.Instances) {
		return "Warning"
	}
	return "Healthy"
}

// Failover performs a failover to this replica
// Ported from mssql-server-ha: Failover()
func (h *AGHelper) Failover(ctx context.Context, allowDataLoss bool) error {
	// Validate AG name before using in SQL
	if err := validateSQLIdentifier(h.agName, "AG name"); err != nil {
		return fmt.Errorf("invalid AG name: %w", err)
	}

	role, err := h.GetRole(ctx)
	if err != nil {
		return fmt.Errorf("failed to get current role: %w", err)
	}

	if role == RolePrimary {
		klog.Info("Already primary, no failover needed")
		return nil
	}

	// Sanitize AG name for safe SQL execution
	safeAGName := sanitizeSQLIdentifier(h.agName)
	if safeAGName == "" {
		return fmt.Errorf("AG name sanitization failed")
	}

	var failoverQuery string
	if allowDataLoss {
		// Force failover with potential data loss
		failoverQuery = fmt.Sprintf(`ALTER AVAILABILITY GROUP %s FORCE_FAILOVER_ALLOW_DATA_LOSS`, safeAGName)
		klog.Warning("Performing force failover with potential data loss")
	} else {
		// Normal failover (requires sync)
		failoverQuery = fmt.Sprintf(`ALTER AVAILABILITY GROUP %s FAILOVER`, safeAGName)
		klog.Info("Performing planned failover")
	}

	_, err = h.db.ExecContext(ctx, failoverQuery)
	if err != nil {
		return fmt.Errorf("failover failed: %w", err)
	}

	klog.Info("Failover completed successfully")
	return nil
}

// MonitorLoop continuously monitors the AG state
func (h *AGHelper) MonitorLoop(ctx context.Context) {
	ticker := time.NewTicker(h.monitorInterval)
	defer ticker.Stop()

	lastHealth := ""
	waitingLogCount := 0

	for {
		select {
		case <-ctx.Done():
			return
		case <-h.stopCh:
			return
		case <-ticker.C:
			h.monitorAG(ctx, &lastHealth, &waitingLogCount)
		}
	}
}

// monitorAG monitors the configured AG
func (h *AGHelper) monitorAG(ctx context.Context, lastHealth *string, waitingLogCount *int) {
	state, err := h.GetAGState(ctx)
	if err != nil {
		klog.Errorf("Failed to get AG state: %v", err)
		return
	}

	h.state.mu.Lock()
	h.state = state
	h.state.mu.Unlock()

	// Reduce log noise for "Waiting" state
	if state.Health == "Waiting" {
		*waitingLogCount++
		// Only log every 6th iteration (once per minute with 10s interval)
		if *waitingLogCount%6 == 1 {
			klog.Infof("AG '%s' not configured yet, waiting... (role=%s)", h.agName, state.Role)
		}
	} else {
		*waitingLogCount = 0
		// Log state changes
		if state.Health != *lastHealth {
			klog.Infof("AG '%s' State changed: health=%s, role=%s, sync=%s",
				h.agName, state.Health, state.Role, state.SyncState)
		} else {
			klog.V(4).Infof("AG '%s' State: role=%s, sync=%s, health=%s",
				h.agName, state.Role, state.SyncState, state.Health)
		}
	}
	*lastHealth = state.Health
}

// HTTP Handlers

// handleHealth returns the health status
func (h *AGHelper) handleHealth(w http.ResponseWriter, r *http.Request) {
	h.state.mu.RLock()
	defer h.state.mu.RUnlock()

	w.Header().Set("Content-Type", "application/json")

	// "Waiting" is acceptable for liveness - pod is alive, just waiting for AG setup
	// "Healthy" and "Warning" are also OK
	// Only "Critical" returns 503
	switch h.state.Health {
	case "Healthy", "Warning", "Waiting":
		w.WriteHeader(http.StatusOK)
	default:
		w.WriteHeader(http.StatusServiceUnavailable)
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"status": h.state.Health,
		"role":   h.state.Role,
	})
}

// handleState returns the complete AG state
func (h *AGHelper) handleState(w http.ResponseWriter, r *http.Request) {
	h.state.mu.RLock()
	defer h.state.mu.RUnlock()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(h.state)
}

// handleReady returns readiness status (stricter than health)
// Use for Kubernetes readiness probe - only ready when AG is functional
func (h *AGHelper) handleReady(w http.ResponseWriter, r *http.Request) {
	h.state.mu.RLock()
	defer h.state.mu.RUnlock()

	w.Header().Set("Content-Type", "application/json")

	// For readiness, we require the AG to actually be configured and working
	// "Waiting" means not ready to receive traffic
	switch h.state.Health {
	case "Healthy", "Warning":
		w.WriteHeader(http.StatusOK)
	default:
		w.WriteHeader(http.StatusServiceUnavailable)
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"ready":  h.state.Health == "Healthy" || h.state.Health == "Warning",
		"status": h.state.Health,
		"role":   h.state.Role,
	})
}

// handleRole returns just the role
func (h *AGHelper) handleRole(w http.ResponseWriter, r *http.Request) {
	h.state.mu.RLock()
	defer h.state.mu.RUnlock()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"role":        h.state.Role,
		"isPrimary":   h.state.IsLocalPrimary,
		"replicaName": h.state.LocalReplicaName,
	})
}

// handleFailover handles failover requests
func (h *AGHelper) handleFailover(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req FailoverRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Minute)
	defer cancel()

	if err := h.Failover(ctx, req.AllowDataLoss); err != nil {
		klog.Errorf("Failover failed: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{
		"status": "failover initiated",
	})
}

// handleSequence returns the sequence number
func (h *AGHelper) handleSequence(w http.ResponseWriter, r *http.Request) {
	h.state.mu.RLock()
	defer h.state.mu.RUnlock()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"sequenceNumber": h.state.SequenceNumber,
		"replicaName":    h.state.LocalReplicaName,
	})
}

// ListenerInfo represents the AG Listener configuration from SQL Server
type ListenerInfo struct {
	ListenerName string `json:"listenerName"`
	IPAddress    string `json:"ipAddress"`
	Port         int32  `json:"port"`
	State        string `json:"state"` // ONLINE, PENDING, PENDING_OFFLINE
	AGName       string `json:"agName"`
}

// handleListener returns the AG Listener information if configured
func (h *AGHelper) handleListener(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	listener, err := h.getListenerInfo(ctx, h.agName)
	if err != nil {
		klog.V(4).Infof("Error getting listener info: %v", err)
		// Return empty listener with error - not an HTTP error since AG might not have a listener
		json.NewEncoder(w).Encode(map[string]interface{}{
			"hasListener": false,
			"error":       err.Error(),
			"agName":      h.agName,
		})
		return
	}

	if listener == nil {
		// No listener configured - this is normal, not an error
		json.NewEncoder(w).Encode(map[string]interface{}{
			"hasListener": false,
			"agName":      h.agName,
			"message":     "No AG Listener configured for this Availability Group",
		})
		return
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"hasListener": true,
		"listener":    listener,
		"agName":      h.agName,
	})
}

// getListenerInfo queries SQL Server for the AG Listener configuration
func (h *AGHelper) getListenerInfo(ctx context.Context, agName string) (*ListenerInfo, error) {
	if h.db == nil {
		return nil, fmt.Errorf("database connection not available")
	}

	// Validate AG name before using in query
	if err := validateSQLIdentifier(agName, "agName"); err != nil {
		return nil, err
	}

	// Query to get AG listener information
	// Uses parameterized query to prevent SQL injection
	query := `
		SELECT 
			agl.dns_name AS listener_name,
			ip.ip_address,
			agl.port,
			agl.state_desc AS state
		FROM sys.availability_group_listeners agl
		JOIN sys.availability_groups ag ON agl.group_id = ag.group_id
		JOIN sys.availability_group_listener_ip_addresses ip ON agl.listener_id = ip.listener_id
		WHERE ag.name = @agName
	`

	rows, err := h.db.QueryContext(ctx, query, sql.Named("agName", agName))
	if err != nil {
		return nil, fmt.Errorf("failed to query listener info: %w", err)
	}
	defer rows.Close()

	if !rows.Next() {
		// No listener found
		return nil, nil
	}

	var listener ListenerInfo
	var ipAddress sql.NullString

	if err := rows.Scan(&listener.ListenerName, &ipAddress, &listener.Port, &listener.State); err != nil {
		return nil, fmt.Errorf("failed to scan listener info: %w", err)
	}

	listener.AGName = agName
	if ipAddress.Valid {
		listener.IPAddress = ipAddress.String
	}

	return &listener, nil
}

// StartHTTPServer starts the HTTP server for the sidecar API
func (h *AGHelper) StartHTTPServer(ctx context.Context) error {
	mux := http.NewServeMux()
	mux.HandleFunc("/health", h.handleHealth)  // Liveness - passes even when waiting for AG
	mux.HandleFunc("/healthz", h.handleHealth) // Alias for liveness
	mux.HandleFunc("/ready", h.handleReady)    // Readiness - only passes when AG is functional
	mux.HandleFunc("/readyz", h.handleReady)   // Alias for readiness
	mux.HandleFunc("/state", h.handleState)
	mux.HandleFunc("/role", h.handleRole)
	mux.HandleFunc("/failover", h.handleFailover)
	mux.HandleFunc("/sequence", h.handleSequence)
	mux.HandleFunc("/listener", h.handleListener) // AG Listener information

	server := &http.Server{
		Addr:         fmt.Sprintf(":%d", h.httpPort),
		Handler:      mux,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 30 * time.Second,
	}

	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		server.Shutdown(shutdownCtx)
	}()

	klog.Infof("Starting HTTP server on port %d", h.httpPort)
	return server.ListenAndServe()
}

// Stop gracefully stops the AGHelper
func (h *AGHelper) Stop() {
	close(h.stopCh)
	if h.db != nil {
		h.db.Close()
	}
}

// getEnvWithFallback returns the first non-empty environment variable value,
// or the default if none are set. This allows backward compatibility with SA_PASSWORD
// while preferring the new AG_HELPER_* variables.
func getEnvWithFallback(primary, fallback, defaultVal string) string {
	if val := os.Getenv(primary); val != "" {
		return val
	}
	if val := os.Getenv(fallback); val != "" {
		return val
	}
	return defaultVal
}

// logCredentialSource logs which credential source is being used (for debugging)
// and emits warnings for insecure configurations
func logCredentialSource(username string) {
	// Determine credential source for logging
	if os.Getenv("AG_HELPER_USERNAME") != "" && os.Getenv("AG_HELPER_PASSWORD") != "" {
		klog.Info("Using AG Helper credentials (AG_HELPER_USERNAME/AG_HELPER_PASSWORD)")
		if username != "sa" {
			klog.Info("Using dedicated health check login (recommended for production)")
		}
	} else if os.Getenv("SA_PASSWORD") != "" {
		klog.Warning("Using SA credentials for AG health monitoring. For production, create a dedicated SQL login with VIEW SERVER STATE and ALTER ANY AVAILABILITY GROUP permissions.")
	}

	// Warn if using SA account
	if username == "sa" {
		klog.Warning("AG Helper is using the 'sa' account. Consider creating a dedicated least-privilege login for health monitoring.")
	}
}

func main() {
	var (
		agName            string
		sqlHost           string
		sqlPort           int
		sqlUser           string
		sqlPassword       string
		monitorInterval   time.Duration
		connectionTimeout time.Duration
		httpPort          int
	)

	flag.StringVar(&agName, "ag-name", os.Getenv("AG_NAME"), "Name of the Availability Group")
	flag.StringVar(&sqlHost, "sql-host", "localhost", "SQL Server host")
	flag.IntVar(&sqlPort, "sql-port", 1433, "SQL Server port")
	// AG Helper credentials - dedicated least-privilege SQL login for health monitoring
	// Following the Pacemaker pattern from Microsoft's SQL Server Linux AG documentation
	flag.StringVar(&sqlUser, "sql-user", getEnvWithFallback("AG_HELPER_USERNAME", "SA_USER", "sa"), "SQL Server user for AG health monitoring")
	flag.StringVar(&sqlPassword, "sql-password", getEnvWithFallback("AG_HELPER_PASSWORD", "SA_PASSWORD", ""), "SQL Server password for AG health monitoring")
	flag.DurationVar(&monitorInterval, "monitor-interval", 10*time.Second, "Interval between AG state checks")
	flag.DurationVar(&connectionTimeout, "connection-timeout", 30*time.Second, "SQL connection timeout")
	flag.IntVar(&httpPort, "http-port", 8080, "HTTP API port")

	klog.InitFlags(nil)
	flag.Parse()

	if agName == "" {
		klog.Fatal("AG name is required")
	}
	if sqlPassword == "" {
		klog.Fatal("AG helper password is required (set AG_HELPER_PASSWORD or SA_PASSWORD environment variable)")
	}

	// Log credential source for debugging (never log actual credentials)
	logCredentialSource(sqlUser)

	connStr := fmt.Sprintf("server=%s;port=%d;user id=%s;password=%s;database=master;connection timeout=30",
		sqlHost, sqlPort, sqlUser, sqlPassword)

	helper := NewAGHelper(agName, connStr, monitorInterval, connectionTimeout, httpPort)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle signals
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	// Connect with retry
	for i := 0; i < 30; i++ {
		if err := helper.Connect(ctx); err != nil {
			klog.Warningf("Failed to connect to SQL Server (attempt %d/30): %v", i+1, err)
			time.Sleep(10 * time.Second)
			continue
		}
		break
	}

	if helper.db == nil {
		klog.Fatal("Failed to connect to SQL Server after 30 attempts")
	}

	// Start monitoring loop
	go helper.MonitorLoop(ctx)

	// Start HTTP server in goroutine
	go func() {
		if err := helper.StartHTTPServer(ctx); err != http.ErrServerClosed {
			klog.Errorf("HTTP server error: %v", err)
		}
	}()

	klog.Infof("AG Helper started for AG: %s", agName)

	// Wait for signal
	sig := <-sigCh
	klog.Infof("Received signal %v, shutting down", sig)

	helper.Stop()
	cancel()

	klog.Info("AG Helper stopped")
}
