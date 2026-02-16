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

// NOTE: json tags are required. Any new fields you add must have json tags for the fields to be serialized.
// Important: Run "make generate" to regenerate code after modifying this file

// SQLServerSpec defines the desired state of SQLServer
type SQLServerSpec struct {
	// Description is an optional human-readable description for auditing and searchability
	// +optional
	// +kubebuilder:validation:MaxLength=1024
	Description string `json:"description,omitempty"`

	// Version is the SQL Server version key. This value is looked up against the
	// image catalog in OperatorConfiguration.spec.images.catalog. Built-in keys
	// are "2019", "2022", and "2025". Custom keys (e.g., "2025-fts", "2025-ai")
	// can be added to the catalog by the cluster administrator.
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=128
	// +kubebuilder:validation:Pattern=`^[a-zA-Z0-9][a-zA-Z0-9._-]*$`
	// +kubebuilder:default="2022"
	Version string `json:"version,omitempty"`

	// Edition is the SQL Server edition
	// +kubebuilder:validation:Enum=Developer;Express;Standard;Enterprise
	// +kubebuilder:default=Developer
	Edition string `json:"edition,omitempty"`

	// Instance contains instance-specific configuration
	Instance InstanceSpec `json:"instance"`

	// Credentials contains authentication configuration
	Credentials CredentialsSpec `json:"credentials"`

	// ActiveDirectory contains AD authentication configuration (optional)
	// +optional
	ActiveDirectory *ActiveDirectorySpec `json:"activeDirectory,omitempty"`

	// Service defines how the SQL Server instance is exposed
	// +optional
	Service *ServiceSpec `json:"service,omitempty"`

	// Monitoring contains monitoring/observability configuration
	// +optional
	Monitoring *MonitoringSpec `json:"monitoring,omitempty"`

	// Metadata contains labels and annotations to propagate to child resources
	// +optional
	Metadata *ObjectMeta `json:"metadata,omitempty"`

	// Shutdown indicates if the cluster should be shut down
	// +optional
	Shutdown *bool `json:"shutdown,omitempty"`
}

// InstanceSpec defines the SQL Server instance configuration
type InstanceSpec struct {
	// Count is the number of SQL Server instances to deploy (1 for standalone, 2+ for AG)
	// Note: These are independent instances, not identical replicas. For AGs, each instance
	// has its own identity, storage, and can serve different roles (primary/secondary).
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=9
	// +kubebuilder:default=1
	Count int32 `json:"count,omitempty"`

	// Image is the SQL Server container image
	// +optional
	Image string `json:"image,omitempty"`

	// ImagePullPolicy defines when to pull the image
	// +kubebuilder:validation:Enum=Always;Never;IfNotPresent
	// +kubebuilder:default=IfNotPresent
	ImagePullPolicy corev1.PullPolicy `json:"imagePullPolicy,omitempty"`

	// ImagePullSecrets for private registries
	// +optional
	ImagePullSecrets []corev1.LocalObjectReference `json:"imagePullSecrets,omitempty"`

	// Resources defines CPU/memory resources for the SQL Server container
	// +optional
	Resources corev1.ResourceRequirements `json:"resources,omitempty"`

	// Storage defines persistent storage configuration
	Storage StorageSpec `json:"storage"`

	// Config contains SQL Server configuration options (mssql.conf)
	// +optional
	Config *SQLServerConfig `json:"config,omitempty"`

	// SecurityContext for the pod
	// +optional
	SecurityContext *corev1.PodSecurityContext `json:"securityContext,omitempty"`

	// NodeSelector for pod scheduling
	// +optional
	NodeSelector map[string]string `json:"nodeSelector,omitempty"`

	// Tolerations for pod scheduling
	// +optional
	Tolerations []corev1.Toleration `json:"tolerations,omitempty"`

	// Affinity for pod scheduling
	// +optional
	Affinity *corev1.Affinity `json:"affinity,omitempty"`

	// PriorityClassName for the pods
	// +optional
	PriorityClassName string `json:"priorityClassName,omitempty"`
}

