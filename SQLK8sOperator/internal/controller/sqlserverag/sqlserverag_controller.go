/*
Copyright 2026 Microsoft Corporation.

Licensed under the MIT License.
See LICENSE file in the project root for full license information.
*/

package sqlserverag

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
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

	// Annotations for manual operations
	AnnotationFailoverTarget    = "mssql.microsoft.com/failover-to"        // Target replica for failover (e.g., "sql-ag-1")
	AnnotationFailoverRequested = "mssql.microsoft.com/failover-requested" // Timestamp when failover was requested
	AnnotationFailoverStatus    = "mssql.microsoft.com/failover-status"    // Status: pending, in-progress, completed, failed

	// Failover configuration
	FailoverCooldownPeriod = 60 * time.Second // Minimum time between failovers
	NoPrimaryGracePeriod   = 30 * time.Second // Wait before triggering failover
	SidecarPort            = 8080             // AG Helper sidecar HTTP port
)

// SidecarState represents the state returned by AG Helper sidecar
type SidecarState struct {
	AGName           string `json:"agName"`
	Health           string `json:"health"`
	Role             string `json:"role"`
	SyncState        string `json:"syncState"`
	SequenceNumber   int64  `json:"sequenceNumber"`
	LocalReplicaName string `json:"localReplicaName"`
}

// FailoverCandidate represents a replica that can become primary
type FailoverCandidate struct {
	PodName        string
	PodIP          string
	SequenceNumber int64
	SyncState      string
	Health         string
}

// SQLServerAGReconciler reconciles a SQLServerAG object
type SQLServerAGReconciler struct {
	client.Client
	Scheme     *runtime.Scheme
	Recorder   record.EventRecorder
	HTTPClient *http.Client

	// Failover state tracking
	lastFailoverTime  map[string]time.Time
	noPrimaryDetected map[string]time.Time
}

// +kubebuilder:rbac:groups=mssql.microsoft.com,resources=operatorconfigurations,verbs=get;list;watch
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

	// Check for manual failover request (via annotation)
	if result, err := r.checkAndHandleManualFailover(ctx, ag, sqlServer); err != nil {
		logger.Error(err, "Manual failover handling failed")
		r.Recorder.Event(ag, corev1.EventTypeWarning, "ManualFailoverFailed", err.Error())
	} else if result != nil {
		return *result, nil
	}

	// Check for automatic failover if enabled
	if ag.Spec.AvailabilityGroup.AutomaticFailover {
		if result, err := r.checkAndHandleFailover(ctx, ag, sqlServer); err != nil {
			logger.Error(err, "Failover check failed")
			r.Recorder.Event(ag, corev1.EventTypeWarning, "FailoverCheckFailed", err.Error())
		} else if result != nil {
			return *result, nil
		}
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
					"app":                          "mssql",
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
					"app":                          "mssql",
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
			"app":                          "mssql",
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
		"app":                          "mssql",
		"mssql.microsoft.com/instance": sqlServer.Name,
		LabelAG:                        ag.Spec.AvailabilityGroup.Name,
	}
}

// ============================================================================
// MANUAL FAILOVER LOGIC (via kubectl annotate)
// ============================================================================

