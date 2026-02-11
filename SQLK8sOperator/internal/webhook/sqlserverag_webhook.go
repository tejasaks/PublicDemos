/*
Copyright 2026 Microsoft Corporation.

Licensed under the MIT License.
See LICENSE file in the project root for full license information.
*/

package webhook

import (
	"context"
	"net/http"
	"strings"

	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	"github.com/microsoft/mssql-operator/internal/validation"
	mssqlv1alpha1 "github.com/microsoft/mssql-operator/pkg/apis/mssql.microsoft.com/v1alpha1"
)

// SQLServerAGValidator validates SQLServerAG resources
type SQLServerAGValidator struct {
	Client  client.Client
	decoder *admission.Decoder
	config  *validation.ValidationConfig
}

// NewSQLServerAGValidator creates a new SQLServerAGValidator
func NewSQLServerAGValidator(c client.Client, config *validation.ValidationConfig) *SQLServerAGValidator {
	if config == nil {
		config = validation.DefaultValidationConfig()
	}
	return &SQLServerAGValidator{
		Client: c,
		config: config,
	}
}

// Handle implements admission.Handler
func (v *SQLServerAGValidator) Handle(ctx context.Context, req admission.Request) admission.Response {
	ag := &mssqlv1alpha1.SQLServerAG{}

	err := v.decoder.Decode(req, ag)
	if err != nil {
		return admission.Errored(http.StatusBadRequest, err)
	}

	klog.V(4).Infof("Validating SQLServerAG %s/%s", ag.Namespace, ag.Name)

	// Perform validation
	result := v.validate(ctx, ag)

	// Log warnings
	for _, warning := range result.Warnings {
		klog.Warningf("SQLServerAG %s/%s: %s", ag.Namespace, ag.Name, warning)
	}

	if !result.Valid {
		errorMsg := strings.Join(result.Errors, "; ")
		klog.Errorf("SQLServerAG %s/%s validation failed: %s", ag.Namespace, ag.Name, errorMsg)
		return admission.Denied(errorMsg)
	}

	// Return allowed with warnings if any
	if len(result.Warnings) > 0 {
		return admission.Allowed("").WithWarnings(result.Warnings...)
	}

	return admission.Allowed("")
}

// validate performs all validation checks
func (v *SQLServerAGValidator) validate(ctx context.Context, ag *mssqlv1alpha1.SQLServerAG) *validation.ValidationResult {
	result := validation.NewValidationResult()

	// 1. Validate resource name (NetBIOS limit)
	nameResult := validation.ValidateSQLServerName(ag.Name)
	result.Merge(nameResult)

	// 2. Validate Availability Group name (SQL identifier)
	agNameResult := validation.ValidateAGName(ag.Spec.AvailabilityGroup.Name)
	result.Merge(agNameResult)

	// 3. Validate database names
	for _, db := range ag.Spec.AvailabilityGroup.Databases {
		dbResult := validation.ValidateDatabaseName(db.Name)
		result.Merge(dbResult)

		// Validate backup path if specified
		if db.BackupPath != "" {
			pathResult := validation.ValidatePath(db.BackupPath, "backupPath")
			result.Merge(pathResult)
		}
	}

	// 4. Validate instance count
	instanceCount := ag.Spec.AvailabilityGroup.InstanceCount
	if instanceCount < 2 {
		result.AddError("Availability Group requires at least 2 instances (has %d)", instanceCount)
	}
	if instanceCount > 9 {
		result.AddError("Availability Group supports maximum 9 instances (has %d)", instanceCount)
	}

	// 5. Validate endpoint port
	if ag.Spec.AvailabilityGroup.EndpointPort > 0 {
		portResult := validation.ValidatePort(ag.Spec.AvailabilityGroup.EndpointPort, "AG endpoint")
		result.Merge(portResult)
	}

	// 6. Validate SQLServer reference
	if ag.Spec.SQLServerRef.Name == "" {
		result.AddError("sqlServerRef.name is required")
	} else {
		refResult := validation.ValidateResourceName(ag.Spec.SQLServerRef.Name, 253)
		result.Merge(refResult)
	}

	// 7. Validate sidecar image if specified
	if ag.Spec.Sidecar != nil && ag.Spec.Sidecar.Image != "" {
		imageResult := validation.ValidateImageReference(ag.Spec.Sidecar.Image)
		result.Merge(imageResult)
	}

	// 8. Validate HealthCheckCredentials
	credResult := v.validateHealthCheckCredentials(ctx, ag)
	result.Merge(credResult)

	// 9. Validate Listener configuration if specified
	if ag.Spec.Listener != nil {
		listenerResult := v.validateListener(ctx, ag)
		result.Merge(listenerResult)
	}

	return result
}

