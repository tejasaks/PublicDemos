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
	StateSynchronized    SyncState = "SYNCHRONIZED"
	StateSynchronizing   SyncState = "SYNCHRONIZING"
	StateNotSynchronizing SyncState = "NOT_SYNCHRONIZING"
	StateSuspended       SyncState = "SUSPENDED"
)

// ConnectedState represents the connection state
type ConnectedState string

const (
	Connected    ConnectedState = "CONNECTED"
	Disconnected ConnectedState = "DISCONNECTED"
)

// AGState represents the complete state of the AG from this replica's perspective
type AGState struct {
	AGName           string                   `json:"agName"`
	LocalReplicaName string                   `json:"localReplicaName"`
	Role             AGRole                   `json:"role"`
	SyncState        SyncState                `json:"syncState"`
	IsLocalPrimary   bool                     `json:"isLocalPrimary"`
	SequenceNumber   int64                    `json:"sequenceNumber"`
	Replicas         []ReplicaState           `json:"replicas"`
	Databases        []DatabaseState          `json:"databases"`
	LastUpdated      time.Time                `json:"lastUpdated"`
	Health           string                   `json:"health"`
	mu               sync.RWMutex
}

// ReplicaState represents the state of an AG replica
type ReplicaState struct {
	ReplicaName         string         `json:"replicaName"`
	Role                AGRole         `json:"role"`
	AvailabilityMode    string         `json:"availabilityMode"`
	FailoverMode        string         `json:"failoverMode"`
	SynchronizationState SyncState      `json:"synchronizationState"`
	ConnectedState      ConnectedState `json:"connectedState"`
	SequenceNumber      int64          `json:"sequenceNumber"`
	IsLocal             bool           `json:"isLocal"`
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
	agName            string
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

	// Get all replicas
	replicas, err := h.getReplicaStates(ctx)
	if err != nil {
		klog.Warningf("Failed to get replica states: %v", err)
	}
	state.Replicas = replicas

	// Get database states
	databases, err := h.getDatabaseStates(ctx)
	if err != nil {
		klog.Warningf("Failed to get database states: %v", err)
	}
	state.Databases = databases

	// Determine overall health
	state.Health = h.determineHealth(state)

	// Find local replica name
	for _, r := range state.Replicas {
		if r.IsLocal {
			state.LocalReplicaName = r.ReplicaName
			state.SyncState = r.SynchronizationState
			break
		}
	}

	return state, nil
}

// getReplicaStates retrieves states of all replicas
func (h *AGHelper) getReplicaStates(ctx context.Context) ([]ReplicaState, error) {
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

	var replicas []ReplicaState
	for rows.Next() {
		var r ReplicaState
		var syncHealth, connState string
		err := rows.Scan(
			&r.ReplicaName,
			&r.Role,
			&r.AvailabilityMode,
			&r.FailoverMode,
			&syncHealth,
			&connState,
			&r.IsLocal,
			&r.SequenceNumber,
		)
		if err != nil {
			klog.Warningf("Failed to scan replica row: %v", err)
			continue
		}

		// Map sync health to sync state
		switch syncHealth {
		case "HEALTHY":
			r.SynchronizationState = StateSynchronized
		case "PARTIALLY_HEALTHY":
			r.SynchronizationState = StateSynchronizing
		default:
			r.SynchronizationState = StateNotSynchronizing
		}

		r.ConnectedState = ConnectedState(connState)
		replicas = append(replicas, r)
	}

	return replicas, nil
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
	if state.Role == RoleNotAvailable {
		return "Critical"
	}

	syncedCount := 0
	for _, r := range state.Replicas {
		if r.SynchronizationState == StateSynchronized {
			syncedCount++
		}
	}

	if syncedCount == 0 {
		return "Critical"
	}
	if syncedCount < len(state.Replicas) {
		return "Warning"
	}
	return "Healthy"
}

// Failover performs a failover to this replica
// Ported from mssql-server-ha: Failover()
func (h *AGHelper) Failover(ctx context.Context, allowDataLoss bool) error {
	role, err := h.GetRole(ctx)
	if err != nil {
		return fmt.Errorf("failed to get current role: %w", err)
	}

	if role == RolePrimary {
		klog.Info("Already primary, no failover needed")
		return nil
	}

	var failoverQuery string
	if allowDataLoss {
		// Force failover with potential data loss
		failoverQuery = fmt.Sprintf(`ALTER AVAILABILITY GROUP [%s] FORCE_FAILOVER_ALLOW_DATA_LOSS`, h.agName)
		klog.Warning("Performing force failover with potential data loss")
	} else {
		// Normal failover (requires sync)
		failoverQuery = fmt.Sprintf(`ALTER AVAILABILITY GROUP [%s] FAILOVER`, h.agName)
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

	for {
		select {
		case <-ctx.Done():
			return
		case <-h.stopCh:
			return
		case <-ticker.C:
			state, err := h.GetAGState(ctx)
			if err != nil {
				klog.Errorf("Failed to get AG state: %v", err)
				continue
			}

			h.state.mu.Lock()
			h.state = state
			h.state.mu.Unlock()

			klog.V(4).Infof("AG State: role=%s, sync=%s, health=%s",
				state.Role, state.SyncState, state.Health)
		}
	}
}

// HTTP Handlers

// handleHealth returns the health status
func (h *AGHelper) handleHealth(w http.ResponseWriter, r *http.Request) {
	h.state.mu.RLock()
	defer h.state.mu.RUnlock()

	w.Header().Set("Content-Type", "application/json")

	if h.state.Health == "Healthy" || h.state.Health == "Warning" {
		w.WriteHeader(http.StatusOK)
	} else {
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

// handleRole returns just the role
func (h *AGHelper) handleRole(w http.ResponseWriter, r *http.Request) {
	h.state.mu.RLock()
	defer h.state.mu.RUnlock()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"role":       h.state.Role,
		"isPrimary":  h.state.IsLocalPrimary,
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

// StartHTTPServer starts the HTTP server for the sidecar API
func (h *AGHelper) StartHTTPServer(ctx context.Context) error {
	mux := http.NewServeMux()
	mux.HandleFunc("/health", h.handleHealth)
	mux.HandleFunc("/healthz", h.handleHealth)
	mux.HandleFunc("/state", h.handleState)
	mux.HandleFunc("/role", h.handleRole)
	mux.HandleFunc("/failover", h.handleFailover)
	mux.HandleFunc("/sequence", h.handleSequence)

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
	flag.StringVar(&sqlUser, "sql-user", "sa", "SQL Server user")
	flag.StringVar(&sqlPassword, "sql-password", os.Getenv("SA_PASSWORD"), "SQL Server password")
	flag.DurationVar(&monitorInterval, "monitor-interval", 10*time.Second, "Interval between AG state checks")
	flag.DurationVar(&connectionTimeout, "connection-timeout", 30*time.Second, "SQL connection timeout")
	flag.IntVar(&httpPort, "http-port", 8080, "HTTP API port")

	klog.InitFlags(nil)
	flag.Parse()

	if agName == "" {
		klog.Fatal("AG name is required")
	}
	if sqlPassword == "" {
		klog.Fatal("SA password is required")
	}

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
