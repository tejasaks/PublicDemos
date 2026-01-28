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
	// SQLServerRef references the SQLServer instance to configure as an AG
	SQLServerRef corev1.LocalObjectReference `json:"sqlServerRef"`

	// AvailabilityGroup contains the AG configuration
	AvailabilityGroup AvailabilityGroupConfig `json:"availabilityGroup"`

	// Failover contains failover behavior configuration
	// +optional
	Failover *FailoverConfig `json:"failover,omitempty"`

	// Endpoints defines service endpoints for the AG
	// +optional
	Endpoints *AGEndpointsSpec `json:"endpoints,omitempty"`

	// Sidecar contains configuration for the AG helper sidecar
	// +optional
	Sidecar *AGSidecarSpec `json:"sidecar,omitempty"`
}

// AvailabilityGroupConfig defines the AG configuration
type AvailabilityGroupConfig struct {
	// Name is the name of the Availability Group
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=128
	Name string `json:"name"`

	// Replicas is the number of AG replicas (typically 2-5)
	// +kubebuilder:validation:Minimum=2
	// +kubebuilder:validation:Maximum=9
	// +kubebuilder:default=3
	Replicas int32 `json:"replicas,omitempty"`

	// PrimaryConfig defines the primary replica configuration
	PrimaryConfig ReplicaConfig `json:"primaryConfig"`

	// SecondaryConfig defines the secondary replicas configuration
	SecondaryConfig ReplicaConfig `json:"secondaryConfig"`

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
	// When enabled, the controller will detect primary failure and promote a secondary
	// +kubebuilder:default=true
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

// ReplicaConfig defines configuration for primary or secondary replicas
type ReplicaConfig struct {
	// AvailabilityMode defines the synchronization mode
	// +kubebuilder:validation:Enum=SynchronousCommit;AsynchronousCommit
	// +kubebuilder:default=SynchronousCommit
	AvailabilityMode string `json:"availabilityMode,omitempty"`

	// FailoverMode is always External for Kubernetes-managed AG
	// +kubebuilder:validation:Enum=External
	// +kubebuilder:default=External
	FailoverMode string `json:"failoverMode,omitempty"`

	// ReadableSecondary defines read access to secondary replicas
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
}

// AGEndpointsSpec defines service endpoints for the AG
type AGEndpointsSpec struct {
	// Primary endpoint configuration
	Primary *AGServiceSpec `json:"primary,omitempty"`

	// Secondary endpoint configuration (for read-only routing)
	Secondary *AGServiceSpec `json:"secondary,omitempty"`
}

// AGServiceSpec defines a service for AG endpoints
type AGServiceSpec struct {
	// Type is the Kubernetes service type
	// +kubebuilder:validation:Enum=ClusterIP;NodePort;LoadBalancer
	// +kubebuilder:default=ClusterIP
	Type corev1.ServiceType `json:"type,omitempty"`

	// Port is the SQL Server port
	// +kubebuilder:default=1433
	Port int32 `json:"port,omitempty"`

	// Annotations for the service
	// +optional
	Annotations map[string]string `json:"annotations,omitempty"`
}

// AGSidecarSpec defines the AG helper sidecar configuration
type AGSidecarSpec struct {
	// Image is the AG helper sidecar image
	// +kubebuilder:default="mssql-ag-helper:latest"
	Image string `json:"image,omitempty"`

	// Resources for the sidecar container
	// +optional
	Resources *corev1.ResourceRequirements `json:"resources,omitempty"`

	// MonitorInterval is how often to check AG health
	// +kubebuilder:default="10s"
	MonitorInterval string `json:"monitorInterval,omitempty"`

	// ConnectionTimeout for SQL connections
	// +kubebuilder:default="30s"
	ConnectionTimeout string `json:"connectionTimeout,omitempty"`
}

// SQLServerAGStatus defines the observed state of SQLServerAG
type SQLServerAGStatus struct {
	// Phase represents the current AG lifecycle phase
	// +kubebuilder:validation:Enum=Pending;Creating;Synchronized;Degraded;Failed
	Phase string `json:"phase,omitempty"`

	// PrimaryReplica is the name of the current primary replica pod
	PrimaryReplica string `json:"primaryReplica,omitempty"`

	// SynchronizedReplicas is the count of synchronized replicas
	SynchronizedReplicas int32 `json:"synchronizedReplicas,omitempty"`

	// HealthyDatabases lists databases that are healthy in the AG
	// +optional
	HealthyDatabases []string `json:"healthyDatabases,omitempty"`

	// Replicas contains status of each AG replica
	// +optional
	Replicas []AGReplicaStatus `json:"replicas,omitempty"`

	// Conditions represent the latest available observations
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// LastFailoverTime is when the last failover occurred
	// +optional
	LastFailoverTime *metav1.Time `json:"lastFailoverTime,omitempty"`

	// ObservedGeneration is the most recent generation observed
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`
}

// AGReplicaStatus represents the status of an AG replica
type AGReplicaStatus struct {
	// Name is the replica name (pod name)
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
// +kubebuilder:printcolumn:name="Replicas",type="integer",JSONPath=".spec.availabilityGroup.replicas"
// +kubebuilder:printcolumn:name="Phase",type="string",JSONPath=".status.phase"
// +kubebuilder:printcolumn:name="Primary",type="string",JSONPath=".status.primaryReplica"
// +kubebuilder:printcolumn:name="Synced",type="integer",JSONPath=".status.synchronizedReplicas"
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
// based on the number of sync-commit replicas (ported from mssql-server-ha)
func CalculateRequiredSynchronizedSecondaries(numReplicas int32) int32 {
	// Formula: (n-1)/2 where n is the number of sync-commit replicas
	if numReplicas <= 1 {
		return 0
	}
	return (numReplicas - 1) / 2
}