// validateHealthCheckCredentials validates the AG Helper credential configuration
func (v *SQLServerAGValidator) validateHealthCheckCredentials(ctx context.Context, ag *mssqlv1alpha1.SQLServerAG) *validation.ValidationResult {
	result := validation.NewValidationResult()

	creds := ag.Spec.AvailabilityGroup.HealthCheckCredentials

	// Check if any credentials are specified
	hasSecretRef := creds.SecretRef != nil
	hasPlainText := creds.Username != "" || creds.Password != ""

	// Cross-field validation: must use one method or the other, not both
	if hasSecretRef && hasPlainText {
		result.AddError("healthCheckCredentials: specify either secretRef OR plain text username/password, not both")
	}

	// If neither is specified, that's OK - will fall back to SA credentials with warning
	if !hasSecretRef && !hasPlainText {
		result.AddWarning("healthCheckCredentials: no credentials specified, AG Helper will fall back to SA account. " +
			"For production, create a dedicated SQL login with VIEW SERVER STATE and ALTER ANY AVAILABILITY GROUP permissions")
	}

	// Validate secretRef if provided
	if hasSecretRef {
		// Validate username secret
		if creds.SecretRef.UsernameSecret.Name == "" {
			result.AddError("healthCheckCredentials.secretRef.usernameSecret.name is required")
		} else {
			nameResult := validation.ValidateResourceName(creds.SecretRef.UsernameSecret.Name, 253)
			for _, err := range nameResult.Errors {
				result.AddError("healthCheckCredentials.secretRef.usernameSecret.name: %s", err)
			}

			// Check secret exists
			validator := validation.NewValidator(v.Client, v.config)
			existsResult := validator.ValidateSecretExists(ctx, creds.SecretRef.UsernameSecret.Name, ag.Namespace)
			for _, err := range existsResult.Errors {
				result.AddError("healthCheckCredentials.secretRef.usernameSecret: %s", err)
			}
			for _, warn := range existsResult.Warnings {
				result.AddWarning("healthCheckCredentials.secretRef.usernameSecret: %s", warn)
			}
		}

		if creds.SecretRef.UsernameSecret.Key == "" {
			result.AddError("healthCheckCredentials.secretRef.usernameSecret.key is required")
		}

		// Validate password secret
		if creds.SecretRef.PasswordSecret.Name == "" {
			result.AddError("healthCheckCredentials.secretRef.passwordSecret.name is required")
		} else {
			nameResult := validation.ValidateResourceName(creds.SecretRef.PasswordSecret.Name, 253)
			for _, err := range nameResult.Errors {
				result.AddError("healthCheckCredentials.secretRef.passwordSecret.name: %s", err)
			}

			// Check secret exists (if different from username secret)
			if creds.SecretRef.PasswordSecret.Name != creds.SecretRef.UsernameSecret.Name {
				validator := validation.NewValidator(v.Client, v.config)
				existsResult := validator.ValidateSecretExists(ctx, creds.SecretRef.PasswordSecret.Name, ag.Namespace)
				for _, err := range existsResult.Errors {
					result.AddError("healthCheckCredentials.secretRef.passwordSecret: %s", err)
				}
				for _, warn := range existsResult.Warnings {
					result.AddWarning("healthCheckCredentials.secretRef.passwordSecret: %s", warn)
				}
			}
		}

		if creds.SecretRef.PasswordSecret.Key == "" {
			result.AddError("healthCheckCredentials.secretRef.passwordSecret.key is required")
		}
	}

	// Validate plain text credentials if provided
	if hasPlainText {
		// Strong warning about plain text
		result.AddWarning("healthCheckCredentials: using plain text credentials is NOT RECOMMENDED for production. " +
			"Consider using secretRef instead to avoid exposing credentials in manifests")

		// Validate username (SQL identifier)
		if creds.Username != "" {
			usernameResult := validation.ValidateSQLIdentifier(creds.Username, "healthCheckCredentials.username")
			result.Merge(usernameResult)

			// Check for SQL injection patterns
			sqlResult := validation.DetectSQLInjection(creds.Username, "healthCheckCredentials.username")
			result.Merge(sqlResult)
		} else {
			result.AddError("healthCheckCredentials.username is required when using plain text credentials")
		}

		// Validate password complexity
		if creds.Password != "" {
			passResult := validation.ValidatePasswordComplexity(creds.Password)
			if !passResult.Valid {
				result.AddWarning("healthCheckCredentials.password: %s", passResult.Message)
			}
		} else {
			result.AddError("healthCheckCredentials.password is required when using plain text credentials")
		}
	}

	// Validate instanceCredentials if provided
	for instanceIndex, instanceCreds := range ag.Spec.AvailabilityGroup.InstanceCredentials {
		prefix := "instanceCredentials[" + instanceIndex + "]"

		instanceHasSecretRef := instanceCreds.SecretRef != nil
		instanceHasPlainText := instanceCreds.Username != "" || instanceCreds.Password != ""

		if instanceHasSecretRef && instanceHasPlainText {
			result.AddError("%s: specify either secretRef OR plain text, not both", prefix)
		}

		if instanceHasSecretRef {
			if instanceCreds.SecretRef.UsernameSecret.Name == "" {
				result.AddError("%s.secretRef.usernameSecret.name is required", prefix)
			}
			if instanceCreds.SecretRef.UsernameSecret.Key == "" {
				result.AddError("%s.secretRef.usernameSecret.key is required", prefix)
			}
			if instanceCreds.SecretRef.PasswordSecret.Name == "" {
				result.AddError("%s.secretRef.passwordSecret.name is required", prefix)
			}
			if instanceCreds.SecretRef.PasswordSecret.Key == "" {
				result.AddError("%s.secretRef.passwordSecret.key is required", prefix)
			}
		}

		if instanceHasPlainText {
			result.AddWarning("%s: using plain text credentials is NOT RECOMMENDED", prefix)

			if instanceCreds.Username != "" {
				usernameResult := validation.ValidateSQLIdentifier(instanceCreds.Username, prefix+".username")
				result.Merge(usernameResult)
			}
		}
	}

	return result
}