// StorageSpec defines storage configuration for SQL Server
type StorageSpec struct {
	// Data volume configuration for user databases
	Data VolumeSpec `json:"data"`

	// Log volume configuration for transaction logs
	// +optional
	Log *VolumeSpec `json:"log,omitempty"`

	// TempDB volume configuration
	// +optional
	TempDB *VolumeSpec `json:"tempdb,omitempty"`

	// Backup volume configuration
	// +optional
	Backup *VolumeSpec `json:"backup,omitempty"`

	// Secrets volume for credentials
	// +optional
	Secrets *VolumeSpec `json:"secrets,omitempty"`
}

// VolumeSpec defines a persistent volume configuration
type VolumeSpec struct {
	// Size is the size of the volume
	// +kubebuilder:validation:Pattern=^[0-9]+[KMGT]i$
	Size string `json:"size"`

	// StorageClass is the name of the StorageClass
	// +optional
	StorageClass *string `json:"storageClass,omitempty"`

	// AccessMode for the PVC
	// +kubebuilder:validation:Enum=ReadWriteOnce;ReadWriteMany;ReadOnlyMany
	// +kubebuilder:default=ReadWriteOnce
	AccessMode corev1.PersistentVolumeAccessMode `json:"accessMode,omitempty"`

	// Selector for selecting specific PVs
	// +optional
	Selector *metav1.LabelSelector `json:"selector,omitempty"`
}

// SQLServerConfig contains SQL Server configuration options
type SQLServerConfig struct {
	// AgentEnabled enables SQL Server Agent
	// +kubebuilder:default=true
	AgentEnabled bool `json:"agentEnabled,omitempty"`

	// HADREnabled enables Always On Availability Groups
	// +kubebuilder:default=true
	HADREnabled bool `json:"hadrEnabled,omitempty"`

	// MemoryLimitMB sets the memory limit for SQL Server
	// +optional
	MemoryLimitMB *int32 `json:"memoryLimitMB,omitempty"`

	// Collation sets the server collation
	// +kubebuilder:default="SQL_Latin1_General_CP1_CI_AS"
	Collation string `json:"collation,omitempty"`

	// LCID sets the language ID
	// +kubebuilder:default=1033
	LCID int32 `json:"lcid,omitempty"`

	// TraceFlags enables specific trace flags
	// +optional
	TraceFlags []int32 `json:"traceFlags,omitempty"`

	// TLSEnabled enables TLS encryption
	// +kubebuilder:default=false
	TLSEnabled bool `json:"tlsEnabled,omitempty"`

	// TLSCertSecretRef references a secret containing TLS certificates
	// +optional
	TLSCertSecretRef *corev1.LocalObjectReference `json:"tlsCertSecretRef,omitempty"`

	// CustomMSSQLConf allows raw mssql.conf content
	// +optional
	CustomMSSQLConf string `json:"customMSSQLConf,omitempty"`
}

// CredentialsSpec defines authentication configuration
type CredentialsSpec struct {
	// SAPasswordSecretRef references a secret containing the SA password
	SAPasswordSecretRef SecretKeyRef `json:"saPasswordSecretRef"`

	// CreateDefaultLogin controls whether to create additional default logins
	// +optional
	CreateDefaultLogin bool `json:"createDefaultLogin,omitempty"`
}

// SecretKeyRef references a key in a Secret
type SecretKeyRef struct {
	// Name is the name of the secret
	Name string `json:"name"`

	// Key is the key within the secret
	// +kubebuilder:default=password
	Key string `json:"key,omitempty"`
}

