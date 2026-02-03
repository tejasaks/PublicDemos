/*
Copyright 2026 Microsoft Corporation.

Licensed under the MIT License.
See LICENSE file in the project root for full license information.
*/

// Package validation provides validation functions for SQL Server Kubernetes Operator resources.
// It includes cluster capability checks, security validations, and input sanitization.
package validation

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"

	storagev1 "k8s.io/api/storage/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// ValidationConfig holds configuration for validation behavior
type ValidationConfig struct {
	// ClusterCapabilityChecks enables/disables cluster capability validation
	ClusterCapabilityChecks bool `json:"clusterCapabilityChecks"`

	// ValidationTimeout is the maximum time to wait for cluster checks
	ValidationTimeout time.Duration `json:"validationTimeout"`

	// StorageClassValidation: "block" or "warn"
	StorageClassValidation string `json:"storageClassValidation"`

	// SecretValidation: "block" or "warn"
	SecretValidation string `json:"secretValidation"`

	// PasswordComplexity: "enforce" or "warn"
	PasswordComplexity string `json:"passwordComplexity"`

	// NodeValidation: "block" or "warn"
	NodeValidation string `json:"nodeValidation"`
}

// DefaultValidationConfig returns the default validation configuration
func DefaultValidationConfig() *ValidationConfig {
	return &ValidationConfig{
		ClusterCapabilityChecks: true,
		ValidationTimeout:       3 * time.Second,
		StorageClassValidation:  "block",
		SecretValidation:        "warn",
		PasswordComplexity:      "enforce",
		NodeValidation:          "block",
	}
}

// ValidationResult represents the result of a validation check
type ValidationResult struct {
	Valid    bool
	Errors   []string
	Warnings []string
}

// NewValidationResult creates a new validation result
func NewValidationResult() *ValidationResult {
	return &ValidationResult{
		Valid:    true,
		Errors:   []string{},
		Warnings: []string{},
	}
}

// AddError adds an error to the result
func (r *ValidationResult) AddError(format string, args ...interface{}) {
	r.Valid = false
	r.Errors = append(r.Errors, fmt.Sprintf(format, args...))
}

// AddWarning adds a warning to the result
func (r *ValidationResult) AddWarning(format string, args ...interface{}) {
	r.Warnings = append(r.Warnings, fmt.Sprintf(format, args...))
}

// Merge combines another result into this one
func (r *ValidationResult) Merge(other *ValidationResult) {
	if !other.Valid {
		r.Valid = false
	}
	r.Errors = append(r.Errors, other.Errors...)
	r.Warnings = append(r.Warnings, other.Warnings...)
}

// Validator performs validation checks
type Validator struct {
	client client.Client
	config *ValidationConfig
}

// NewValidator creates a new validator
func NewValidator(c client.Client, config *ValidationConfig) *Validator {
	if config == nil {
		config = DefaultValidationConfig()
	}
	return &Validator{
		client: c,
		config: config,
	}
}

// ValidateStorageClass checks if a StorageClass exists in the cluster
func (v *Validator) ValidateStorageClass(ctx context.Context, name string) *ValidationResult {
	result := NewValidationResult()

	if name == "" {
		// Empty means use cluster default - always valid
		return result
	}

	if !v.config.ClusterCapabilityChecks {
		return result
	}

	// Apply timeout
	ctx, cancel := context.WithTimeout(ctx, v.config.ValidationTimeout)
	defer cancel()

	sc := &storagev1.StorageClass{}
	err := v.client.Get(ctx, types.NamespacedName{Name: name}, sc)

	if err != nil {
		if errors.IsNotFound(err) {
			// StorageClass not found - get available ones for helpful error
			availableSCs := v.listAvailableStorageClasses(ctx)
			msg := fmt.Sprintf("StorageClass '%s' not found in cluster. Available StorageClasses: %v. "+
				"Update spec.instance.storage.storageClass or remove to use cluster default.",
				name, availableSCs)

			if v.config.StorageClassValidation == "block" {
				result.AddError("%s", msg)
			} else {
				result.AddWarning("%s", msg)
			}
		} else if ctx.Err() == context.DeadlineExceeded {
			// Timeout - allow with warning
			klog.Warningf("StorageClass validation timed out after %v, allowing creation with warning", v.config.ValidationTimeout)
			result.AddWarning("StorageClass validation timed out, could not verify '%s' exists", name)
		} else {
			// Other error - allow with warning
			klog.Warningf("StorageClass validation failed: %v, allowing creation with warning", err)
			result.AddWarning("Could not validate StorageClass '%s': %v", name, err)
		}
	}

	return result
}

