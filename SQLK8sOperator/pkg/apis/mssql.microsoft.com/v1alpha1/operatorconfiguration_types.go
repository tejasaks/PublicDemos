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

// OperatorConfigurationSpec defines the configuration for the MSSQL Operator
type OperatorConfigurationSpec struct {
	// DockerImage is the default SQL Server image
	// +kubebuilder:default="mcr.microsoft.com/mssql/server:2022-latest"
	DockerImage string `json:"dockerImage,omitempty"`

	// SidecarImage is the default AG helper sidecar image
	// +kubebuilder:default="mssql-ag-helper:latest"
	SidecarImage string `json:"sidecarImage,omitempty"`

	// Workers is the number of concurrent reconciliation workers
	// +kubebuilder:default=8
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=32
	Workers int32 `json:"workers,omitempty"`

	// ResyncPeriod is the interval for full resync of all clusters
	// +kubebuilder:default="30m"
	ResyncPeriod string `json:"resyncPeriod,omitempty"`

	// RepairPeriod is the interval for repair checks
	// +kubebuilder:default="5m"
	RepairPeriod string `json:"repairPeriod,omitempty"`

	// ResourceDefaults contains default resource configurations
	// +optional
	ResourceDefaults *ResourceDefaults `json:"resourceDefaults,omitempty"`

	// Kubernetes contains Kubernetes-specific settings
	// +optional
	Kubernetes *KubernetesConfiguration `json:"kubernetes,omitempty"`

	// Timeouts contains various timeout configurations
	// +optional
	Timeouts *OperatorTimeouts `json:"timeouts,omitempty"`

	// Monitoring contains monitoring defaults
	// +optional
	Monitoring *MonitoringDefaults `json:"monitoring,omitempty"`

	// ActiveDirectory contains AD defaults
	// +optional
	ActiveDirectory *ActiveDirectoryDefaults `json:"activeDirectory,omitempty"`
}

// ResourceDefaults contains default resource settings
type ResourceDefaults struct {
	// DefaultCPURequest is the default CPU request
	// +kubebuilder:default="1"
	DefaultCPURequest string `json:"defaultCPURequest,omitempty"`

	// DefaultMemoryRequest is the default memory request
	// +kubebuilder:default="2Gi"
	DefaultMemoryRequest string `json:"defaultMemoryRequest,omitempty"`

	// DefaultCPULimit is the default CPU limit
	// +kubebuilder:default="4"
	DefaultCPULimit string `json:"defaultCPULimit,omitempty"`

	// DefaultMemoryLimit is the default memory limit
	// +kubebuilder:default="8Gi"
	DefaultMemoryLimit string `json:"defaultMemoryLimit,omitempty"`

	// MinInstances is the minimum allowed instances
	// +kubebuilder:default=1
	MinInstances int32 `json:"minInstances,omitempty"`

	// MaxInstances is the maximum allowed instances
	// +kubebuilder:default=9
	MaxInstances int32 `json:"maxInstances,omitempty"`
}

// KubernetesConfiguration contains K8s-specific settings
type KubernetesConfiguration struct {
	// ClusterNameLabel is the label used to identify clusters
	// +kubebuilder:default="mssql.microsoft.com/cluster"
	ClusterNameLabel string `json:"clusterNameLabel,omitempty"`

	// EnablePodDisruptionBudget enables PDB creation
	// +kubebuilder:default=true
	EnablePodDisruptionBudget bool `json:"enablePodDisruptionBudget,omitempty"`

	// PodManagementPolicy for StatefulSets
	// +kubebuilder:validation:Enum=OrderedReady;Parallel
	// +kubebuilder:default="OrderedReady"
	PodManagementPolicy string `json:"podManagementPolicy,omitempty"`

	// WatchedNamespace limits the operator to a specific namespace (empty = all)
	// +optional
	WatchedNamespace string `json:"watchedNamespace,omitempty"`

	// InheritedLabels are labels from SQLServer CR to propagate to child resources
	// +optional
	InheritedLabels []string `json:"inheritedLabels,omitempty"`

	// InheritedAnnotations are annotations from SQLServer CR to propagate
	// +optional
	InheritedAnnotations []string `json:"inheritedAnnotations,omitempty"`

	// PodServiceAccountName is the service account for SQL Server pods
	// +kubebuilder:default="mssql-sa"
	PodServiceAccountName string `json:"podServiceAccountName,omitempty"`

	// EnableSecretsDeletion allows deletion of secrets when cluster is deleted
	// +kubebuilder:default=true
	EnableSecretsDeletion bool `json:"enableSecretsDeletion,omitempty"`

	// EnablePVCDeletion allows deletion of PVCs when cluster is deleted
	// +kubebuilder:default=false
	EnablePVCDeletion bool `json:"enablePVCDeletion,omitempty"`

	// PodAntiAffinity configures pod anti-affinity for HA
	// +optional
	PodAntiAffinity *PodAntiAffinityConfig `json:"podAntiAffinity,omitempty"`
}

