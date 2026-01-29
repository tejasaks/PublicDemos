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

	mssqlv1alpha1 "github.com/microsoft/mssql-operator/pkg/apis/mssql.microsoft.com/v1alpha1"
	"github.com/microsoft/mssql-operator/internal/validation"
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

	// 4. Validate replica count
	replicas := ag.Spec.AvailabilityGroup.Replicas
	if replicas < 2 {
		result.AddError("Availability Group requires at least 2 replicas (has %d)", replicas)
	}
	if replicas > 9 {
		result.AddError("Availability Group supports maximum 9 replicas (has %d)", replicas)
	}

	// 5. Validate endpoint port
	if ag.Spec.AvailabilityGroup.EndpointPort > 0 {
		portResult := validation.ValidatePort(ag.Spec.AvailabilityGroup.EndpointPort, "AG endpoint")
		result.Merge(portResult)
	}

	// 6. Validate service endpoints
	if ag.Spec.Endpoints != nil {
		if ag.Spec.Endpoints.Primary != nil && ag.Spec.Endpoints.Primary.Port > 0 {
			portResult := validation.ValidatePort(ag.Spec.Endpoints.Primary.Port, "Primary endpoint")
			result.Merge(portResult)
		}
		if ag.Spec.Endpoints.Secondary != nil && ag.Spec.Endpoints.Secondary.Port > 0 {
			portResult := validation.ValidatePort(ag.Spec.Endpoints.Secondary.Port, "Secondary endpoint")
			result.Merge(portResult)
		}
	}

	// 7. Validate SQLServer reference
	if ag.Spec.SQLServerRef.Name == "" {
		result.AddError("sqlServerRef.name is required")
	} else {
		refResult := validation.ValidateResourceName(ag.Spec.SQLServerRef.Name, 253)
		result.Merge(refResult)
	}

	// 8. Validate sidecar image if specified
	if ag.Spec.Sidecar != nil && ag.Spec.Sidecar.Image != "" {
		imageResult := validation.ValidateImageReference(ag.Spec.Sidecar.Image)
		result.Merge(imageResult)
	}

	// 9. Validate labels and annotations in endpoints
	if ag.Spec.Endpoints != nil {
		if ag.Spec.Endpoints.Primary != nil {
			for k, val := range ag.Spec.Endpoints.Primary.Annotations {
				annotResult := validation.ValidateAnnotationValue(val, 0)
				if !annotResult.Valid {
					for _, err := range annotResult.Errors {
						result.AddError("Primary endpoint annotation '%s': %s", k, err)
					}
				}
			}
		}
	}

	return result
}

// InjectDecoder injects the decoder
func (v *SQLServerAGValidator) InjectDecoder(d *admission.Decoder) error {
	v.decoder = d
	return nil
}
