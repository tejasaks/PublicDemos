/*
Copyright 2026 Microsoft Corporation.

Licensed under the MIT License.
See LICENSE file in the project root for full license information.
*/

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// OperatorConfigurationSpec defines the configuration for the MSSQL Operator
type OperatorConfigurationSpec struct {
	// Images contains container image configuration for all components
	// This is the recommended way to configure custom images for SQL Server,
	// AG Helper, and SQL Exporter across the entire cluster
	// +optional
	Images *ImageConfiguration `json:"images,omitempty"`

	// DockerImage is the default SQL Server image (DEPRECATED: use images.sql2022 instead)
	// +kubebuilder:default="mcr.microsoft.com/mssql/server:2022-latest"
	DockerImage string `json:"dockerImage,omitempty"`

	// SidecarImage is the default AG helper sidecar image (DEPRECATED: use images.agHelper instead)
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

	// Validation contains validation behavior configuration
	// +optional
	Validation *ValidationConfiguration `json:"validation,omitempty"`

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

// ImageConfiguration defines container images for all operator-managed components
// This allows cluster administrators to specify custom images from private registries
// or pin to specific versions across all SQL Server deployments
type ImageConfiguration struct {
	// SQL2019 is the container image for SQL Server 2019
	// +kubebuilder:default="mcr.microsoft.com/mssql/server:2019-latest"
	SQL2019 string `json:"sql2019,omitempty"`

	// SQL2022 is the container image for SQL Server 2022
	// +kubebuilder:default="mcr.microsoft.com/mssql/server:2022-latest"
	SQL2022 string `json:"sql2022,omitempty"`

	// SQL2025 is the container image for SQL Server 2025
	// +kubebuilder:default="mcr.microsoft.com/mssql/server:2025-latest"
	SQL2025 string `json:"sql2025,omitempty"`

	// AGHelper is the container image for the AG Helper sidecar
	// +kubebuilder:default="mssql-ag-helper:latest"
	AGHelper string `json:"agHelper,omitempty"`

	// SQLExporter is the container image for the SQL Exporter (Prometheus metrics)
	// +kubebuilder:default="burningalchemist/sql_exporter:latest"
	SQLExporter string `json:"sqlExporter,omitempty"`

	// ImagePullSecrets is a list of secret names for authenticating to private registries
	// These secrets will be added to all pods created by the operator
	// +optional
	ImagePullSecrets []string `json:"imagePullSecrets,omitempty"`

	// DefaultPullPolicy is the default image pull policy for all containers
	// +kubebuilder:validation:Enum=Always;IfNotPresent;Never
	// +kubebuilder:default="IfNotPresent"
	DefaultPullPolicy string `json:"defaultPullPolicy,omitempty"`
}

// ValidationConfiguration defines validation behavior for the operator
type ValidationConfiguration struct {
	// ClusterCapabilityChecks enables validation of cluster capabilities (StorageClass, etc.)
	// +kubebuilder:default=true
	ClusterCapabilityChecks bool `json:"clusterCapabilityChecks,omitempty"`

	// ValidationTimeout is the maximum time to wait for cluster capability checks
	// +kubebuilder:default="3s"
	ValidationTimeout string `json:"validationTimeout,omitempty"`

	// StorageClassValidation controls how StorageClass validation failures are handled
	// +kubebuilder:validation:Enum=block;warn
	// +kubebuilder:default="block"
	StorageClassValidation string `json:"storageClassValidation,omitempty"`

	// SecretValidation controls how Secret validation failures are handled
	// +kubebuilder:validation:Enum=block;warn
	// +kubebuilder:default="warn"
	SecretValidation string `json:"secretValidation,omitempty"`

	// PasswordComplexity controls password complexity validation
	// +kubebuilder:validation:Enum=enforce;warn
	// +kubebuilder:default="enforce"
	PasswordComplexity string `json:"passwordComplexity,omitempty"`

	// NodeValidation controls node selector validation
	// +kubebuilder:validation:Enum=block;warn
	// +kubebuilder:default="block"
	NodeValidation string `json:"nodeValidation,omitempty"`
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

// GetSQLImage returns the SQL Server image for the specified version
// It checks the ImageConfiguration first, then falls back to DefaultImages
func (c *ImageConfiguration) GetSQLImage(version string) string {
	if c != nil {
		switch version {
		case "2019":
			if c.SQL2019 != "" {
				return c.SQL2019
			}
		case "2022":
			if c.SQL2022 != "" {
				return c.SQL2022
			}
		case "2025":
			if c.SQL2025 != "" {
				return c.SQL2025
			}
		}
	}
	// Fall back to defaults
	if img, ok := DefaultImages[version]; ok {
		return img
	}
	return DefaultImages["2022"] // Ultimate fallback
}

// GetAGHelperImage returns the AG Helper sidecar image
func (c *ImageConfiguration) GetAGHelperImage() string {
	if c != nil && c.AGHelper != "" {
		return c.AGHelper
	}
	return DefaultSidecarImage
}

// GetSQLExporterImage returns the SQL Exporter image
func (c *ImageConfiguration) GetSQLExporterImage() string {
	if c != nil && c.SQLExporter != "" {
		return c.SQLExporter
	}
	return DefaultExporterImage
}

// GetImagePullPolicy returns the default image pull policy
func (c *ImageConfiguration) GetImagePullPolicy() string {
	if c != nil && c.DefaultPullPolicy != "" {
		return c.DefaultPullPolicy
	}
	return "IfNotPresent"
}
