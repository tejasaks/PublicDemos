/*
Copyright 2026 Microsoft Corporation.

Licensed under the MIT License.
See LICENSE file in the project root for full license information.
*/

package webhook

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	"github.com/microsoft/mssql-operator/internal/validation"
	mssqlv1alpha1 "github.com/microsoft/mssql-operator/pkg/apis/mssql.microsoft.com/v1alpha1"
)

// SQLServerValidator validates SQLServer resources
type SQLServerValidator struct {
	Client  client.Client
	decoder *admission.Decoder
	config  *validation.ValidationConfig
}

// NewSQLServerValidator creates a new SQLServerValidator
func NewSQLServerValidator(c client.Client, config *validation.ValidationConfig) *SQLServerValidator {
	if config == nil {
		config = validation.DefaultValidationConfig()
	}
	return &SQLServerValidator{
		Client: c,
		config: config,
	}
}

// Handle implements admission.Handler
func (v *SQLServerValidator) Handle(ctx context.Context, req admission.Request) admission.Response {
	sqlserver := &mssqlv1alpha1.SQLServer{}

	err := v.decoder.Decode(req, sqlserver)
	if err != nil {
		return admission.Errored(http.StatusBadRequest, err)
	}

	klog.V(4).Infof("Validating SQLServer %s/%s", sqlserver.Namespace, sqlserver.Name)

	// Perform validation
	result := v.validate(ctx, sqlserver)

	// Log warnings
	for _, warning := range result.Warnings {
		klog.Warningf("SQLServer %s/%s: %s", sqlserver.Namespace, sqlserver.Name, warning)
	}

	if !result.Valid {
		errorMsg := strings.Join(result.Errors, "; ")
		klog.Errorf("SQLServer %s/%s validation failed: %s", sqlserver.Namespace, sqlserver.Name, errorMsg)
		return admission.Denied(errorMsg)
	}

	// Return allowed with warnings if any
	if len(result.Warnings) > 0 {
		return admission.Allowed("").WithWarnings(result.Warnings...)
	}

	return admission.Allowed("")
}

// validate performs all validation checks
func (v *SQLServerValidator) validate(ctx context.Context, sqlserver *mssqlv1alpha1.SQLServer) *validation.ValidationResult {
	result := validation.NewValidationResult()
	validator := validation.NewValidator(v.Client, v.config)

	// 1. Validate resource name (NetBIOS limit)
	nameResult := validation.ValidateSQLServerName(sqlserver.Name)
	result.Merge(nameResult)

	// 2. Validate storage classes if specified
	// Check data volume storage class (required)
	if sqlserver.Spec.Instance.Storage.Data.StorageClass != nil && *sqlserver.Spec.Instance.Storage.Data.StorageClass != "" {
		scResult := validator.ValidateStorageClass(ctx, *sqlserver.Spec.Instance.Storage.Data.StorageClass)
		result.Merge(scResult)
	}
	// Check log volume storage class (optional)
	if sqlserver.Spec.Instance.Storage.Log != nil && sqlserver.Spec.Instance.Storage.Log.StorageClass != nil && *sqlserver.Spec.Instance.Storage.Log.StorageClass != "" {
		scResult := validator.ValidateStorageClass(ctx, *sqlserver.Spec.Instance.Storage.Log.StorageClass)
		result.Merge(scResult)
	}
	// Check tempdb volume storage class (optional)
	if sqlserver.Spec.Instance.Storage.TempDB != nil && sqlserver.Spec.Instance.Storage.TempDB.StorageClass != nil && *sqlserver.Spec.Instance.Storage.TempDB.StorageClass != "" {
		scResult := validator.ValidateStorageClass(ctx, *sqlserver.Spec.Instance.Storage.TempDB.StorageClass)
		result.Merge(scResult)
	}
	// Check backup volume storage class (optional)
	if sqlserver.Spec.Instance.Storage.Backup != nil && sqlserver.Spec.Instance.Storage.Backup.StorageClass != nil && *sqlserver.Spec.Instance.Storage.Backup.StorageClass != "" {
		scResult := validator.ValidateStorageClass(ctx, *sqlserver.Spec.Instance.Storage.Backup.StorageClass)
		result.Merge(scResult)
	}

	// 3. Validate SA password secret reference
	if sqlserver.Spec.Credentials.SAPasswordSecretRef.Name != "" {
		// Validate secret name format
		secretNameResult := validation.ValidateResourceName(sqlserver.Spec.Credentials.SAPasswordSecretRef.Name, 253)
		result.Merge(secretNameResult)

		// Check if secret exists (warn mode per config)
		secretResult := v.validateSecretAndPassword(ctx, sqlserver)
		result.Merge(secretResult)
	} else {
		result.AddError("saPasswordSecretRef.name is required")
	}

	// 4. Validate container image if specified
	if sqlserver.Spec.Instance.Image != "" {
		imageResult := validation.ValidateImageReference(sqlserver.Spec.Instance.Image)
		result.Merge(imageResult)
	}

	// 5. Validate version resolves to an image (catalog lookup + defaults + instance.image)
	// Skip if spec.instance.image is set (it overrides the catalog entirely)
	if sqlserver.Spec.Instance.Image == "" {
		versionResult := v.validateVersionResolvesToImage(ctx, sqlserver)
		result.Merge(versionResult)
	}

	// 5. Validate memory is sufficient for SQL Server
	if sqlserver.Spec.Instance.Resources.Limits.Memory() != nil {
		memResult := validation.ValidateMemoryForSQLServer(sqlserver.Spec.Instance.Resources.Limits.Memory().String())
		result.Merge(memResult)
	}

	// 6. Validate ports
	if sqlserver.Spec.Service != nil && sqlserver.Spec.Service.Port > 0 {
		portResult := validation.ValidatePort(sqlserver.Spec.Service.Port, "SQL Server")
		result.Merge(portResult)
	}

	// 7. Validate labels and annotations
	if sqlserver.Spec.Metadata != nil {
		for k, val := range sqlserver.Spec.Metadata.Labels {
			labelResult := validation.ValidateLabelValue(val)
			if !labelResult.Valid {
				for _, err := range labelResult.Errors {
					result.AddError("Label '%s': %s", k, err)
				}
			}
		}
	}

	return result
}

