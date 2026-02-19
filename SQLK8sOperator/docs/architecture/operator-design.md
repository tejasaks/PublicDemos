# Operator Design

[← Back to Architecture](overview.md) | [Documentation Home](../README.md)

This document describes the internal design of the MSSQL Kubernetes Operator, including controller patterns, reconciliation logic, and error handling.

## Table of Contents

- [Controller Pattern](#controller-pattern)
- [Controller Structure](#controller-structure)
- [Reconciliation Flow](#reconciliation-flow)
- [Owner References](#owner-references)
- [Status Management](#status-management)
- [Error Handling](#error-handling)
- [Finalizers](#finalizers)
- [Event Recording](#event-recording)
- [Concurrency](#concurrency)

## Controller Pattern

The operator uses the **controller-runtime** framework, which implements the Kubernetes controller pattern:

```
┌─────────────────────────────────────────────────────────────────┐
│                    Controller Manager                            │
│                                                                  │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────────────┐   │
│  │ SQLServer    │  │ SQLServerAG  │  │ OperatorConfiguration│   │
│  │ Controller   │  │ Controller   │  │ Controller           │   │
│  └──────┬───────┘  └──────┬───────┘  └──────────┬───────────┘   │
│         │                 │                      │               │
│         ▼                 ▼                      ▼               │
│  ┌─────────────────────────────────────────────────────────────┐│
│  │                    Work Queue                                ││
│  │  - Rate-limited                                              ││
│  │  - Deduplication                                             ││
│  │  - Exponential backoff                                       ││
│  └─────────────────────────────────────────────────────────────┘│
└─────────────────────────────────────────────────────────────────┘
```

### Level-Triggered vs Edge-Triggered

The operator uses **level-triggered** reconciliation:
- Controllers react to the current state, not specific events
- Reconciliation is idempotent - running multiple times produces same result
- Missing events don't cause drift

## Controller Structure

### SQLServer Controller

```go
type SQLServerReconciler struct {
    client.Client
    Scheme   *runtime.Scheme
    Recorder record.EventRecorder
    Config   *operatorconfig.Config
}

func (r *SQLServerReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
    // 1. Fetch the SQLServer instance
    // 2. Handle deletion (finalizers)
    // 3. Set defaults
    // 4. Validate
    // 5. Reconcile child resources
    // 6. Update status
    // 7. Return result
}
```

### SQLServerAG Controller

```go
type SQLServerAGReconciler struct {
    client.Client
    Scheme     *runtime.Scheme
    Recorder   record.EventRecorder
    HTTPClient *http.Client
}

func (r *SQLServerAGReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
    // 1. Fetch the SQLServerAG instance
    // 2. Handle deletion (finalizers)
    // 3. Fetch referenced SQLServer
    // 4. Check SQLServer readiness
    // 5. Update AG status (query each pod's sidecar at :8080/state)
    // 6. Re-fetch AG (fresh resourceVersion after status write)
    // 7. Reconcile listener (if spec.listener is configured)
    // 8. Check manual failover (annotation-triggered)
    // 9. Check automatic failover (if enabled)
    // 10. Requeue after monitorInterval
}
```

## Reconciliation Flow

### SQLServer Reconcile Loop

```
┌────────────────────────────────────────────────────────────────┐
│                    SQLServer Reconcile Loop                     │
├────────────────────────────────────────────────────────────────┤
│  1. Fetch SQLServer CR                                          │
│     └─ If not found → return (deleted, GC handles cleanup)     │
│                                                                 │
│  2. Check for Finalizer                                         │
│     └─ If deleting → run cleanup, remove finalizer, return     │
│                                                                 │
│  3. Set Default Values                                          │
│     └─ Apply OperatorConfiguration defaults                    │
│     └─ Set default image based on version                      │
│     └─ Set default resource limits                             │
│                                                                 │
│  4. Validate Spec                                               │
│     └─ Name length (≤13 chars)                                 │
│     └─ Memory limit (≥2Gi)                                     │
│     └─ StorageClass exists                                     │
│     └─ Secret exists (warn only)                               │
│                                                                 │
│  5. Reconcile Child Resources (in order):                       │
│     ├─ 5a. ConfigMap (mssql.conf)                              │
│     ├─ 5b. PVCs (data, log, tempdb, backup)                    │
│     ├─ 5c. StatefulSet                                         │
│     └─ 5d. Services (headless, client)                         │
│                                                                 │
│  6. Update Status                                               │
│     └─ Phase: Pending → Creating → Running → Failed            │
│     └─ Conditions: Ready, Available, Progressing               │
│     └─ ReadyInstances, CurrentInstances                          │
│                                                                 │
│  7. Return Result                                               │
│     └─ Success: return ctrl.Result{}, nil                      │
│     └─ Requeue: return ctrl.Result{Requeue: true}, nil         │
│     └─ Error: return ctrl.Result{}, err                        │
└────────────────────────────────────────────────────────────────┘
```

### Child Resource Reconciliation

Each child resource follows this pattern:

```go
func (r *SQLServerReconciler) reconcileStatefulSet(ctx context.Context, sqlserver *v1alpha1.SQLServer) error {
    // 1. Build desired StatefulSet spec
    desired := r.buildStatefulSet(sqlserver)
    
    // 2. Set owner reference
    if err := controllerutil.SetControllerReference(sqlserver, desired, r.Scheme); err != nil {
        return err
    }
    
    // 3. Get current StatefulSet
    current := &appsv1.StatefulSet{}
    err := r.Get(ctx, types.NamespacedName{Name: desired.Name, Namespace: desired.Namespace}, current)
    
    if errors.IsNotFound(err) {
        // 4a. Create if not exists
        return r.Create(ctx, desired)
    } else if err != nil {
        return err
    }
    
    // 4b. Update if changed
    if !reflect.DeepEqual(current.Spec, desired.Spec) {
        current.Spec = desired.Spec
        return r.Update(ctx, current)
    }
    
    return nil
}
```

## Owner References

All child resources are created with OwnerReferences:

```yaml
apiVersion: apps/v1
kind: StatefulSet
metadata:
  name: sql-prod-01
  namespace: mssql
  ownerReferences:
    - apiVersion: mssql.microsoft.com/v1alpha1
      kind: SQLServer
      name: sql-prod-01
      uid: abc-123-def-456
      controller: true
      blockOwnerDeletion: true
```

### Benefits

| Feature | Description |
|---------|-------------|
| **Garbage Collection** | Child resources auto-deleted when parent is deleted |
| **Visibility** | `kubectl get sts --show-owners` shows ownership |
| **Adoption** | Orphaned resources can be re-adopted |
| **Blocking** | `blockOwnerDeletion` prevents parent deletion while children exist |

## Status Management

### Status Structure

```go
type SQLServerStatus struct {
    // Phase represents the current lifecycle phase
    Phase SQLServerPhase `json:"phase"`
    
    // Conditions represent detailed status
    Conditions []metav1.Condition `json:"conditions,omitempty"`
    
    // Instance counts
    ReadyInstances   int32 `json:"readyInstances"`
    CurrentInstances int32 `json:"currentInstances"`
    
    // Version tracking
    CurrentVersion string `json:"currentVersion,omitempty"`
    TargetVersion  string `json:"targetVersion,omitempty"`
}
```

### Phase Transitions

```
     ┌─────────────────────────────────────────────────────────┐
     │                                                         │
     ▼                                                         │
┌─────────┐     ┌──────────┐     ┌─────────┐     ┌────────┐   │
│ Pending │────▶│ Creating │────▶│ Running │────▶│ Failed │───┘
└─────────┘     └──────────┘     └────┬────┘     └────────┘
                                      │
                                      ▼
                                 ┌──────────┐
                                 │ Upgrading│
                                 └──────────┘
```

### Conditions Pattern

Following Kubernetes conventions for conditions:

```go
// Update condition helper
func (r *SQLServerReconciler) setCondition(sqlserver *v1alpha1.SQLServer, 
    condType string, status metav1.ConditionStatus, reason, message string) {
    
    condition := metav1.Condition{
        Type:               condType,
        Status:             status,
        Reason:             reason,
        Message:            message,
        LastTransitionTime: metav1.Now(),
    }
    
    meta.SetStatusCondition(&sqlserver.Status.Conditions, condition)
}
```

## Error Handling

### Requeue Strategies

| Scenario | Strategy | Code |
|----------|----------|------|
| Transient error | Requeue with backoff | `return ctrl.Result{}, err` |
| Waiting for resource | Requeue after delay | `return ctrl.Result{RequeueAfter: 10*time.Second}, nil` |
| Permanent error | Don't requeue, update status | `return ctrl.Result{}, nil` |
| Success | No requeue | `return ctrl.Result{}, nil` |

### Error Categories

```go
func (r *SQLServerReconciler) handleReconcileError(err error, sqlserver *v1alpha1.SQLServer) (ctrl.Result, error) {
    switch {
    case errors.IsConflict(err):
        // Optimistic locking conflict - requeue immediately
        return ctrl.Result{Requeue: true}, nil
        
    case errors.IsNotFound(err):
        // Resource was deleted - nothing to do
        return ctrl.Result{}, nil
        
    case errors.IsInvalid(err):
        // Validation error - don't requeue, update status
        r.setCondition(sqlserver, "Ready", metav1.ConditionFalse, "ValidationFailed", err.Error())
        return ctrl.Result{}, nil
        
    default:
        // Transient error - requeue with backoff
        return ctrl.Result{}, err
    }
}
```

## Finalizers

Finalizers ensure cleanup logic runs before resource deletion:

```go
const finalizerName = "mssql.microsoft.com/finalizer"

func (r *SQLServerReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
    sqlserver := &v1alpha1.SQLServer{}
    if err := r.Get(ctx, req.NamespacedName, sqlserver); err != nil {
        return ctrl.Result{}, client.IgnoreNotFound(err)
    }
    
    // Check if being deleted
    if !sqlserver.DeletionTimestamp.IsZero() {
        if controllerutil.ContainsFinalizer(sqlserver, finalizerName) {
            // Run cleanup logic
            if err := r.cleanupResources(ctx, sqlserver); err != nil {
                return ctrl.Result{}, err
            }
            
            // Remove finalizer
            controllerutil.RemoveFinalizer(sqlserver, finalizerName)
            if err := r.Update(ctx, sqlserver); err != nil {
                return ctrl.Result{}, err
            }
        }
        return ctrl.Result{}, nil
    }
    
    // Add finalizer if not present
    if !controllerutil.ContainsFinalizer(sqlserver, finalizerName) {
        controllerutil.AddFinalizer(sqlserver, finalizerName)
        if err := r.Update(ctx, sqlserver); err != nil {
            return ctrl.Result{}, err
        }
    }
    
    // Continue with normal reconciliation...
}
```

## Event Recording

Events provide visibility into controller actions:

```go
// Record successful creation
r.Recorder.Event(sqlserver, corev1.EventTypeNormal, "Created", 
    fmt.Sprintf("Created StatefulSet %s", sts.Name))

// Record warning
r.Recorder.Event(sqlserver, corev1.EventTypeWarning, "StorageClassNotFound",
    fmt.Sprintf("StorageClass %s not found, using default", storageClass))

// Record error
r.Recorder.Event(sqlserver, corev1.EventTypeWarning, "ReconcileError",
    fmt.Sprintf("Failed to reconcile: %v", err))
```

View events:
```bash
kubectl get events -n mssql --field-selector involvedObject.name=sql-prod-01
```

## Concurrency

### Controller Concurrency

```go
func (r *SQLServerReconciler) SetupWithManager(mgr ctrl.Manager) error {
    return ctrl.NewControllerManagedBy(mgr).
        For(&v1alpha1.SQLServer{}).
        Owns(&appsv1.StatefulSet{}).
        Owns(&corev1.Service{}).
        Owns(&corev1.ConfigMap{}).
        WithOptions(controller.Options{
            MaxConcurrentReconciles: 3,  // Process up to 3 SQLServers concurrently
        }).
        Complete(r)
}
```

### Work Queue Deduplication

The work queue automatically deduplicates requests:
- If `sql-prod-01` is already in the queue, new requests are ignored
- Prevents thundering herd on rapid updates

### Rate Limiting

Default rate limiter configuration:
- Initial backoff: 5ms
- Max backoff: 1000s
- Exponential increase on repeated failures

## Next Steps

- [CRD Design](crd-design.md) - Custom Resource Definition details
- [Sidecar Architecture](sidecar-architecture.md) - AG Helper and SQL Exporter
- [Networking](networking.md) - Services and traffic flow