// listAvailableStorageClasses returns a list of available StorageClass names
func (v *Validator) listAvailableStorageClasses(ctx context.Context) []string {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	scList := &storagev1.StorageClassList{}
	if err := v.client.List(ctx, scList); err != nil {
		return []string{"(unable to list)"}
	}

	names := make([]string, 0, len(scList.Items))
	for _, sc := range scList.Items {
		names = append(names, sc.Name)
	}

	if len(names) == 0 {
		return []string{"(none available)"}
	}
	return names
}

// ValidateSecretExists checks if a Secret exists in the specified namespace
func (v *Validator) ValidateSecretExists(ctx context.Context, name, namespace string) *ValidationResult {
	result := NewValidationResult()

	if name == "" {
		result.AddError("Secret name is required")
		return result
	}

	if !v.config.ClusterCapabilityChecks {
		return result
	}

	// Apply timeout
	ctx, cancel := context.WithTimeout(ctx, v.config.ValidationTimeout)
	defer cancel()

	secret := &struct {
		metav1TypeMeta   `json:",inline"`
		metav1ObjectMeta `json:"metadata,omitempty"`
	}{}

	// Use unstructured to avoid importing corev1 just for this check
	err := v.client.Get(ctx, types.NamespacedName{Name: name, Namespace: namespace}, nil)

	if err != nil {
		if errors.IsNotFound(err) {
			msg := fmt.Sprintf("Secret '%s' not found in namespace '%s'. "+
				"Create it with: kubectl create secret generic %s --from-literal=password=<password> -n %s",
				name, namespace, name, namespace)

			if v.config.SecretValidation == "block" {
				result.AddError("%s", msg)
			} else {
				result.AddWarning("%s", msg)
				klog.Warningf("Secret '%s' not found in namespace '%s', proceeding with warning", name, namespace)
			}
		} else if ctx.Err() == context.DeadlineExceeded {
			klog.Warningf("Secret validation timed out after %v, allowing creation with warning", v.config.ValidationTimeout)
			result.AddWarning("Secret validation timed out, could not verify '%s' exists", name)
		}
		// For other errors, we don't block - the controller will handle it
	}

	// Suppress unused variable warning
	_ = secret

	return result
}

// metav1TypeMeta and metav1ObjectMeta are minimal types for unstructured access
type metav1TypeMeta struct {
	Kind       string `json:"kind,omitempty"`
	APIVersion string `json:"apiVersion,omitempty"`
}

type metav1ObjectMeta struct {
	Name      string `json:"name,omitempty"`
	Namespace string `json:"namespace,omitempty"`
}

// ValidateResourceName validates a Kubernetes resource name
func ValidateResourceName(name string, maxLength int) *ValidationResult {
	result := NewValidationResult()

	if name == "" {
		result.AddError("Name is required")
		return result
	}

	if len(name) > maxLength {
		result.AddError("Name '%s' exceeds maximum length of %d characters (has %d)",
			name, maxLength, len(name))
	}

	// Kubernetes naming pattern
	pattern := regexp.MustCompile(`^[a-z0-9]([-a-z0-9]*[a-z0-9])?$`)
	if !pattern.MatchString(name) {
		result.AddError("Name '%s' is invalid. Must consist of lowercase alphanumeric characters or '-', "+
			"start with an alphanumeric character, and end with an alphanumeric character", name)
	}

	return result
}