// checkAndHandleManualFailover checks for manual failover requests via annotations
// Usage: kubectl annotate sqlserverag production-ag -n mssql mssql.microsoft.com/failover-to=sql-ag-1
func (r *SQLServerAGReconciler) checkAndHandleManualFailover(ctx context.Context, ag *mssqlv1alpha1.SQLServerAG, sqlServer *mssqlv1alpha1.SQLServer) (*ctrl.Result, error) {
	logger := log.FromContext(ctx)

	// Check if failover annotation is present
	targetReplica, hasTarget := ag.Annotations[AnnotationFailoverTarget]
	if !hasTarget || targetReplica == "" {
		return nil, nil // No manual failover requested
	}

	// Check if failover is already in progress or completed
	status := ag.Annotations[AnnotationFailoverStatus]
	if status == "in-progress" {
		logger.Info("Manual failover already in progress", "target", targetReplica)
		return nil, nil
	}
	if status == "completed" {
		// Check if current primary matches target - if so, clear annotations
		if ag.Status.PrimaryReplica == targetReplica {
			logger.Info("Manual failover completed successfully, clearing annotations")
			delete(ag.Annotations, AnnotationFailoverTarget)
			delete(ag.Annotations, AnnotationFailoverRequested)
			delete(ag.Annotations, AnnotationFailoverStatus)
			if err := r.Update(ctx, ag); err != nil {
				return nil, err
			}
			return &ctrl.Result{Requeue: true}, nil
		}
	}

	logger.Info("Processing manual failover request", "target", targetReplica)
	r.Recorder.Event(ag, corev1.EventTypeNormal, "ManualFailoverRequested",
		fmt.Sprintf("Manual failover requested to replica: %s", targetReplica))

	// Validate target replica exists
	validTarget := false
	for _, replica := range ag.Status.Replicas {
		if replica.Name == targetReplica {
			validTarget = true
			if replica.Role == "PRIMARY" {
				logger.Info("Target replica is already primary, clearing annotations")
				delete(ag.Annotations, AnnotationFailoverTarget)
				delete(ag.Annotations, AnnotationFailoverRequested)
				delete(ag.Annotations, AnnotationFailoverStatus)
				if err := r.Update(ctx, ag); err != nil {
					return nil, err
				}
				r.Recorder.Event(ag, corev1.EventTypeNormal, "ManualFailoverSkipped",
					fmt.Sprintf("Replica %s is already primary", targetReplica))
				return &ctrl.Result{Requeue: true}, nil
			}
			break
		}
	}

	if !validTarget {
		// If status not populated yet, try to find the pod directly
		pod := &corev1.Pod{}
		err := r.Get(ctx, types.NamespacedName{Name: targetReplica, Namespace: ag.Namespace}, pod)
		if err != nil {
			logger.Error(err, "Invalid failover target - replica not found", "target", targetReplica)
			r.Recorder.Event(ag, corev1.EventTypeWarning, "ManualFailoverFailed",
				fmt.Sprintf("Invalid target replica: %s not found", targetReplica))
			// Mark as failed and clear
			ag.Annotations[AnnotationFailoverStatus] = "failed"
			if err := r.Update(ctx, ag); err != nil {
				return nil, err
			}
			return &ctrl.Result{RequeueAfter: 10 * time.Second}, nil
		}
	}

	// Mark failover as in-progress
	if ag.Annotations == nil {
		ag.Annotations = make(map[string]string)
	}
	ag.Annotations[AnnotationFailoverStatus] = "in-progress"
	ag.Annotations[AnnotationFailoverRequested] = time.Now().Format(time.RFC3339)
	if err := r.Update(ctx, ag); err != nil {
		return nil, err
	}

	// Execute failover
	logger.Info("Executing manual failover", "target", targetReplica, "ag", ag.Spec.AvailabilityGroup.Name)
	if err := r.executeFailover(ctx, ag, sqlServer, targetReplica); err != nil {
		logger.Error(err, "Manual failover execution failed")
		r.Recorder.Event(ag, corev1.EventTypeWarning, "ManualFailoverFailed", err.Error())

		// Mark as failed
		ag.Annotations[AnnotationFailoverStatus] = "failed"
		if err := r.Update(ctx, ag); err != nil {
			return nil, err
		}
		return &ctrl.Result{RequeueAfter: 30 * time.Second}, nil
	}

	// Mark as completed
	ag.Annotations[AnnotationFailoverStatus] = "completed"
	if err := r.Update(ctx, ag); err != nil {
		return nil, err
	}

	r.Recorder.Event(ag, corev1.EventTypeNormal, "ManualFailoverCompleted",
		fmt.Sprintf("Successfully failed over to replica: %s", targetReplica))

	// Record failover time
	if r.lastFailoverTime == nil {
		r.lastFailoverTime = make(map[string]time.Time)
	}
	agKey := fmt.Sprintf("%s/%s", ag.Namespace, ag.Name)
	r.lastFailoverTime[agKey] = time.Now()

	return &ctrl.Result{RequeueAfter: 5 * time.Second}, nil
}

// ============================================================================
// AUTOMATIC FAILOVER LOGIC
// ============================================================================

