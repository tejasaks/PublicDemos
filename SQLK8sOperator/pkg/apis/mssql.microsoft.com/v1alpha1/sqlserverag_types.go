/*
Copyright 2026 Microsoft Corporation.

Licensed under the MIT License.
See LICENSE file in the project root for full license information.
*/

package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// SQLServerAGSpec defines the desired state of SQLServerAG (Availability Group)
type SQLServerAGSpec struct {
	// Description is an optional human-readable description for auditing and searchability
	// +optional
	// +kubebuilder:validation:MaxLength=1024
	Description string `json:"description,omitempty"`

	// SQLServerRef references the SQLServer instance to configure as an AG
	SQLServerRef corev1.LocalObjectReference `json:"sqlServerRef"`

	// AvailabilityGroup contains the AG configuration
	AvailabilityGroup AvailabilityGroupConfig `json:"availabilityGroup"`

	// Listener configures the Kubernetes Service for AG listener access
	// This creates a VIP (ClusterIP) that the operator manages to point to the primary replica
	// +optional
	Listener *AGListenerSpec `json:"listener,omitempty"`

	// Failover contains failover behavior configuration
	// +optional
	Failover *FailoverConfig `json:"failover,omitempty"`

	// Sidecar contains configuration for the AG helper sidecar
	// +optional
	Sidecar *AGSidecarSpec `json:"sidecar,omitempty"`
}

// ListenerPhase represents the current phase of the AG Listener
// +kubebuilder:validation:Enum=Pending;WaitingForListener;Ready;Degraded;Maintenance
type ListenerPhase string

const (
	// ListenerPhasePending indicates the listener Service is being created
	ListenerPhasePending ListenerPhase = "Pending"

	// ListenerPhaseWaitingForListener indicates the VIP Service exists but the operator
	// is waiting for the user to create the AG Listener via T-SQL using the VIP
	ListenerPhaseWaitingForListener ListenerPhase = "WaitingForListener"

	// ListenerPhaseReady indicates the listener is configured and routing to the primary
	ListenerPhaseReady ListenerPhase = "Ready"

	// ListenerPhaseDegraded indicates the listener exists but cannot route (no primary)
	ListenerPhaseDegraded ListenerPhase = "Degraded"

	// ListenerPhaseMaintenance indicates the listener is in maintenance mode
	// (set via annotation mssql.microsoft.com/listener-maintenance=true)
	ListenerPhaseMaintenance ListenerPhase = "Maintenance"
)

// AGListenerSpec defines the configuration for the AG Listener Service
// The operator creates a Kubernetes Service without a selector and manages Endpoints
// to route traffic to the current primary replica
type AGListenerSpec struct {
	// Name is the name of the AG Listener (as configured in SQL Server via T-SQL)
	// This should match the listener name used in CREATE AVAILABILITY GROUP LISTENER
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=63
	// +kubebuilder:validation:Pattern=^[a-z0-9]([-a-z0-9]*[a-z0-9])?$
	Name string `json:"name"`

	// Port is the TCP port the listener accepts connections on
	// This is the port clients use to connect and must match the port in T-SQL listener config
	// Default is 1433, but can be any valid port for security/multi-instance scenarios
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=65535
	// +kubebuilder:default=1433
	Port int32 `json:"port,omitempty"`

	// ServiceType specifies the type of Kubernetes Service to create
	// ClusterIP: Internal cluster access only (default)
	// LoadBalancer: External access via cloud load balancer
	// NodePort: External access via node ports (not recommended for production)
	// +kubebuilder:validation:Enum=ClusterIP;LoadBalancer;NodePort
	// +kubebuilder:default=ClusterIP
	ServiceType corev1.ServiceType `json:"serviceType,omitempty"`

	// ClusterIP allows specifying a static ClusterIP for the listener Service
	// If not specified, Kubernetes assigns one automatically
	// Useful when you need a predictable VIP for DNS or connection strings
	// +optional
	ClusterIP string `json:"clusterIP,omitempty"`

	// LoadBalancerIP allows specifying a static IP for LoadBalancer type services
	// This is cloud-provider specific and may not be supported by all providers
	// +optional
	LoadBalancerIP string `json:"loadBalancerIP,omitempty"`

	// Annotations to add to the listener Service
	// Useful for cloud-provider specific configurations (e.g., internal load balancers)
	// +optional
	Annotations map[string]string `json:"annotations,omitempty"`
}

