/*
Copyright 2026 Microsoft Corporation.

Licensed under the MIT License.
See LICENSE file in the project root for full license information.
*/

package sqlserverag

import (
	"context"
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	mssqlv1alpha1 "github.com/microsoft/mssql-operator/pkg/apis/mssql.microsoft.com/v1alpha1"
)

const (
	// SQLServerAGFinalizer is the finalizer for SQLServerAG resources
	SQLServerAGFinalizer = "mssql.microsoft.com/ag-finalizer"

	// Labels
	LabelAG = "mssql.microsoft.com/ag"
)

// SQLServerAGReconciler reconciles a SQLServerAG object
type SQLServerAGReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	Recorder record.EventRecorder
}

// +kubebuilder:rbac:groups=mssql.microsoft.com,resources=sqlserverags,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=mssql.microsoft.com,resources=sqlserverags/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=mssql.microsoft.com,resources=sqlserverags/finalizers,verbs=update
// +kubebuilder:rbac:groups=mssql.microsoft.com,resources=sqlservers,verbs=get;list;watch
// +kubebuilder:rbac:groups=core,resources=endpoints,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=coordination.k8s.io,resources=leases,verbs=get;list;watch;create;update;patch;delete

// Reconcile handles SQLServerAG reconciliation
func (r *SQLServerAGReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	// Fetch the SQLServerAG instance
	ag := &mssqlv1alpha1.SQLServerAG{}
	if err := r.Get(ctx, req.NamespacedName, ag); err != nil {
		if errors.IsNotFound(err) {
			logger.Info("SQLServerAG resource not found, likely deleted")
			return ctrl.Result{}, nil
		}
		logger.Error(err, "Failed to get SQLServerAG")
		return ctrl.Result{}, err
	}

	// Handle deletion
	if !ag.DeletionTimestamp.IsZero() {
		return r.handleDeletion(ctx, ag)
	}

	// Add finalizer if not present
	if !containsString(ag.Finalizers, SQLServerAGFinalizer) {
		ag.Finalizers = append(ag.Finalizers, SQLServerAGFinalizer)
		if err := r.Update(ctx, ag); err != nil {
			return ctrl.Result{}, err
		}
	}

	// Get referenced SQLServer
	sqlServer := &mssqlv1alpha1.SQLServer{}
	if err := r.Get(ctx, types.NamespacedName{
		Name:      ag.Spec.SQLServerRef.Name,
		Namespace: ag.Namespace,
	}, sqlServer); err != nil {
		logger.Error(err, "Failed to get referenced SQLServer")
		r.Recorder.Event(ag, corev1.EventTypeWarning, "SQLServerNotFound",
			fmt.Sprintf("Referenced SQLServer %s not found", ag.Spec.SQLServerRef.Name))
		return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
	}

	// Check if SQLServer is ready
	if !sqlServer.Status.Ready {
		logger.Info("Waiting for SQLServer to be ready")
		return ctrl.Result{RequeueAfter: 10 * time.Second}, nil
	}

	// Update status phase if needed
	if ag.Status.Phase == "" || ag.Status.Phase == "Pending" {
		if err := r.updatePhase(ctx, ag, "Creating"); err != nil {
			return ctrl.Result{}, err
		}
	}

	// Reconcile AG endpoints
	if err := r.reconcileEndpoints(ctx, ag, sqlServer); err != nil {
		r.Recorder.Event(ag, corev1.EventTypeWarning, "EndpointsFailed", err.Error())
		return ctrl.Result{}, err
	}

	// Update AG status from sidecars
	if err := r.updateAGStatus(ctx, ag, sqlServer); err != nil {
		logger.Error(err, "Failed to update AG status")
		// Don't return error, continue with requeue
	}

	// Requeue to monitor AG health
	monitorInterval := 10 * time.Second
	if ag.Spec.Sidecar != nil && ag.Spec.Sidecar.MonitorInterval != "" {
		if d, err := time.ParseDuration(ag.Spec.Sidecar.MonitorInterval); err == nil {
			monitorInterval = d
		}
	}

	return ctrl.Result{RequeueAfter: monitorInterval}, nil
}