// ValidateSQLServerName validates a SQL Server resource name (NetBIOS limit)
func ValidateSQLServerName(name string) *ValidationResult {
	// SQL Server NetBIOS limit is 15 chars, minus 2 for pod suffix (-0, -1, etc.)
	return ValidateResourceName(name, 13)
}

// ValidateLabelValue validates a Kubernetes label value
func ValidateLabelValue(value string) *ValidationResult {
	result := NewValidationResult()

	if len(value) > 63 {
		result.AddError("Label value '%s' exceeds maximum length of 63 characters", truncate(value, 20))
	}

	if value == "" {
		return result // Empty is valid for labels
	}

	// Label value pattern (if non-empty)
	pattern := regexp.MustCompile(`^[a-zA-Z0-9]([-_.a-zA-Z0-9]*[a-zA-Z0-9])?$`)
	if !pattern.MatchString(value) {
		result.AddError("Label value '%s' is invalid. Must consist of alphanumeric characters, '-', '_', or '.', "+
			"and start/end with an alphanumeric character", truncate(value, 20))
	}

	return result
}

// ValidateAnnotationValue validates a Kubernetes annotation value
func ValidateAnnotationValue(value string, maxLength int) *ValidationResult {
	result := NewValidationResult()

	if maxLength <= 0 {
		maxLength = 262144 // 256KB default K8s limit
	}

	if len(value) > maxLength {
		result.AddError("Annotation value exceeds maximum length of %d characters (has %d)",
			maxLength, len(value))
	}

	return result
}

// ValidateDescription validates the description field
func ValidateDescription(description string) *ValidationResult {
	result := NewValidationResult()

	if len(description) > 1024 {
		result.AddError("Description exceeds maximum length of 1024 characters (has %d)", len(description))
	}

	return result
}

// truncate shortens a string for display
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

// ValidatePort validates a port number
func ValidatePort(port int32, fieldName string) *ValidationResult {
	result := NewValidationResult()

	if port < 1 || port > 65535 {
		result.AddError("%s port %d is invalid. Must be between 1 and 65535", fieldName, port)
	}

	// Warn about privileged ports
	if port < 1024 {
		result.AddWarning("%s port %d is a privileged port. Consider using a port >= 1024", fieldName, port)
	}

	return result
}

// ValidateMemoryForSQLServer validates memory is sufficient for SQL Server
func ValidateMemoryForSQLServer(memoryString string) *ValidationResult {
	result := NewValidationResult()

	if memoryString == "" {
		result.AddWarning("No memory limit specified. SQL Server requires minimum 2GB RAM")
		return result
	}

	// Parse memory string (e.g., "2Gi", "4096Mi")
	memoryBytes := parseMemoryString(memoryString)

	minMemory := int64(2 * 1024 * 1024 * 1024) // 2GB in bytes
	if memoryBytes < minMemory {
		result.AddError("Memory limit '%s' is below SQL Server minimum requirement of 2Gi", memoryString)
	}

	return result
}

// parseMemoryString parses a Kubernetes memory string to bytes
func parseMemoryString(s string) int64 {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0
	}

	var multiplier int64 = 1
	var numStr string

	if strings.HasSuffix(s, "Ki") {
		multiplier = 1024
		numStr = strings.TrimSuffix(s, "Ki")
	} else if strings.HasSuffix(s, "Mi") {
		multiplier = 1024 * 1024
		numStr = strings.TrimSuffix(s, "Mi")
	} else if strings.HasSuffix(s, "Gi") {
		multiplier = 1024 * 1024 * 1024
		numStr = strings.TrimSuffix(s, "Gi")
	} else if strings.HasSuffix(s, "Ti") {
		multiplier = 1024 * 1024 * 1024 * 1024
		numStr = strings.TrimSuffix(s, "Ti")
	} else if strings.HasSuffix(s, "K") {
		multiplier = 1000
		numStr = strings.TrimSuffix(s, "K")
	} else if strings.HasSuffix(s, "M") {
		multiplier = 1000 * 1000
		numStr = strings.TrimSuffix(s, "M")
	} else if strings.HasSuffix(s, "G") {
		multiplier = 1000 * 1000 * 1000
		numStr = strings.TrimSuffix(s, "G")
	} else {
		numStr = s
	}

	var num int64
	fmt.Sscanf(numStr, "%d", &num)
	return num * multiplier
}