// ActiveDirectorySpec defines Active Directory authentication configuration
// This enables Windows Authentication via Kerberos for SQL Server on Linux
type ActiveDirectorySpec struct {
	// Enabled indicates whether AD authentication is enabled
	// +kubebuilder:default=false
	Enabled bool `json:"enabled"`

	// Realm is the Kerberos realm (e.g., "CONTOSO.COM")
	// +kubebuilder:validation:Pattern=^[A-Z][A-Z0-9.-]*[A-Z0-9]$
	Realm string `json:"realm"`

	// DomainControllers is the list of domain controller hostnames/IPs
	// +kubebuilder:validation:MinItems=1
	DomainControllers []string `json:"domainControllers"`

	// ServiceAccountSecretRef references a secret containing the AD service account credentials
	// The secret should contain 'username' and 'password' keys
	ServiceAccountSecretRef corev1.LocalObjectReference `json:"serviceAccountSecretRef"`

	// SPNPrefix is the Service Principal Name prefix (default: MSSQLSvc)
	// +kubebuilder:default="MSSQLSvc"
	SPNPrefix string `json:"spnPrefix,omitempty"`

	// KeytabSecretRef references a secret containing the Kerberos keytab file (optional)
	// If not provided, the operator will attempt to create one
	// +optional
	KeytabSecretRef *corev1.LocalObjectReference `json:"keytabSecretRef,omitempty"`

	// ReverseProxyLookup enables reverse DNS lookup
	// +kubebuilder:default=false
	ReverseProxyLookup bool `json:"reverseProxyLookup,omitempty"`

	// PreferredDC is the preferred domain controller
	// +optional
	PreferredDC string `json:"preferredDC,omitempty"`

	// AdminGroup is the AD group that will have sysadmin access
	// +optional
	AdminGroup string `json:"adminGroup,omitempty"`

	// NetBIOSDomain is the NetBIOS domain name (e.g., "CONTOSO")
	// +optional
	NetBIOSDomain string `json:"netBIOSDomain,omitempty"`

	// DNSSuffix is the DNS suffix for the domain
	// +optional
	DNSSuffix string `json:"dnsSuffix,omitempty"`
}

// ServiceSpec defines how the SQL Server instance is exposed
type ServiceSpec struct {
	// Type is the Kubernetes service type
	// +kubebuilder:validation:Enum=ClusterIP;NodePort;LoadBalancer
	// +kubebuilder:default=ClusterIP
	Type corev1.ServiceType `json:"type,omitempty"`

	// Port is the SQL Server port
	// +kubebuilder:default=1433
	Port int32 `json:"port,omitempty"`

	// NodePort for NodePort/LoadBalancer services (optional)
	// +optional
	NodePort *int32 `json:"nodePort,omitempty"`

	// LoadBalancerIP for LoadBalancer services (optional)
	// +optional
	LoadBalancerIP string `json:"loadBalancerIP,omitempty"`

	// Annotations for the service
	// +optional
	Annotations map[string]string `json:"annotations,omitempty"`
}

// MonitoringSpec defines monitoring configuration
type MonitoringSpec struct {
	// Enabled indicates whether monitoring is enabled
	// +kubebuilder:default=true
	Enabled bool `json:"enabled,omitempty"`

	// ExporterImage is the SQL Exporter container image
	// +kubebuilder:default="burningalchemist/sql_exporter:latest"
	ExporterImage string `json:"exporterImage,omitempty"`

	// ExporterPort is the port for the exporter metrics endpoint
	// +kubebuilder:default=9399
	ExporterPort int32 `json:"exporterPort,omitempty"`

	// ExporterResources defines resources for the exporter container
	// +optional
	ExporterResources *corev1.ResourceRequirements `json:"exporterResources,omitempty"`

	// CustomQueries allows defining custom SQL queries for metrics
	// +optional
	CustomQueries string `json:"customQueries,omitempty"`
}

// ObjectMeta contains metadata to propagate to child resources
type ObjectMeta struct {
	// Labels to apply to all child resources
	// +optional
	Labels map[string]string `json:"labels,omitempty"`

	// Annotations to apply to all child resources
	// +optional
	Annotations map[string]string `json:"annotations,omitempty"`
}