// reconcileEndpoints creates or updates services for AG primary/secondary routing
func (r *SQLServerAGReconciler) reconcileEndpoints(ctx context.Context, ag *mssqlv1alpha1.SQLServerAG, sqlServer *mssqlv1alpha1.SQLServer) error {
	logger := log.FromContext(ctx)

	if ag.Spec.Endpoints == nil {
		return nil
	}

	// Primary endpoint service
	if ag.Spec.Endpoints.Primary != nil {
		primarySvc := &corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name:        fmt.Sprintf("%s-primary", ag.Spec.AvailabilityGroup.Name),
				Namespace:   ag.Namespace,
				Labels:      r.labelsForAG(ag, sqlServer),
				Annotations: ag.Spec.Endpoints.Primary.Annotations,
			},
			Spec: corev1.ServiceSpec{
				Type: ag.Spec.Endpoints.Primary.Type,
				Selector: map[string]string{
					"app":                      "mssql",
					"mssql.microsoft.com/instance": sqlServer.Name,
					"mssql.microsoft.com/role":     "primary",
				},
				Ports: []corev1.ServicePort{
					{
						Name:       "sql",
						Port:       ag.Spec.Endpoints.Primary.Port,
						TargetPort: intstr.FromInt(1433),
					},
				},
			},
		}

		if err := ctrl.SetControllerReference(ag, primarySvc, r.Scheme); err != nil {
			return err
		}

		found := &corev1.Service{}
		err := r.Get(ctx, types.NamespacedName{Name: primarySvc.Name, Namespace: primarySvc.Namespace}, found)
		if err != nil && errors.IsNotFound(err) {
			logger.Info("Creating primary endpoint service", "name", primarySvc.Name)
			if err := r.Create(ctx, primarySvc); err != nil {
				return err
			}
		} else if err != nil {
			return err
		}
	}

	// Secondary endpoint service (for read-only routing)
	if ag.Spec.Endpoints.Secondary != nil {
		secondarySvc := &corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name:        fmt.Sprintf("%s-secondary", ag.Spec.AvailabilityGroup.Name),
				Namespace:   ag.Namespace,
				Labels:      r.labelsForAG(ag, sqlServer),
				Annotations: ag.Spec.Endpoints.Secondary.Annotations,
			},
			Spec: corev1.ServiceSpec{
				Type: ag.Spec.Endpoints.Secondary.Type,
				Selector: map[string]string{
					"app":                      "mssql",
					"mssql.microsoft.com/instance": sqlServer.Name,
					"mssql.microsoft.com/role":     "secondary",
				},
				Ports: []corev1.ServicePort{
					{
						Name:       "sql",
						Port:       ag.Spec.Endpoints.Secondary.Port,
						TargetPort: intstr.FromInt(1433),
					},
				},
			},
		}

		if err := ctrl.SetControllerReference(ag, secondarySvc, r.Scheme); err != nil {
			return err
		}

		found := &corev1.Service{}
		err := r.Get(ctx, types.NamespacedName{Name: secondarySvc.Name, Namespace: secondarySvc.Namespace}, found)
		if err != nil && errors.IsNotFound(err) {
			logger.Info("Creating secondary endpoint service", "name", secondarySvc.Name)
			if err := r.Create(ctx, secondarySvc); err != nil {
				return err
			}
		} else if err != nil {
			return err
		}
	}

	return nil
}