// AvailabilityGroupConfig defines the AG configuration
type AvailabilityGroupConfig struct {
	// Name is the name of the Availability Group (required)
	// Each SQLServerAG resource monitors exactly one AG
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=128
	Name string `json:"name"`

	// InstanceCount is the number of SQL Server instances participating in the AG (typically 2-5)
	// Note: These are independent instances, not replicas. Each has its own identity and storage.
	// +kubebuilder:validation:Minimum=2
	// +kubebuilder:validation:Maximum=9
	// +kubebuilder:default=3
	InstanceCount int32 `json:"instanceCount,omitempty"`

	// HealthCheckCredentials defines the SQL login credentials for AG health monitoring
	// This is the default for all instances; individual instances can override
	HealthCheckCredentials HealthCheckCredentialsSpec `json:"healthCheckCredentials"`

	// InstanceCredentials allows per-instance credential overrides
	// Key is the instance index (0, 1, 2, etc.)
	// +optional
	InstanceCredentials map[string]HealthCheckCredentialsSpec `json:"instanceCredentials,omitempty"`

	// PrimaryConfig defines the primary instance configuration
	PrimaryConfig InstanceConfig `json:"primaryConfig"`

	// SecondaryConfig defines the secondary instances configuration
	SecondaryConfig InstanceConfig `json:"secondaryConfig"`

	// SeedingMode defines how databases are seeded to secondaries
	// +kubebuilder:validation:Enum=Automatic;Manual
	// +kubebuilder:default=Automatic
	SeedingMode string `json:"seedingMode,omitempty"`

	// Databases is the list of databases to include in the AG
	// +optional
	Databases []AGDatabase `json:"databases,omitempty"`

	// DBFailover enables automatic failover based on database health
	// +kubebuilder:default=true
	DBFailover bool `json:"dbFailover,omitempty"`

	// AutomaticFailover enables controller-driven automatic failover when primary is lost
	// When false (default), the operator only monitors AG health; failover requires manual intervention
	// When true, the controller will detect primary failure and promote a secondary automatically
	// +kubebuilder:default=false
	AutomaticFailover bool `json:"automaticFailover,omitempty"`

	// ClusterType is always External for Kubernetes
	// +kubebuilder:validation:Enum=External
	// +kubebuilder:default=External
	ClusterType string `json:"clusterType,omitempty"`

	// EndpointPort is the port for the database mirroring endpoint
	// +kubebuilder:default=5022
	EndpointPort int32 `json:"endpointPort,omitempty"`

	// ExternalWriteLeaseValidity is the duration for the external write lease (SQL 2019+)
	// +kubebuilder:default="20s"
	ExternalWriteLeaseValidity string `json:"externalWriteLeaseValidity,omitempty"`
}

// InstanceConfig defines configuration for primary or secondary instances in an AG
// Note: SQL Server documentation refers to these as "availability replicas" internally,
// but they are independent SQL Server instances, not identical copies.
type InstanceConfig struct {
	// AvailabilityMode defines the synchronization mode
	// +kubebuilder:validation:Enum=SynchronousCommit;AsynchronousCommit
	// +kubebuilder:default=SynchronousCommit
	AvailabilityMode string `json:"availabilityMode,omitempty"`

	// FailoverMode is always External for Kubernetes-managed AG
	// +kubebuilder:validation:Enum=External
	// +kubebuilder:default=External
	FailoverMode string `json:"failoverMode,omitempty"`

	// ReadableSecondary defines read access to secondary instances
	// +kubebuilder:validation:Enum=No;ReadOnly;All
	// +kubebuilder:default=ReadOnly
	ReadableSecondary string `json:"readableSecondary,omitempty"`

	// SessionTimeout in seconds
	// +kubebuilder:default=10
	SessionTimeout int32 `json:"sessionTimeout,omitempty"`
}