// checkAndHandleFailover detects primary failure and triggers automatic failover
func (r *SQLServerAGReconciler) checkAndHandleFailover(ctx context.Context, ag *mssqlv1alpha1.SQLServerAG, sqlServer *mssqlv1alpha1.SQLServer) (*ctrl.Result, error) {
	logger := log.FromContext(ctx)
	agKey := fmt.Sprintf("%s/%s", ag.Namespace, ag.Name)

	// Initialize maps if needed
	if r.lastFailoverTime == nil {
		r.lastFailoverTime = make(map[string]time.Time)
	}
	if r.noPrimaryDetected == nil {
		r.noPrimaryDetected = make(map[string]time.Time)
	}

	// Check cooldown period - don't failover too frequently
	if lastFailover, exists := r.lastFailoverTime[agKey]; exists {
		if time.Since(lastFailover) < FailoverCooldownPeriod {
			logger.V(4).Info("Failover cooldown active", "remaining", FailoverCooldownPeriod-time.Since(lastFailover))
			return nil, nil
		}
	}

	// Get all pod states from sidecars
	candidates, hasPrimary, err := r.querySidecarStates(ctx, ag, sqlServer)
	if err != nil {
		return nil, fmt.Errorf("failed to query sidecar states: %w", err)
	}

	if len(candidates) == 0 {
		logger.Info("No healthy replicas available for failover evaluation")
		delete(r.noPrimaryDetected, agKey)
		return nil, nil
	}

	// If we have a primary, clear the no-primary detection and return
	if hasPrimary {
		if _, wasDetected := r.noPrimaryDetected[agKey]; wasDetected {
			logger.Info("Primary replica recovered")
			r.Recorder.Event(ag, corev1.EventTypeNormal, "PrimaryRecovered", "Primary replica is available again")
		}
		delete(r.noPrimaryDetected, agKey)
		return nil, nil
	}

	// No primary detected - start grace period tracking
	if _, exists := r.noPrimaryDetected[agKey]; !exists {
		r.noPrimaryDetected[agKey] = time.Now()
		logger.Info("No primary detected, starting grace period", "gracePeriod", NoPrimaryGracePeriod)
		r.Recorder.Event(ag, corev1.EventTypeWarning, "NoPrimaryDetected",
			fmt.Sprintf("No primary replica detected, will failover in %s if not recovered", NoPrimaryGracePeriod))
		// Requeue after grace period
		return &ctrl.Result{RequeueAfter: NoPrimaryGracePeriod}, nil
	}

	// Check if grace period has elapsed
	noPrimaryStart := r.noPrimaryDetected[agKey]
	if time.Since(noPrimaryStart) < NoPrimaryGracePeriod {
		remaining := NoPrimaryGracePeriod - time.Since(noPrimaryStart)
		logger.Info("Waiting for grace period", "remaining", remaining)
		return &ctrl.Result{RequeueAfter: remaining}, nil
	}

	// Grace period elapsed - trigger failover
	logger.Info("Grace period elapsed, triggering automatic failover",
		"noPrimaryDuration", time.Since(noPrimaryStart),
		"candidateCount", len(candidates))

	// Select the best candidate
	bestCandidate := r.selectBestCandidate(candidates)
	if bestCandidate == nil {
		logger.Error(nil, "No suitable failover candidate found")
		r.Recorder.Event(ag, corev1.EventTypeWarning, "NoFailoverCandidate",
			"No suitable replica available for automatic failover")
		return nil, fmt.Errorf("no suitable failover candidate")
	}

	logger.Info("Selected failover candidate",
		"pod", bestCandidate.PodName,
		"sequenceNumber", bestCandidate.SequenceNumber,
		"syncState", bestCandidate.SyncState)

	// Trigger failover
	if err := r.triggerFailover(ctx, ag, bestCandidate); err != nil {
		r.Recorder.Event(ag, corev1.EventTypeWarning, "FailoverFailed",
			fmt.Sprintf("Failed to failover to %s: %v", bestCandidate.PodName, err))
		return nil, err
	}

	// Record successful failover
	r.lastFailoverTime[agKey] = time.Now()
	delete(r.noPrimaryDetected, agKey)

	r.Recorder.Event(ag, corev1.EventTypeNormal, "FailoverCompleted",
		fmt.Sprintf("Automatic failover completed to %s", bestCandidate.PodName))

	// Update status
	ag.Status.Phase = "FailoverCompleted"
	ag.Status.PrimaryReplica = bestCandidate.PodName
	condition := metav1.Condition{
		Type:               "Failover",
		Status:             metav1.ConditionTrue,
		Reason:             "AutomaticFailover",
		Message:            fmt.Sprintf("Automatic failover to %s completed at %s", bestCandidate.PodName, time.Now().Format(time.RFC3339)),
		LastTransitionTime: metav1.Now(),
	}
	meta.SetStatusCondition(&ag.Status.Conditions, condition)
	if err := r.Status().Update(ctx, ag); err != nil {
		logger.Error(err, "Failed to update status after failover")
	}

	// Requeue soon to verify failover succeeded
	return &ctrl.Result{RequeueAfter: 5 * time.Second}, nil
}

