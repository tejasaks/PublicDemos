/*
Copyright 2026 Microsoft Corporation.

Licensed under the MIT License.
See LICENSE file in the project root for full license information.
*/

package v1alpha1

import (
	"sort"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// OperatorConfigurationSpec defines the configuration for the MSSQL Operator
type OperatorConfigurationSpec struct {
	// Images contains container image configuration for all components
	// This is the recommended way to configure custom images for SQL Server,
	// AG Helper, and SQL Exporter across the entire cluster
	// +optional
	Images *ImageConfiguration `json:"images,omitempty"`

	// DockerImage is the default SQL Server image (DEPRECATED: use images.catalog instead)
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
// This allows cluster administrators to specify custom images from private registries,
// pin to specific versions, or register unlimited custom SQL Server images (e.g.,
// SQL2025+FTS, SQL2025+AI, SQL2022+Polybase) via the catalog map.
type ImageConfiguration struct {
	// Catalog is a map of version keys to SQL Server container image references.
	// The map key is the version string that users specify in SQLServer.spec.version,
	// and the value is the full container image reference.
	//
	// Built-in defaults ("2019", "2022", "2025") are provided automatically and do not
	// need to be listed unless you want to override them with custom images.
	//
	// Example:
	//   catalog:
	//     "2022": "mcr.microsoft.com/mssql/server:2022-CU16-ubuntu-22.04"
	//     "2025": "mcr.microsoft.com/mssql/server:2025-latest"
	//     "2025-fts": "myregistry.azurecr.io/mssql/server:2025-fts"
	//     "2025-ai": "myregistry.azurecr.io/mssql-ai:2025-ollama"
	//     "2022-polybase": "myregistry.azurecr.io/mssql/server:2022-polybase"
	// +optional
	Catalog map[string]string `json:"catalog,omitempty"`

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
const DefaultSidecarImage = "ghcr.io/tejasaks/mssql-ag-helper:v1.0.0"

// GetSQLImage returns the SQL Server image for the specified version key.
// It checks the catalog map first, then falls back to DefaultImages.
func (c *ImageConfiguration) GetSQLImage(version string) string {
	if c != nil && c.Catalog != nil {
		if img, ok := c.Catalog[version]; ok && img != "" {
			return img
		}
	}
	// Fall back to built-in defaults
	if img, ok := DefaultImages[version]; ok {
		return img
	}
	return "" // Unknown version — let caller decide
}

// GetAvailableVersions returns a sorted list of all version keys available
// (from catalog + built-in defaults). Used for user-facing error messages.
func (c *ImageConfiguration) GetAvailableVersions() []string {
	seen := make(map[string]bool)
	for k := range DefaultImages {
		seen[k] = true
	}
	if c != nil && c.Catalog != nil {
		for k := range c.Catalog {
			seen[k] = true
		}
	}
	versions := make([]string, 0, len(seen))
	for k := range seen {
		versions = append(versions, k)
	}
	// Sort for deterministic output
	sort.Strings(versions)
	return versions
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