// PodAntiAffinityConfig defines pod anti-affinity settings
type PodAntiAffinityConfig struct {
	// Enabled enables pod anti-affinity
	// +kubebuilder:default=true
	Enabled bool `json:"enabled,omitempty"`

	// TopologyKey for anti-affinity
	// +kubebuilder:default="kubernetes.io/hostname"
	TopologyKey string `json:"topologyKey,omitempty"`

	// PreferredDuringScheduling uses preferred instead of required affinity
	// +kubebuilder:default=false
	PreferredDuringScheduling bool `json:"preferredDuringScheduling,omitempty"`
}

// OperatorTimeouts contains timeout configurations
type OperatorTimeouts struct {
	// ResourceCheckInterval is the interval between resource checks
	// +kubebuilder:default="3s"
	ResourceCheckInterval string `json:"resourceCheckInterval,omitempty"`

	// ResourceCheckTimeout is the timeout for resource checks
	// +kubebuilder:default="10m"
	ResourceCheckTimeout string `json:"resourceCheckTimeout,omitempty"`

	// PodDeletionWaitTimeout is how long to wait for pod deletion
	// +kubebuilder:default="10m"
	PodDeletionWaitTimeout string `json:"podDeletionWaitTimeout,omitempty"`

	// ReadyWaitInterval is the interval for readiness checks
	// +kubebuilder:default="4s"
	ReadyWaitInterval string `json:"readyWaitInterval,omitempty"`

	// ReadyWaitTimeout is the timeout for readiness
	// +kubebuilder:default="30s"
	ReadyWaitTimeout string `json:"readyWaitTimeout,omitempty"`
}

// MonitoringDefaults contains default monitoring settings
type MonitoringDefaults struct {
	// ExporterImage is the default exporter image
	// +kubebuilder:default="burningalchemist/sql_exporter:latest"
	ExporterImage string `json:"exporterImage,omitempty"`

	// EnablePrometheusServiceMonitor creates ServiceMonitor resources
	// +kubebuilder:default=true
	EnablePrometheusServiceMonitor bool `json:"enablePrometheusServiceMonitor,omitempty"`

	// ScrapeInterval for Prometheus
	// +kubebuilder:default="15s"
	ScrapeInterval string `json:"scrapeInterval,omitempty"`
}

// ActiveDirectoryDefaults contains default AD settings
type ActiveDirectoryDefaults struct {
	// DefaultSPNPrefix is the default SPN prefix
	// +kubebuilder:default="MSSQLSvc"
	DefaultSPNPrefix string `json:"defaultSPNPrefix,omitempty"`

	// AdutilImage is the image for adutil operations
	// +optional
	AdutilImage string `json:"adutilImage,omitempty"`
}

// OperatorConfigurationStatus defines the observed state
type OperatorConfigurationStatus struct {
	// CurrentConfiguration shows the active configuration
	// +optional
	CurrentConfiguration string `json:"currentConfiguration,omitempty"`

	// Conditions represent the latest observations
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Cluster,shortName=mssqlconfig
// +kubebuilder:printcolumn:name="Docker-Image",type="string",JSONPath=".spec.dockerImage"
// +kubebuilder:printcolumn:name="Workers",type="integer",JSONPath=".spec.workers"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"

// OperatorConfiguration is the Schema for the operator configuration
type OperatorConfiguration struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   OperatorConfigurationSpec   `json:"spec,omitempty"`
	Status OperatorConfigurationStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// OperatorConfigurationList contains a list of OperatorConfiguration
type OperatorConfigurationList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []OperatorConfiguration `json:"items"`
}

func init() {
	SchemeBuilder.Register(&OperatorConfiguration{}, &OperatorConfigurationList{})
}

// Default images for different SQL Server versions
var DefaultImages = map[string]string{
	"2019": "mcr.microsoft.com/mssql/server:2019-latest",
	"2022": "mcr.microsoft.com/mssql/server:2022-latest",
	"2025": "mcr.microsoft.com/mssql/server:2025-latest",
}

// DefaultExporterImage is the default SQL Exporter image
const DefaultExporterImage = "burningalchemist/sql_exporter:latest"

// DefaultSidecarImage is the default AG helper sidecar image
const DefaultSidecarImage = "mssql-ag-helper:latest"