// updateAGStatus queries sidecars for AG health and updates status
func (r *SQLServerAGReconciler) updateAGStatus(ctx context.Context, ag *mssqlv1alpha1.SQLServerAG, sqlServer *mssqlv1alpha1.SQLServer) error {
	logger := log.FromContext(ctx)

	// Get pods for the SQLServer
	podList := &corev1.PodList{}
	listOpts := []client.ListOption{
		client.InNamespace(ag.Namespace),
		client.MatchingLabels(map[string]string{
			"app":                      "mssql",
			"mssql.microsoft.com/instance": sqlServer.Name,
		}),
	}
	if err := r.List(ctx, podList, listOpts...); err != nil {
		return err
	}

	// Build replica status
	replicas := make([]mssqlv1alpha1.AGReplicaStatus, 0, len(podList.Items))
	var primaryReplica string
	synchronizedCount := int32(0)

	for _, pod := range podList.Items {
		// In a real implementation, we would query the sidecar API
		// For now, we'll derive role from pod labels or annotations
		role := pod.Labels["mssql.microsoft.com/role"]
		if role == "" {
			role = "SECONDARY" // Default
		}
		if role == "primary" || role == "PRIMARY" {
			primaryReplica = pod.Name
		}

		syncState := "SYNCHRONIZING"
		if role == "PRIMARY" || role == "primary" {
			syncState = "SYNCHRONIZED"
		}

		// Check if pod is ready
		ready := false
		for _, cond := range pod.Status.Conditions {
			if cond.Type == corev1.PodReady && cond.Status == corev1.ConditionTrue {
				ready = true
				break
			}
		}

		if syncState == "SYNCHRONIZED" && ready {
			synchronizedCount++
		}

		replicas = append(replicas, mssqlv1alpha1.AGReplicaStatus{
			Name:                 pod.Name,
			Role:                 role,
			SynchronizationState: syncState,
			ConnectedState:       "CONNECTED",
			Health:               "Healthy",
			IsLocal:              false,
		})
	}

	// Determine phase
	phase := "Degraded"
	if synchronizedCount >= 1 && primaryReplica != "" {
		phase = "Synchronized"
	}
	if synchronizedCount == ag.Spec.AvailabilityGroup.Replicas && primaryReplica != "" {
		phase = "Synchronized"
	}

	// Update status
	ag.Status.Phase = phase
	ag.Status.PrimaryReplica = primaryReplica
	ag.Status.SynchronizedReplicas = synchronizedCount
	ag.Status.Replicas = replicas
	ag.Status.ObservedGeneration = ag.Generation

	// Set Ready condition
	condition := metav1.Condition{
		Type:               "Ready",
		Status:             metav1.ConditionFalse,
		Reason:             "NotReady",
		Message:            "Availability Group is not synchronized",
		LastTransitionTime: metav1.Now(),
	}
	if phase == "Synchronized" {
		condition.Status = metav1.ConditionTrue
		condition.Reason = "Synchronized"
		condition.Message = fmt.Sprintf("AG is synchronized with %d replicas", synchronizedCount)
	}
	meta.SetStatusCondition(&ag.Status.Conditions, condition)

	logger.Info("Updating AG status", "phase", phase, "primary", primaryReplica, "synchronized", synchronizedCount)
	return r.Status().Update(ctx, ag)
}

// updatePhase updates just the phase
func (r *SQLServerAGReconciler) updatePhase(ctx context.Context, ag *mssqlv1alpha1.SQLServerAG, phase string) error {
	ag.Status.Phase = phase
	return r.Status().Update(ctx, ag)
}

// handleDeletion handles cleanup when SQLServerAG is deleted
func (r *SQLServerAGReconciler) handleDeletion(ctx context.Context, ag *mssqlv1alpha1.SQLServerAG) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	if containsString(ag.Finalizers, SQLServerAGFinalizer) {
		logger.Info("Running finalizer for SQLServerAG", "name", ag.Name)
		r.Recorder.Event(ag, corev1.EventTypeNormal, "Deleting", "Running cleanup finalizer")

		// Perform cleanup here if needed
		// For example, remove AG from SQL Server instances

		// Remove finalizer
		ag.Finalizers = removeString(ag.Finalizers, SQLServerAGFinalizer)
		if err := r.Update(ctx, ag); err != nil {
			return ctrl.Result{}, err
		}
	}

	return ctrl.Result{}, nil
}

// labelsForAG returns labels for resources created by this AG
func (r *SQLServerAGReconciler) labelsForAG(ag *mssqlv1alpha1.SQLServerAG, sqlServer *mssqlv1alpha1.SQLServer) map[string]string {
	return map[string]string{
		"app":                      "mssql",
		"mssql.microsoft.com/instance": sqlServer.Name,
		LabelAG:                    ag.Spec.AvailabilityGroup.Name,
	}
}

// SetupWithManager sets up the controller with the Manager
func (r *SQLServerAGReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&mssqlv1alpha1.SQLServerAG{}).
		Owns(&corev1.Service{}).
		Complete(r)
}

// Helper functions
func containsString(slice []string, s string) bool {
	for _, item := range slice {
		if item == s {
			return true
		}
	}
	return false
}

func removeString(slice []string, s string) []string {
	result := make([]string, 0, len(slice))
	for _, item := range slice {
		if item != s {
			result = append(result, item)
		}
	}
	return result
}