// validateListener validates the AG Listener configuration
func (v *SQLServerAGValidator) validateListener(ctx context.Context, ag *mssqlv1alpha1.SQLServerAG) *validation.ValidationResult {
	result := validation.NewValidationResult()

	listener := ag.Spec.Listener

	// Validate listener name (must be DNS-compatible for K8s Service)
	if listener.Name == "" {
		result.AddError("listener.name is required")
	} else {
		// K8s Service name validation: lowercase, alphanumeric and hyphens, max 63 chars
		if len(listener.Name) > 63 {
			result.AddError("listener.name exceeds maximum length of 63 characters (has %d)", len(listener.Name))
		}

		// Check for valid DNS subdomain format (lowercase, alphanumeric, hyphens)
		nameResult := validation.ValidateKubernetesName(listener.Name, "listener.name")
		result.Merge(nameResult)
	}

	// Validate listener port
	if listener.Port != 0 {
		portResult := validation.ValidatePort(listener.Port, "listener.port")
		result.Merge(portResult)

		// Warn if using well-known ports other than 1433
		if listener.Port < 1024 && listener.Port != 1433 {
			result.AddWarning("listener.port %d is a well-known port; consider using port >= 1024 for non-root containers", listener.Port)
		}
	}

	// Validate service type
	if listener.ServiceType != "" {
		switch listener.ServiceType {
		case "ClusterIP", "LoadBalancer", "NodePort":
			// Valid types
		default:
			result.AddError("listener.serviceType must be one of: ClusterIP, LoadBalancer, NodePort (got %s)", listener.ServiceType)
		}

		// NodePort warning
		if listener.ServiceType == "NodePort" {
			result.AddWarning("listener.serviceType NodePort exposes the service on each node's IP; " +
				"consider LoadBalancer or ClusterIP with Ingress for production")
		}
	}

	// Validate clusterIP if specified (must be valid IP format)
	if listener.ClusterIP != "" {
		if listener.ClusterIP != "None" {
			ipResult := validation.ValidateIPAddress(listener.ClusterIP, "listener.clusterIP")
			result.Merge(ipResult)
		}
	}

	// Validate loadBalancerIP if specified
	if listener.LoadBalancerIP != "" {
		if listener.ServiceType != "" && listener.ServiceType != "LoadBalancer" {
			result.AddError("listener.loadBalancerIP can only be set when serviceType is LoadBalancer")
		}
		ipResult := validation.ValidateIPAddress(listener.LoadBalancerIP, "listener.loadBalancerIP")
		result.Merge(ipResult)
	}

	// Validate annotations if specified
	for key := range listener.Annotations {
		// Check for reserved annotations
		if strings.HasPrefix(key, "mssql.microsoft.com/") {
			result.AddWarning("listener.annotations: %s uses reserved prefix 'mssql.microsoft.com/'", key)
		}
	}

	return result
}

// InjectDecoder injects the decoder
func (v *SQLServerAGValidator) InjectDecoder(d *admission.Decoder) error {
	v.decoder = d
	return nil
}