// AGDatabase defines a database to include in the AG
type AGDatabase struct {
	// Name is the database name
	Name string `json:"name"`

	// BackupPath is the path for initial backup (for manual seeding)
	// +optional
	BackupPath string `json:"backupPath,omitempty"`
}

// HealthCheckCredentialsSpec defines credentials for AG Helper health monitoring
// The AG Helper uses these credentials to connect to SQL Server instead of the SA account
// This follows the SQL Server Pacemaker pattern for least-privilege health checking
type HealthCheckCredentialsSpec struct {
	// SecretRef references a Kubernetes secret containing the credentials
	// +optional
	SecretRef *HealthCheckSecretRef `json:"secretRef,omitempty"`

	// Username is the SQL login username (plain text - NOT RECOMMENDED for production)
	// Use secretRef instead for production deployments
	// +optional
	Username string `json:"username,omitempty"`

	// Password is the SQL login password (plain text - NOT RECOMMENDED for production)
	// Use secretRef instead for production deployments
	// WARNING: Plain text passwords in manifests are a security risk
	// +optional
	Password string `json:"password,omitempty"`
}

// HealthCheckSecretRef references secrets containing AG Helper credentials
type HealthCheckSecretRef struct {
	// UsernameSecret references the secret containing the SQL username
	UsernameSecret SecretKeyRef `json:"usernameSecret"`

	// PasswordSecret references the secret containing the SQL password
	PasswordSecret SecretKeyRef `json:"passwordSecret"`
}

// Note: SecretKeyRef is defined in sqlserver_types.go and reused here

// FailoverConfig defines failover behavior
type FailoverConfig struct {
	// Automatic enables automatic failover
	// +kubebuilder:default=true
	Automatic bool `json:"automatic,omitempty"`

	// DataLossThreshold defines acceptable data loss (0 = no data loss)
	// +kubebuilder:default=0
	DataLossThreshold int32 `json:"dataLossThreshold,omitempty"`

	// HealthCheckTimeout is the timeout for health checks
	// +kubebuilder:default="30s"
	HealthCheckTimeout string `json:"healthCheckTimeout,omitempty"`

	// LeaseTimeout is the primary write lease duration
	// +kubebuilder:default="60s"
	LeaseTimeout string `json:"leaseTimeout,omitempty"`

	// RequiredSynchronizedSecondaries is the minimum number of synchronized secondaries
	// required before committing on the primary (-1 for auto-calculate)
	// +kubebuilder:default=-1
	RequiredSynchronizedSecondaries int32 `json:"requiredSynchronizedSecondaries,omitempty"`

	// FailureConditionLevel controls which SQL Server internal health signals trigger
	// failover, modeled after the WSFC failure_condition_level used by SQL Server AGs.
	// When set, the AG Helper sidecar calls sp_server_diagnostics alongside its normal
	// DMV health checks and evaluates the returned component states against this level.
	//
	// Levels (cumulative — each level includes all lower levels):
	//   1 — (Default) AG topology only. The sidecar monitors AG role, sync state, and
	//       instance connectivity using DMV queries. sp_server_diagnostics is NOT called.
	//       This is the baseline behavior and matches how the operator worked before this
	//       field was introduced.
	//   2 — sp_server_diagnostics responsiveness. The sidecar calls sp_server_diagnostics
	//       each monitor cycle. If the procedure fails to respond within the health check
	//       timeout, health is set to Critical. No component-state evaluation is performed.
	//   3 — System component errors. In addition to level 2 checks, the sidecar evaluates
	//       the "system" component from sp_server_diagnostics. If system reports state = 3
	//       (error), health is set to Critical. Detects: spinlock issues, severe access
	//       violations, and out-of-memory conditions.
	//   4 — Resource component errors. In addition to level 3, the sidecar also evaluates
	//       the "resource" component. If resource reports state = 3 (error), health is set
	//       to Critical. Detects: memory pressure, scheduler yields, and excessive I/O latency.
	//   5 — Query processing errors. In addition to level 4, the sidecar also evaluates
	//       the "query_processing" component. If query_processing reports state = 3 (error),
	//       health is set to Critical. Detects: deadlocked schedulers, long-running queries,
	//       and excessive worker thread wait times.
	//
	// IMPORTANT: Levels 2-5 call sp_server_diagnostics over TDS (network). This partially
	// negates the preemptive-thread guarantee that WSFC enjoys (in-process DLL). If SQL
	// Server is completely unresponsive to TDS, the existing staleness threshold serves as
	// the backstop — stale data triggers unhealthy after stalenessThreshold elapses.
	//
	// +optional
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=5
	FailureConditionLevel *int32 `json:"failureConditionLevel,omitempty"`
}