// ValidateImageReference validates a container image reference
func ValidateImageReference(image string) *ValidationResult {
	result := NewValidationResult()

	if image == "" {
		result.AddError("Image reference is required")
		return result
	}

	// Check for spaces (common mistake)
	if strings.Contains(image, " ") {
		result.AddError("Image reference '%s' contains spaces, which is invalid", image)
	}

	// Check for shell metacharacters that could indicate injection
	dangerousChars := []string{";", "|", "&", "$", "`", "(", ")", "{", "}", "<", ">", "\\", "\n", "\r"}
	for _, char := range dangerousChars {
		if strings.Contains(image, char) {
			result.AddError("Image reference '%s' contains invalid character '%s'", truncate(image, 30), char)
		}
	}

	// Basic format check: should have at least registry/image or image:tag format
	// Allow: nginx, nginx:latest, registry.io/image, registry.io/org/image:tag
	validPattern := regexp.MustCompile(`^[a-zA-Z0-9][-a-zA-Z0-9_./:@]*[a-zA-Z0-9]$`)
	if !validPattern.MatchString(image) {
		result.AddError("Image reference '%s' has invalid format", truncate(image, 30))
	}

	return result
}

// ValidateKubernetesName validates a name for use as a Kubernetes resource name
// Kubernetes names must be DNS-1123 subdomain: lowercase, alphanumeric, and hyphens
// Must start and end with alphanumeric character, max 63 characters
func ValidateKubernetesName(name string, fieldName string) *ValidationResult {
	result := NewValidationResult()

	if name == "" {
		result.AddError("%s is required", fieldName)
		return result
	}

	if len(name) > 63 {
		result.AddError("%s '%s' exceeds maximum length of 63 characters (has %d)", fieldName, truncate(name, 20), len(name))
	}

	// DNS-1123 label: lowercase alphanumeric and hyphens
	// Must start with alphanumeric, must end with alphanumeric
	validPattern := regexp.MustCompile(`^[a-z0-9]([-a-z0-9]*[a-z0-9])?$`)
	if !validPattern.MatchString(name) {
		result.AddError("%s '%s' is invalid: must be lowercase, alphanumeric, may contain hyphens, "+
			"and must start and end with an alphanumeric character", fieldName, truncate(name, 20))
	}

	// Check for uppercase (common mistake)
	if strings.ToLower(name) != name {
		result.AddError("%s '%s' contains uppercase characters; Kubernetes names must be lowercase", fieldName, truncate(name, 20))
	}

	return result
}

// ValidateIPAddress validates an IPv4 or IPv6 address
func ValidateIPAddress(ip string, fieldName string) *ValidationResult {
	result := NewValidationResult()

	if ip == "" {
		result.AddError("%s is required", fieldName)
		return result
	}

	// IPv4 pattern
	ipv4Pattern := regexp.MustCompile(`^(\d{1,3}\.){3}\d{1,3}$`)
	// IPv6 pattern (simplified - handles common formats)
	ipv6Pattern := regexp.MustCompile(`^([0-9a-fA-F]{0,4}:){2,7}[0-9a-fA-F]{0,4}$|^::$|^::1$`)

	if ipv4Pattern.MatchString(ip) {
		// Validate IPv4 octets are 0-255
		parts := strings.Split(ip, ".")
		for _, part := range parts {
			var num int
			fmt.Sscanf(part, "%d", &num)
			if num < 0 || num > 255 {
				result.AddError("%s '%s' has invalid IPv4 octet value (must be 0-255)", fieldName, ip)
				break
			}
		}
	} else if ipv6Pattern.MatchString(ip) {
		// Basic IPv6 format is valid
	} else {
		result.AddError("%s '%s' is not a valid IPv4 or IPv6 address", fieldName, ip)
	}

	return result
}