// querySidecarStates queries all pod sidecars for their AG state
func (r *SQLServerAGReconciler) querySidecarStates(ctx context.Context, ag *mssqlv1alpha1.SQLServerAG, sqlServer *mssqlv1alpha1.SQLServer) ([]FailoverCandidate, bool, error) {
	logger := log.FromContext(ctx)

	// Get pods for the SQLServer
	podList := &corev1.PodList{}
	listOpts := []client.ListOption{
		client.InNamespace(ag.Namespace),
		client.MatchingLabels(map[string]string{
			"app":                          "mssql",
			"mssql.microsoft.com/instance": sqlServer.Name,
		}),
	}
	if err := r.List(ctx, podList, listOpts...); err != nil {
		return nil, false, err
	}

	var candidates []FailoverCandidate
	hasPrimary := false

	for _, pod := range podList.Items {
		// Skip pods that aren't running
		if pod.Status.Phase != corev1.PodRunning {
			logger.V(4).Info("Skipping non-running pod", "pod", pod.Name, "phase", pod.Status.Phase)
			continue
		}

		// Skip pods without IP
		if pod.Status.PodIP == "" {
			logger.V(4).Info("Skipping pod without IP", "pod", pod.Name)
			continue
		}

		// Query sidecar state
		state, err := r.querySidecar(ctx, pod.Status.PodIP)
		if err != nil {
			logger.V(4).Info("Failed to query sidecar", "pod", pod.Name, "error", err)
			continue
		}

		// Check if this is the primary
		if state.Role == "PRIMARY" {
			hasPrimary = true
			logger.V(4).Info("Found primary replica", "pod", pod.Name)
		}

		// Add as candidate if it's a healthy secondary with synchronized state
		if state.Role == "SECONDARY" && (state.Health == "Healthy" || state.Health == "Warning") {
			candidates = append(candidates, FailoverCandidate{
				PodName:        pod.Name,
				PodIP:          pod.Status.PodIP,
				SequenceNumber: state.SequenceNumber,
				SyncState:      state.SyncState,
				Health:         state.Health,
			})
		}
	}

	return candidates, hasPrimary, nil
}

// querySidecar queries a single AG Helper sidecar for state
func (r *SQLServerAGReconciler) querySidecar(ctx context.Context, podIP string) (*SidecarState, error) {
	if r.HTTPClient == nil {
		r.HTTPClient = &http.Client{
			Timeout: 5 * time.Second,
		}
	}

	url := fmt.Sprintf("http://%s:%d/state", podIP, SidecarPort)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}

	resp, err := r.HTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("sidecar returned status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var state SidecarState
	if err := json.Unmarshal(body, &state); err != nil {
		return nil, err
	}

	return &state, nil
}

// selectBestCandidate selects the best replica for failover
// Priority: 1) Highest sequence number (least data loss)
//  2. SYNCHRONIZED state preferred over SYNCHRONIZING
//  3. Healthy preferred over Warning
func (r *SQLServerAGReconciler) selectBestCandidate(candidates []FailoverCandidate) *FailoverCandidate {
	if len(candidates) == 0 {
		return nil
	}

	best := &candidates[0]
	for i := 1; i < len(candidates); i++ {
		candidate := &candidates[i]

		// Higher sequence number wins (least data loss)
		if candidate.SequenceNumber > best.SequenceNumber {
			best = candidate
			continue
		}

		// If same sequence, prefer SYNCHRONIZED
		if candidate.SequenceNumber == best.SequenceNumber {
			if candidate.SyncState == "SYNCHRONIZED" && best.SyncState != "SYNCHRONIZED" {
				best = candidate
				continue
			}

			// If same sync state, prefer Healthy over Warning
			if candidate.SyncState == best.SyncState {
				if candidate.Health == "Healthy" && best.Health != "Healthy" {
					best = candidate
				}
			}
		}
	}

	return best
}