// SQLServerStatus defines the observed state of SQLServer
type SQLServerStatus struct {
	// Phase represents the current lifecycle phase
	// +kubebuilder:validation:Enum=Pending;Creating;Running;Failed;Upgrading;Terminating
	Phase string `json:"phase,omitempty"`

	// Ready indicates if the SQL Server instance is ready to accept connections
	Ready bool `json:"ready,omitempty"`

	// CurrentVersion is the current SQL Server version
	CurrentVersion string `json:"currentVersion,omitempty"`

	// CurrentImage is the current container image
	CurrentImage string `json:"currentImage,omitempty"`

	// Instances contains status of each SQL Server instance
	// +optional
	Instances []InstanceStatus `json:"instances,omitempty"`

	// Conditions represent the latest available observations
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// ObservedGeneration is the most recent generation observed
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`
}

// InstanceStatus represents the status of a single SQL Server instance
type InstanceStatus struct {
	// Name is the pod name
	Name string `json:"name"`

	// Role is Primary or Secondary (for AG configurations)
	// +kubebuilder:validation:Enum=Primary;Secondary;Resolving;NotApplicable
	Role string `json:"role,omitempty"`

	// Ready indicates if this instance is ready
	Ready bool `json:"ready"`

	// PodIP is the IP address of the pod
	// +optional
	PodIP string `json:"podIP,omitempty"`

	// Node is the name of the node running this instance
	// +optional
	Node string `json:"node,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:shortName=mssql
// +kubebuilder:printcolumn:name="Version",type="string",JSONPath=".spec.version"
// +kubebuilder:printcolumn:name="Edition",type="string",JSONPath=".spec.edition"
// +kubebuilder:printcolumn:name="Phase",type="string",JSONPath=".status.phase"
// +kubebuilder:printcolumn:name="Ready",type="boolean",JSONPath=".status.ready"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"

// SQLServer is the Schema for the sqlservers API
type SQLServer struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   SQLServerSpec   `json:"spec,omitempty"`
	Status SQLServerStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// SQLServerList contains a list of SQLServer
type SQLServerList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []SQLServer `json:"items"`
}

func init() {
	SchemeBuilder.Register(&SQLServer{}, &SQLServerList{})
}

// GetImage returns the container image to use
// Priority: 1) spec.instance.image (explicit per-SQLServer)
//  2. Hardcoded defaults based on version
//
// Note: Use GetImageWithConfig() to include OperatorConfiguration lookup
func (s *SQLServerSpec) GetImage() string {
	return s.GetImageWithConfig(nil)
}

// GetImageWithConfig returns the container image to use with optional OperatorConfiguration
// Priority order:
//  1. spec.instance.image (explicit per-SQLServer override)
//  2. OperatorConfiguration catalog lookup (catalog[version] → DefaultImages[version])
//  3. Built-in DefaultImages map fallback
func (s *SQLServerSpec) GetImageWithConfig(imageConfig *ImageConfiguration) string {
	// Priority 1: Explicit image in SQLServer spec
	if s.Instance.Image != "" {
		return s.Instance.Image
	}

	// Priority 2: OperatorConfiguration catalog + DefaultImages fallback
	if imageConfig != nil {
		if img := imageConfig.GetSQLImage(s.Version); img != "" {
			return img
		}
	}

	// Priority 3: Built-in DefaultImages map (no OperatorConfiguration present)
	if img, ok := DefaultImages[s.Version]; ok {
		return img
	}

	// Ultimate fallback for unknown versions without catalog entry
	return DefaultImages["2022"]
}

// GetEditionPID returns the MSSQL_PID value for the edition
func (s *SQLServerSpec) GetEditionPID() string {
	switch s.Edition {
	case "Enterprise":
		return "Enterprise"
	case "Standard":
		return "Standard"
	case "Express":
		return "Express"
	default:
		return "Developer"
	}
}