// AGSidecarSpec defines the AG helper sidecar configuration.
// Only image and resources are top-level; operational tuning knobs live
// under the optional "advanced" sub-object so that simple deployments
// can omit them entirely and rely on defaults.
type AGSidecarSpec struct {
	// Image is the AG helper sidecar image
	// +kubebuilder:default="mssql-ag-helper:latest"
	Image string `json:"image,omitempty"`

	// Resources for the sidecar container
	// +optional
	Resources *corev1.ResourceRequirements `json:"resources,omitempty"`

	// Advanced contains optional operational tuning knobs for the AG Helper
	// sidecar. All fields have sensible defaults — most deployments should
	// omit this section entirely.
	// +optional
	Advanced *AGSidecarAdvancedSpec `json:"advanced,omitempty"`
}

// AGSidecarAdvancedSpec contains operational tuning knobs for the AG Helper
// sidecar. All fields have sensible defaults and should only be set when
// the operator needs to be tuned for a specific environment.
type AGSidecarAdvancedSpec struct {
	// MonitorInterval is how often to check AG health
	// +kubebuilder:default="10s"
	MonitorInterval string `json:"monitorInterval,omitempty"`

	// ConnectionTimeout for SQL connections
	// +kubebuilder:default="30s"
	ConnectionTimeout string `json:"connectionTimeout,omitempty"`

	// MaxRetries is the number of reconnect attempts on transient SQL errors
	// before the sidecar declares the connection as Disconnected.
	// +kubebuilder:default=3
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=30
	MaxRetries int `json:"maxRetries,omitempty"`

	// RetryInterval is the delay between SQL reconnect attempts
	// +kubebuilder:default="5s"
	RetryInterval string `json:"retryInterval,omitempty"`

	// StalenessThreshold is how long after the last successful SQL query
	// before the sidecar considers its cached AG state stale. When state is
	// stale the /health and /ready endpoints return 503 and the AG controller
	// treats the instance as degraded for failover evaluation.
	// +kubebuilder:default="30s"
	StalenessThreshold string `json:"stalenessThreshold,omitempty"`
}