// validateSecretAndPassword checks if the secret exists and optionally validates password complexity
func (v *SQLServerValidator) validateSecretAndPassword(ctx context.Context, sqlserver *mssqlv1alpha1.SQLServer) *validation.ValidationResult {
	result := validation.NewValidationResult()

	secretName := sqlserver.Spec.Credentials.SAPasswordSecretRef.Name
	secretKey := sqlserver.Spec.Credentials.SAPasswordSecretRef.Key
	if secretKey == "" {
		secretKey = "password"
	}

	// Check if secret exists
	secret := &corev1.Secret{}
	err := v.Client.Get(ctx, types.NamespacedName{
		Name:      secretName,
		Namespace: sqlserver.Namespace,
	}, secret)

	if err != nil {
		if errors.IsNotFound(err) {
			msg := fmt.Sprintf("Secret '%s' not found in namespace '%s'. "+
				"Create it with: kubectl create secret generic %s --from-literal=%s=<password> -n %s",
				secretName, sqlserver.Namespace, secretName, secretKey, sqlserver.Namespace)

			if v.config.SecretValidation == "block" {
				result.AddError("%s", msg)
			} else {
				result.AddWarning("%s", msg)
			}
			return result
		}
		// Other error - just warn
		result.AddWarning("Could not verify secret '%s': %v", secretName, err)
		return result
	}

	// Secret exists - validate password complexity if we can read it
	if v.config.PasswordComplexity == "enforce" {
		passwordBytes, exists := secret.Data[secretKey]
		if !exists {
			result.AddError("Secret '%s' does not contain key '%s'", secretName, secretKey)
			return result
		}

		password := string(passwordBytes)
		pwResult := validation.ValidatePasswordComplexity(password)
		if !pwResult.Valid {
			result.AddError("%s", pwResult.Message)
		}
	}

	return result
}

// validateVersionResolvesToImage checks that spec.version resolves to a container image
// via the catalog (OperatorConfiguration) or the built-in DefaultImages map.
func (v *SQLServerValidator) validateVersionResolvesToImage(ctx context.Context, sqlserver *mssqlv1alpha1.SQLServer) *validation.ValidationResult {
	result := validation.NewValidationResult()
	version := sqlserver.Spec.Version

	// Check built-in defaults first (fast path)
	if _, ok := mssqlv1alpha1.DefaultImages[version]; ok {
		return result
	}

	// Load OperatorConfiguration to check catalog
	config := &mssqlv1alpha1.OperatorConfiguration{}
	err := v.Client.Get(ctx, types.NamespacedName{Name: "default"}, config)
	if err != nil {
		if errors.IsNotFound(err) {
			// No OperatorConfiguration — version must be in defaults
			available := (&mssqlv1alpha1.ImageConfiguration{}).GetAvailableVersions()
			result.AddError("version %q is not a built-in version and no OperatorConfiguration exists. "+
				"Available built-in versions: %s. "+
				"To use custom versions, create an OperatorConfiguration with an image catalog, "+
				"or set spec.instance.image directly",
				version, strings.Join(available, ", "))
			return result
		}
		// Transient error — warn but allow
		result.AddWarning("Could not load OperatorConfiguration to validate version %q: %v", version, err)
		return result
	}

	// Check catalog
	imageConfig := config.Spec.Images
	if imageConfig != nil {
		if img := imageConfig.GetSQLImage(version); img != "" {
			return result // Found in catalog or defaults
		}
	}

	// Version not found anywhere
	available := imageConfig.GetAvailableVersions()
	result.AddError("version %q does not resolve to any container image. "+
		"Available versions: %s. "+
		"Add it to OperatorConfiguration.spec.images.catalog or set spec.instance.image directly",
		version, strings.Join(available, ", "))

	return result
}

// InjectDecoder injects the decoder
func (v *SQLServerValidator) InjectDecoder(d *admission.Decoder) error {
	v.decoder = d
	return nil
}