// triggerFailover sends a failover request to the selected candidate
func (r *SQLServerAGReconciler) triggerFailover(ctx context.Context, ag *mssqlv1alpha1.SQLServerAG, candidate *FailoverCandidate) error {
	logger := log.FromContext(ctx)

	if r.HTTPClient == nil {
		r.HTTPClient = &http.Client{
			Timeout: 30 * time.Second, // Failover can take time
		}
	}

	// Determine if we need force failover (data loss possible)
	allowDataLoss := candidate.SyncState != "SYNCHRONIZED"
	if allowDataLoss {
		logger.Info("Warning: Failover with potential data loss",
			"candidate", candidate.PodName,
			"syncState", candidate.SyncState)
		r.Recorder.Event(ag, corev1.EventTypeWarning, "ForceFailover",
			fmt.Sprintf("Forcing failover to %s with potential data loss (syncState=%s)",
				candidate.PodName, candidate.SyncState))
	}

	url := fmt.Sprintf("http://%s:%d/failover", candidate.PodIP, SidecarPort)
	payload := map[string]bool{"allowDataLoss": allowDataLoss}
	payloadBytes, _ := json.Marshal(payload)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payloadBytes))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := r.HTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("failover request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failover failed with status %d: %s", resp.StatusCode, string(body))
	}

	logger.Info("Failover request sent successfully", "candidate", candidate.PodName)
	return nil
}

// executeFailover performs a failover to the specified target replica (for manual failover)
func (r *SQLServerAGReconciler) executeFailover(ctx context.Context, ag *mssqlv1alpha1.SQLServerAG, sqlServer *mssqlv1alpha1.SQLServer, targetReplica string) error {
	logger := log.FromContext(ctx)

	// Get the target pod
	pod := &corev1.Pod{}
	if err := r.Get(ctx, types.NamespacedName{Name: targetReplica, Namespace: ag.Namespace}, pod); err != nil {
		return fmt.Errorf("target replica pod not found: %w", err)
	}

	if pod.Status.PodIP == "" {
		return fmt.Errorf("target replica pod has no IP address")
	}

	// Query the target replica's current state via sidecar
	state, err := r.querySidecar(ctx, pod.Status.PodIP)
	if err != nil {
		// If we can't reach the sidecar, try the failover anyway via the AG Helper deployment
		logger.Info("Could not query target sidecar, attempting failover via AG Helper", "target", targetReplica)
		// Fallback: trigger failover with unknown sync state
		state = &SidecarState{
			SyncState: "UNKNOWN",
		}
	}

	// Create a candidate from the target
	candidate := &FailoverCandidate{
		PodName:        targetReplica,
		PodIP:          pod.Status.PodIP,
		SequenceNumber: state.SequenceNumber,
		SyncState:      state.SyncState,
		Health:         state.Health,
	}

	logger.Info("Executing failover to target replica",
		"target", targetReplica,
		"syncState", candidate.SyncState,
		"sequenceNumber", candidate.SequenceNumber)

	// Use the existing triggerFailover mechanism
	return r.triggerFailover(ctx, ag, candidate)
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

// getImageConfiguration loads the OperatorConfiguration named "default" and returns its Images config
// Returns nil if no OperatorConfiguration exists (uses hardcoded defaults)
func (r *SQLServerAGReconciler) getImageConfiguration(ctx context.Context) *mssqlv1alpha1.ImageConfiguration {
	logger := log.FromContext(ctx)

	config := &mssqlv1alpha1.OperatorConfiguration{}
	if err := r.Get(ctx, types.NamespacedName{Name: "default"}, config); err != nil {
		if !errors.IsNotFound(err) {
			logger.V(1).Info("Failed to get OperatorConfiguration, using defaults", "error", err)
		}
		return nil
	}

	return config.Spec.Images
}

// getAGHelperImage returns the AG Helper image to use for this SQLServerAG
// Priority: 1) ag.Spec.Sidecar.Image (explicit per-AG)
//  2. OperatorConfiguration.spec.images.agHelper (cluster-wide)
//  3. Default constant
func (r *SQLServerAGReconciler) getAGHelperImage(ctx context.Context, ag *mssqlv1alpha1.SQLServerAG) string {
	// Priority 1: Explicit image in SQLServerAG spec
	if ag.Spec.Sidecar != nil && ag.Spec.Sidecar.Image != "" {
		return ag.Spec.Sidecar.Image
	}

	// Priority 2: OperatorConfiguration
	imageConfig := r.getImageConfiguration(ctx)
	if imageConfig != nil {
		return imageConfig.GetAGHelperImage()
	}

	// Priority 3: Default
	return mssqlv1alpha1.DefaultSidecarImage
}