// SQLServerAGStatus defines the observed state of SQLServerAG
type SQLServerAGStatus struct {
	// Phase represents the current AG lifecycle phase
	// +kubebuilder:validation:Enum=Pending;Creating;Synchronized;Degraded;Failed
	Phase string `json:"phase,omitempty"`

	// PrimaryReplica is the name of the current primary replica pod
	PrimaryReplica string `json:"primaryReplica,omitempty"`

	// SynchronizedInstances is the count of synchronized instances
	SynchronizedInstances int32 `json:"synchronizedInstances,omitempty"`

	// HealthyDatabases lists databases that are healthy in the AG
	// +optional
	HealthyDatabases []string `json:"healthyDatabases,omitempty"`

	// Instances contains status of each AG instance
	// +optional
	Instances []AGInstanceStatus `json:"instances,omitempty"`

	// Listener contains the status of the AG Listener Service
	// +optional
	Listener *AGListenerStatus `json:"listener,omitempty"`

	// Conditions represent the latest available observations
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// LastFailoverTime is when the last failover occurred
	// +optional
	LastFailoverTime *metav1.Time `json:"lastFailoverTime,omitempty"`

	// ObservedGeneration is the most recent generation observed
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`
}

// AGListenerStatus represents the current state of the AG Listener
type AGListenerStatus struct {
	// Phase represents the current listener lifecycle phase
	Phase ListenerPhase `json:"phase,omitempty"`

	// ServiceName is the name of the Kubernetes Service created for the listener
	ServiceName string `json:"serviceName,omitempty"`

	// VIP is the ClusterIP assigned to the listener Service
	// This is the IP that should be used in T-SQL: CREATE AVAILABILITY GROUP LISTENER
	VIP string `json:"vip,omitempty"`

	// ExternalIP is the external IP when using LoadBalancer service type
	// +optional
	ExternalIP string `json:"externalIP,omitempty"`

	// Port is the listener port
	Port int32 `json:"port,omitempty"`

	// EndpointCount is the number of endpoints in the Endpoints object
	// Should be 1 when listener is Ready (pointing to primary)
	EndpointCount int32 `json:"endpointCount,omitempty"`

	// CurrentPrimary is the pod currently receiving traffic via the listener
	// +optional
	CurrentPrimary string `json:"currentPrimary,omitempty"`

	// LastTransitionTime is when the phase last changed
	// +optional
	LastTransitionTime *metav1.Time `json:"lastTransitionTime,omitempty"`

	// LastCheckedTime is when the listener was last verified
	// +optional
	LastCheckedTime *metav1.Time `json:"lastCheckedTime,omitempty"`

	// Message provides human-readable details about the current phase
	// +optional
	Message string `json:"message,omitempty"`
}

// AGInstanceStatus represents the status of an AG instance
// Note: SQL Server documentation refers to these as "availability replicas", but
// they are independent SQL Server instances with unique identities.
type AGInstanceStatus struct {
	// Name is the instance name (pod name)
	Name string `json:"name"`

	// Role is the current role (PRIMARY, SECONDARY, RESOLVING)
	// +kubebuilder:validation:Enum=PRIMARY;SECONDARY;RESOLVING;CONFIGURATION_ONLY
	Role string `json:"role"`

	// AvailabilityMode of this replica
	AvailabilityMode string `json:"availabilityMode,omitempty"`

	// SynchronizationState (SYNCHRONIZED, SYNCHRONIZING, NOT_SYNCHRONIZING)
	SynchronizationState string `json:"synchronizationState,omitempty"`

	// ConnectedState (CONNECTED, DISCONNECTED)
	ConnectedState string `json:"connectedState,omitempty"`

	// SequenceNumber for determining promotion eligibility
	SequenceNumber int64 `json:"sequenceNumber,omitempty"`

	// IsLocal indicates if this is the local replica
	IsLocal bool `json:"isLocal,omitempty"`

	// Health indicates overall replica health
	// +kubebuilder:validation:Enum=Healthy;Warning;Critical;Unknown
	Health string `json:"health,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:shortName=mssqlag
// +kubebuilder:printcolumn:name="AG-Name",type="string",JSONPath=".spec.availabilityGroup.name"
// +kubebuilder:printcolumn:name="Instances",type="integer",JSONPath=".spec.availabilityGroup.instanceCount"
// +kubebuilder:printcolumn:name="Phase",type="string",JSONPath=".status.phase"
// +kubebuilder:printcolumn:name="Primary",type="string",JSONPath=".status.primaryReplica"
// +kubebuilder:printcolumn:name="Synced",type="integer",JSONPath=".status.synchronizedInstances"
// +kubebuilder:printcolumn:name="Listener",type="string",JSONPath=".status.listener.phase",priority=1
// +kubebuilder:printcolumn:name="VIP",type="string",JSONPath=".status.listener.vip",priority=1
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"

// SQLServerAG is the Schema for the sqlserverags API
type SQLServerAG struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   SQLServerAGSpec   `json:"spec,omitempty"`
	Status SQLServerAGStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// SQLServerAGList contains a list of SQLServerAG
type SQLServerAGList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []SQLServerAG `json:"items"`
}

func init() {
	SchemeBuilder.Register(&SQLServerAG{}, &SQLServerAGList{})
}

// CalculateRequiredSynchronizedSecondaries returns the recommended value
// based on the number of sync-commit instances (ported from mssql-server-ha)
func CalculateRequiredSynchronizedSecondaries(instanceCount int32) int32 {
	// Formula: (n-1)/2 where n is the number of sync-commit instances
	if instanceCount <= 1 {
		return 0
	}
	return (instanceCount - 1) / 2
}
