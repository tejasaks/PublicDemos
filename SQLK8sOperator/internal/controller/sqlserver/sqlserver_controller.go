/*
Copyright 2026 Microsoft Corporation.

Licensed under the MIT License.
See LICENSE file in the project root for full license information.
*/

package sqlserver

import (
	"context"
	"fmt"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/log"

	mssqlv1alpha1 "github.com/microsoft/mssql-operator/pkg/apis/mssql.microsoft.com/v1alpha1"
)

const (
	// SQLServerFinalizer is the finalizer for SQLServer resources
	SQLServerFinalizer = "mssql.microsoft.com/finalizer"

	// Default port for SQL Server
	SQLServerPort = 1433

	// Default mssql user UID in containers
	MSSQLUserID = 10001

	// Labels
	LabelApp       = "app"
	LabelInstance  = "mssql.microsoft.com/instance"
	LabelVersion   = "mssql.microsoft.com/version"
	LabelComponent = "mssql.microsoft.com/component"
)

// SQLServerReconciler reconciles a SQLServer object
type SQLServerReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	Recorder record.EventRecorder
}

// +kubebuilder:rbac:groups=mssql.microsoft.com,resources=operatorconfigurations,verbs=get;list;watch
// +kubebuilder:rbac:groups=mssql.microsoft.com,resources=sqlservers,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=mssql.microsoft.com,resources=sqlservers/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=mssql.microsoft.com,resources=sqlservers/finalizers,verbs=update
// +kubebuilder:rbac:groups=apps,resources=statefulsets,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=services,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=pods,verbs=get;list;watch
// +kubebuilder:rbac:groups=core,resources=secrets,verbs=get;list;watch
// +kubebuilder:rbac:groups=core,resources=configmaps,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=persistentvolumeclaims,verbs=get;list;watch;create;delete
// +kubebuilder:rbac:groups=core,resources=events,verbs=create;patch

// Reconcile is the main reconciliation loop
func (r *SQLServerReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	// Fetch the SQLServer instance
	sqlServer := &mssqlv1alpha1.SQLServer{}
	if err := r.Get(ctx, req.NamespacedName, sqlServer); err != nil {
		if errors.IsNotFound(err) {
			logger.Info("SQLServer resource not found, likely deleted")
			return ctrl.Result{}, nil
		}
		logger.Error(err, "Failed to get SQLServer")
		return ctrl.Result{}, err
	}

	// Handle deletion
	if !sqlServer.DeletionTimestamp.IsZero() {
		return r.handleDeletion(ctx, sqlServer)
	}

	// Add finalizer if not present
	if !containsString(sqlServer.Finalizers, SQLServerFinalizer) {
		sqlServer.Finalizers = append(sqlServer.Finalizers, SQLServerFinalizer)
		if err := r.Update(ctx, sqlServer); err != nil {
			return ctrl.Result{}, err
		}
	}

	// Update status phase to Creating if Pending
	if sqlServer.Status.Phase == "" || sqlServer.Status.Phase == "Pending" {
		if err := r.updatePhase(ctx, sqlServer, "Creating"); err != nil {
			return ctrl.Result{}, err
		}
	}

	// Create or update ConfigMap for mssql.conf
	if err := r.reconcileConfigMap(ctx, sqlServer); err != nil {
		r.Recorder.Event(sqlServer, corev1.EventTypeWarning, "ConfigMapFailed", err.Error())
		return ctrl.Result{}, err
	}

	// Create or update headless Service
	if err := r.reconcileService(ctx, sqlServer); err != nil {
		r.Recorder.Event(sqlServer, corev1.EventTypeWarning, "ServiceFailed", err.Error())
		return ctrl.Result{}, err
	}

	// Create or update StatefulSet
	result, err := r.reconcileStatefulSet(ctx, sqlServer)
	if err != nil {
		r.Recorder.Event(sqlServer, corev1.EventTypeWarning, "StatefulSetFailed", err.Error())
		return result, err
	}

	// Update status based on StatefulSet status
	// Get image config again for status (could cache but it's a fast read)
	imageConfig := r.getImageConfiguration(ctx)
	if err := r.updateStatus(ctx, sqlServer, imageConfig); err != nil {
		return ctrl.Result{}, err
	}

	// Requeue to check status periodically
	return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
}

// reconcileConfigMap creates or updates the ConfigMap for mssql.conf
func (r *SQLServerReconciler) reconcileConfigMap(ctx context.Context, sqlServer *mssqlv1alpha1.SQLServer) error {
	logger := log.FromContext(ctx)

	configMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-config", sqlServer.Name),
			Namespace: sqlServer.Namespace,
			Labels:    r.labelsForSQLServer(sqlServer),
		},
		Data: map[string]string{
			"mssql.conf": r.generateMSSQLConf(sqlServer),
		},
	}

	// Set owner reference
	if err := ctrl.SetControllerReference(sqlServer, configMap, r.Scheme); err != nil {
		return err
	}

	// Check if ConfigMap exists
	found := &corev1.ConfigMap{}
	err := r.Get(ctx, types.NamespacedName{Name: configMap.Name, Namespace: configMap.Namespace}, found)
	if err != nil && errors.IsNotFound(err) {
		logger.Info("Creating ConfigMap", "name", configMap.Name)
		return r.Create(ctx, configMap)
	} else if err != nil {
		return err
	}

	// Update if needed
	found.Data = configMap.Data
	return r.Update(ctx, found)
}

// generateMSSQLConf generates the mssql.conf content
func (r *SQLServerReconciler) generateMSSQLConf(sqlServer *mssqlv1alpha1.SQLServer) string {
	conf := "[sqlagent]\n"
	if sqlServer.Spec.Instance.Config != nil && sqlServer.Spec.Instance.Config.AgentEnabled {
		conf += "enabled = true\n"
	} else {
		conf += "enabled = true\n" // Default to enabled
	}

	conf += "\n[hadr]\n"
	if sqlServer.Spec.Instance.Config != nil && sqlServer.Spec.Instance.Config.HADREnabled {
		conf += "hadrenabled = 1\n"
	} else {
		conf += "hadrenabled = 1\n" // Default to enabled for AG support
	}

	conf += "\n[language]\n"
	if sqlServer.Spec.Instance.Config != nil {
		conf += fmt.Sprintf("lcid = %d\n", sqlServer.Spec.Instance.Config.LCID)
	} else {
		conf += "lcid = 1033\n"
	}

	// Memory settings
	if sqlServer.Spec.Instance.Config != nil && sqlServer.Spec.Instance.Config.MemoryLimitMB != nil {
		conf += "\n[memory]\n"
		conf += fmt.Sprintf("memorylimitmb = %d\n", *sqlServer.Spec.Instance.Config.MemoryLimitMB)
	}

	// Custom config
	if sqlServer.Spec.Instance.Config != nil && sqlServer.Spec.Instance.Config.CustomMSSQLConf != "" {
		conf += "\n" + sqlServer.Spec.Instance.Config.CustomMSSQLConf
	}

	return conf
}

// reconcileService creates or updates the headless service
func (r *SQLServerReconciler) reconcileService(ctx context.Context, sqlServer *mssqlv1alpha1.SQLServer) error {
	logger := log.FromContext(ctx)

	// Headless service for StatefulSet
	headlessService := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-headless", sqlServer.Name),
			Namespace: sqlServer.Namespace,
			Labels:    r.labelsForSQLServer(sqlServer),
		},
		Spec: corev1.ServiceSpec{
			ClusterIP: corev1.ClusterIPNone,
			Selector:  r.selectorLabelsForSQLServer(sqlServer),
			Ports: []corev1.ServicePort{
				{
					Name:       "sql",
					Port:       SQLServerPort,
					TargetPort: intstr.FromInt(SQLServerPort),
				},
			},
		},
	}

	if err := ctrl.SetControllerReference(sqlServer, headlessService, r.Scheme); err != nil {
		return err
	}

	found := &corev1.Service{}
	err := r.Get(ctx, types.NamespacedName{Name: headlessService.Name, Namespace: headlessService.Namespace}, found)
	if err != nil && errors.IsNotFound(err) {
		logger.Info("Creating headless Service", "name", headlessService.Name)
		if err := r.Create(ctx, headlessService); err != nil {
			return err
		}
	} else if err != nil {
		return err
	}

	// Create client-facing service if specified
	if sqlServer.Spec.Service != nil {
		clientService := &corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name:        sqlServer.Name,
				Namespace:   sqlServer.Namespace,
				Labels:      r.labelsForSQLServer(sqlServer),
				Annotations: sqlServer.Spec.Service.Annotations,
			},
			Spec: corev1.ServiceSpec{
				Type:     sqlServer.Spec.Service.Type,
				Selector: r.selectorLabelsForSQLServer(sqlServer),
				Ports: []corev1.ServicePort{
					{
						Name:       "sql",
						Port:       sqlServer.Spec.Service.Port,
						TargetPort: intstr.FromInt(SQLServerPort),
					},
				},
			},
		}

		if sqlServer.Spec.Service.NodePort != nil {
			clientService.Spec.Ports[0].NodePort = *sqlServer.Spec.Service.NodePort
		}
		if sqlServer.Spec.Service.LoadBalancerIP != "" {
			clientService.Spec.LoadBalancerIP = sqlServer.Spec.Service.LoadBalancerIP
		}

		if err := ctrl.SetControllerReference(sqlServer, clientService, r.Scheme); err != nil {
			return err
		}

		found := &corev1.Service{}
		err := r.Get(ctx, types.NamespacedName{Name: clientService.Name, Namespace: clientService.Namespace}, found)
		if err != nil && errors.IsNotFound(err) {
			logger.Info("Creating client Service", "name", clientService.Name)
			if err := r.Create(ctx, clientService); err != nil {
				return err
			}
		} else if err != nil {
			return err
		}
	}

	return nil
}

// reconcileStatefulSet creates or updates the StatefulSet
func (r *SQLServerReconciler) reconcileStatefulSet(ctx context.Context, sqlServer *mssqlv1alpha1.SQLServer) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	// Get cluster-wide image configuration (if any)
	imageConfig := r.getImageConfiguration(ctx)

	sts := r.buildStatefulSet(ctx, sqlServer, imageConfig)

	if err := ctrl.SetControllerReference(sqlServer, sts, r.Scheme); err != nil {
		return ctrl.Result{}, err
	}

	found := &appsv1.StatefulSet{}
	err := r.Get(ctx, types.NamespacedName{Name: sts.Name, Namespace: sts.Namespace}, found)
	if err != nil && errors.IsNotFound(err) {
		logger.Info("Creating StatefulSet", "name", sts.Name)
		r.Recorder.Event(sqlServer, corev1.EventTypeNormal, "Creating", "Creating SQL Server StatefulSet")
		if err := r.Create(ctx, sts); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{RequeueAfter: 10 * time.Second}, nil
	} else if err != nil {
		return ctrl.Result{}, err
	}

	// Check if update is needed (following Zalando pattern)
	if needsUpdate, reason := r.compareStatefulSetWith(found, sts); needsUpdate {
		logger.Info("Updating StatefulSet", "name", sts.Name, "reason", reason)
		r.Recorder.Event(sqlServer, corev1.EventTypeNormal, "Updating", fmt.Sprintf("Updating StatefulSet: %s", reason))

		// Use OnDelete strategy - we control pod deletion during updates
		found.Spec.Template = sts.Spec.Template
		found.Spec.Replicas = sts.Spec.Replicas // K8s StatefulSet field

		if err := r.Update(ctx, found); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{RequeueAfter: 10 * time.Second}, nil
	}

	return ctrl.Result{}, nil
}

// compareStatefulSetWith compares current and desired StatefulSets (Zalando pattern)
func (r *SQLServerReconciler) compareStatefulSetWith(current, desired *appsv1.StatefulSet) (bool, string) {
	// Check instance count (K8s StatefulSet uses 'Replicas' field)
	if *current.Spec.Replicas != *desired.Spec.Replicas {
		return true, fmt.Sprintf("instance count changed from %d to %d", *current.Spec.Replicas, *desired.Spec.Replicas)
	}

	// Check container image
	if len(current.Spec.Template.Spec.Containers) > 0 && len(desired.Spec.Template.Spec.Containers) > 0 {
		if current.Spec.Template.Spec.Containers[0].Image != desired.Spec.Template.Spec.Containers[0].Image {
			return true, fmt.Sprintf("image changed from %s to %s",
				current.Spec.Template.Spec.Containers[0].Image,
				desired.Spec.Template.Spec.Containers[0].Image)
		}
	}

	// Check resource requirements
	if len(current.Spec.Template.Spec.Containers) > 0 && len(desired.Spec.Template.Spec.Containers) > 0 {
		currentRes := current.Spec.Template.Spec.Containers[0].Resources
		desiredRes := desired.Spec.Template.Spec.Containers[0].Resources

		if !currentRes.Limits.Cpu().Equal(*desiredRes.Limits.Cpu()) ||
			!currentRes.Limits.Memory().Equal(*desiredRes.Limits.Memory()) {
			return true, "resource limits changed"
		}
	}

	return false, ""
}

// buildStatefulSet creates the StatefulSet specification
// imageConfig is optional - if nil, uses hardcoded defaults
func (r *SQLServerReconciler) buildStatefulSet(ctx context.Context, sqlServer *mssqlv1alpha1.SQLServer, imageConfig *mssqlv1alpha1.ImageConfiguration) *appsv1.StatefulSet {
	labels := r.labelsForSQLServer(sqlServer)
	instanceCount := sqlServer.Spec.Instance.Count
	if instanceCount == 0 {
		instanceCount = 1
	}

	// Use OnDelete update strategy for operator-controlled rolling updates
	updateStrategy := appsv1.StatefulSetUpdateStrategy{
		Type: appsv1.OnDeleteStatefulSetStrategyType,
	}

	// Pod security context
	runAsNonRoot := true
	fsGroup := int64(MSSQLUserID)
	runAsUser := int64(MSSQLUserID)

	securityContext := &corev1.PodSecurityContext{
		RunAsNonRoot: &runAsNonRoot,
		RunAsUser:    &runAsUser,
		FSGroup:      &fsGroup,
	}
	if sqlServer.Spec.Instance.SecurityContext != nil {
		securityContext = sqlServer.Spec.Instance.SecurityContext
	}

	// Build environment variables
	env := []corev1.EnvVar{
		{
			Name:  "ACCEPT_EULA",
			Value: "Y",
		},
		{
			Name:  "MSSQL_PID",
			Value: sqlServer.Spec.GetEditionPID(),
		},
		{
			Name: "MSSQL_SA_PASSWORD",
			ValueFrom: &corev1.EnvVarSource{
				SecretKeyRef: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: sqlServer.Spec.Credentials.SAPasswordSecretRef.Name,
					},
					Key: sqlServer.Spec.Credentials.SAPasswordSecretRef.Key,
				},
			},
		},
	}

	// Add AD environment variables if enabled
	if sqlServer.Spec.ActiveDirectory != nil && sqlServer.Spec.ActiveDirectory.Enabled {
		env = append(env,
			corev1.EnvVar{Name: "MSSQL_ENABLE_AD_AUTH", Value: "1"},
			corev1.EnvVar{Name: "MSSQL_AD_REALM", Value: sqlServer.Spec.ActiveDirectory.Realm},
		)
		if sqlServer.Spec.ActiveDirectory.NetBIOSDomain != "" {
			env = append(env, corev1.EnvVar{
				Name:  "MSSQL_AD_NETBIOS_DOMAIN",
				Value: sqlServer.Spec.ActiveDirectory.NetBIOSDomain,
			})
		}
	}

	// Build volumes
	volumes := []corev1.Volume{
		{
			Name: "mssql-config",
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: fmt.Sprintf("%s-config", sqlServer.Name),
					},
				},
			},
		},
	}

	volumeMounts := []corev1.VolumeMount{
		{Name: "mssql-config", MountPath: "/var/opt/mssql/mssql.conf", SubPath: "mssql.conf"},
		{Name: "data", MountPath: "/var/opt/mssql/data"},
	}

	if sqlServer.Spec.Instance.Storage.Log != nil {
		volumeMounts = append(volumeMounts, corev1.VolumeMount{
			Name: "log", MountPath: "/var/opt/mssql/log",
		})
	}
	if sqlServer.Spec.Instance.Storage.TempDB != nil {
		volumeMounts = append(volumeMounts, corev1.VolumeMount{
			Name: "tempdb", MountPath: "/var/opt/mssql/tempdb",
		})
	}
	if sqlServer.Spec.Instance.Storage.Backup != nil {
		volumeMounts = append(volumeMounts, corev1.VolumeMount{
			Name: "backup", MountPath: "/var/opt/mssql/backup",
		})
	}

	// Build containers
	containers := []corev1.Container{
		{
			Name:            "mssql",
			Image:           sqlServer.Spec.GetImageWithConfig(imageConfig),
			ImagePullPolicy: sqlServer.Spec.Instance.ImagePullPolicy,
			Ports: []corev1.ContainerPort{
				{Name: "sql", ContainerPort: SQLServerPort},
			},
			Env:          env,
			VolumeMounts: volumeMounts,
			Resources:    sqlServer.Spec.Instance.Resources,
			ReadinessProbe: &corev1.Probe{
				ProbeHandler: corev1.ProbeHandler{
					TCPSocket: &corev1.TCPSocketAction{
						Port: intstr.FromInt(SQLServerPort),
					},
				},
				InitialDelaySeconds: 30,
				PeriodSeconds:       10,
				TimeoutSeconds:      5,
			},
			LivenessProbe: &corev1.Probe{
				ProbeHandler: corev1.ProbeHandler{
					TCPSocket: &corev1.TCPSocketAction{
						Port: intstr.FromInt(SQLServerPort),
					},
				},
				InitialDelaySeconds: 60,
				PeriodSeconds:       20,
				TimeoutSeconds:      5,
			},
		},
	}

	// Add monitoring sidecar if enabled
	if sqlServer.Spec.Monitoring != nil && sqlServer.Spec.Monitoring.Enabled {
		exporterImage := sqlServer.Spec.Monitoring.ExporterImage
		if exporterImage == "" {
			// Use OperatorConfiguration if available, otherwise default
			if imageConfig != nil {
				exporterImage = imageConfig.GetSQLExporterImage()
			} else {
				exporterImage = mssqlv1alpha1.DefaultExporterImage
			}
		}
		exporterPort := sqlServer.Spec.Monitoring.ExporterPort
		if exporterPort == 0 {
			exporterPort = 9399
		}

		containers = append(containers, corev1.Container{
			Name:  "exporter",
			Image: exporterImage,
			Ports: []corev1.ContainerPort{
				{Name: "metrics", ContainerPort: exporterPort},
			},
			Env: []corev1.EnvVar{
				{
					Name:  "SQLEXPORTER_TARGET_DSN",
					Value: fmt.Sprintf("sqlserver://sa:$(SA_PASSWORD)@localhost:%d", SQLServerPort),
				},
				{
					Name: "SA_PASSWORD",
					ValueFrom: &corev1.EnvVarSource{
						SecretKeyRef: &corev1.SecretKeySelector{
							LocalObjectReference: corev1.LocalObjectReference{
								Name: sqlServer.Spec.Credentials.SAPasswordSecretRef.Name,
							},
							Key: sqlServer.Spec.Credentials.SAPasswordSecretRef.Key,
						},
					},
				},
			},
		})
	}

	// Build VolumeClaimTemplates
	volumeClaimTemplates := []corev1.PersistentVolumeClaim{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:   "data",
				Labels: labels,
			},
			Spec: r.buildPVCSpec(sqlServer.Spec.Instance.Storage.Data),
		},
	}

	if sqlServer.Spec.Instance.Storage.Log != nil {
		volumeClaimTemplates = append(volumeClaimTemplates, corev1.PersistentVolumeClaim{
			ObjectMeta: metav1.ObjectMeta{Name: "log", Labels: labels},
			Spec:       r.buildPVCSpec(*sqlServer.Spec.Instance.Storage.Log),
		})
	}
	if sqlServer.Spec.Instance.Storage.TempDB != nil {
		volumeClaimTemplates = append(volumeClaimTemplates, corev1.PersistentVolumeClaim{
			ObjectMeta: metav1.ObjectMeta{Name: "tempdb", Labels: labels},
			Spec:       r.buildPVCSpec(*sqlServer.Spec.Instance.Storage.TempDB),
		})
	}
	if sqlServer.Spec.Instance.Storage.Backup != nil {
		volumeClaimTemplates = append(volumeClaimTemplates, corev1.PersistentVolumeClaim{
			ObjectMeta: metav1.ObjectMeta{Name: "backup", Labels: labels},
			Spec:       r.buildPVCSpec(*sqlServer.Spec.Instance.Storage.Backup),
		})
	}

	sts := &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      sqlServer.Name,
			Namespace: sqlServer.Namespace,
			Labels:    labels,
		},
		Spec: appsv1.StatefulSetSpec{
			ServiceName:          fmt.Sprintf("%s-headless", sqlServer.Name),
			Replicas:             &instanceCount, // K8s StatefulSet 'Replicas' field
			PodManagementPolicy:  appsv1.OrderedReadyPodManagement,
			UpdateStrategy:       updateStrategy,
			Selector:             &metav1.LabelSelector{MatchLabels: r.selectorLabelsForSQLServer(sqlServer)},
			VolumeClaimTemplates: volumeClaimTemplates,
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: labels,
				},
				Spec: corev1.PodSpec{
					SecurityContext:    securityContext,
					Containers:         containers,
					Volumes:            volumes,
					NodeSelector:       sqlServer.Spec.Instance.NodeSelector,
					Tolerations:        sqlServer.Spec.Instance.Tolerations,
					Affinity:           sqlServer.Spec.Instance.Affinity,
					ImagePullSecrets:   r.buildImagePullSecrets(sqlServer, imageConfig),
					PriorityClassName:  sqlServer.Spec.Instance.PriorityClassName,
					ServiceAccountName: "mssql-sa",
				},
			},
		},
	}

	return sts
}

// buildPVCSpec builds a PVC spec from VolumeSpec
func (r *SQLServerReconciler) buildPVCSpec(vol mssqlv1alpha1.VolumeSpec) corev1.PersistentVolumeClaimSpec {
	accessMode := vol.AccessMode
	if accessMode == "" {
		accessMode = corev1.ReadWriteOnce
	}

	spec := corev1.PersistentVolumeClaimSpec{
		AccessModes: []corev1.PersistentVolumeAccessMode{accessMode},
		Resources: corev1.VolumeResourceRequirements{
			Requests: corev1.ResourceList{
				corev1.ResourceStorage: resource.MustParse(vol.Size),
			},
		},
	}

	if vol.StorageClass != nil {
		spec.StorageClassName = vol.StorageClass
	}
	if vol.Selector != nil {
		spec.Selector = vol.Selector
	}

	return spec
}

// updateStatus updates the SQLServer status
func (r *SQLServerReconciler) updateStatus(ctx context.Context, sqlServer *mssqlv1alpha1.SQLServer, imageConfig *mssqlv1alpha1.ImageConfiguration) error {
	logger := log.FromContext(ctx)

	// Get StatefulSet
	sts := &appsv1.StatefulSet{}
	if err := r.Get(ctx, types.NamespacedName{Name: sqlServer.Name, Namespace: sqlServer.Namespace}, sts); err != nil {
		if errors.IsNotFound(err) {
			return nil
		}
		return err
	}

	// Get pods
	podList := &corev1.PodList{}
	listOpts := []client.ListOption{
		client.InNamespace(sqlServer.Namespace),
		client.MatchingLabels(r.selectorLabelsForSQLServer(sqlServer)),
	}
	if err := r.List(ctx, podList, listOpts...); err != nil {
		return err
	}

	// Build instance status
	instances := make([]mssqlv1alpha1.InstanceStatus, 0, len(podList.Items))
	readyCount := 0
	for _, pod := range podList.Items {
		ready := false
		for _, cond := range pod.Status.Conditions {
			if cond.Type == corev1.PodReady && cond.Status == corev1.ConditionTrue {
				ready = true
				readyCount++
				break
			}
		}
		instances = append(instances, mssqlv1alpha1.InstanceStatus{
			Name:  pod.Name,
			Ready: ready,
			PodIP: pod.Status.PodIP,
			Node:  pod.Spec.NodeName,
			Role:  "NotApplicable", // Will be updated by AG controller
		})
	}

	// Determine phase
	phase := "Creating"
	if sts.Status.ReadyReplicas == *sts.Spec.Replicas && readyCount > 0 {
		phase = "Running"
	} else if readyCount > 0 {
		phase = "Creating"
	}

	// Update status
	sqlServer.Status.Phase = phase
	sqlServer.Status.Ready = phase == "Running"
	sqlServer.Status.CurrentVersion = sqlServer.Spec.Version
	sqlServer.Status.CurrentImage = sqlServer.Spec.GetImageWithConfig(imageConfig)
	sqlServer.Status.Instances = instances
	sqlServer.Status.ObservedGeneration = sqlServer.Generation

	// Set Ready condition
	condition := metav1.Condition{
		Type:               "Ready",
		Status:             metav1.ConditionFalse,
		Reason:             "NotReady",
		Message:            "SQL Server is not ready",
		LastTransitionTime: metav1.Now(),
	}
	if sqlServer.Status.Ready {
		condition.Status = metav1.ConditionTrue
		condition.Reason = "Available"
		condition.Message = "SQL Server is ready to accept connections"
	}
	meta.SetStatusCondition(&sqlServer.Status.Conditions, condition)

	logger.Info("Updating status", "phase", phase, "ready", sqlServer.Status.Ready, "instances", len(instances))
	return r.Status().Update(ctx, sqlServer)
}

// updatePhase updates just the phase
func (r *SQLServerReconciler) updatePhase(ctx context.Context, sqlServer *mssqlv1alpha1.SQLServer, phase string) error {
	sqlServer.Status.Phase = phase
	return r.Status().Update(ctx, sqlServer)
}

// handleDeletion handles cleanup when SQLServer is deleted
func (r *SQLServerReconciler) handleDeletion(ctx context.Context, sqlServer *mssqlv1alpha1.SQLServer) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	if containsString(sqlServer.Finalizers, SQLServerFinalizer) {
		logger.Info("Running finalizer for SQLServer", "name", sqlServer.Name)
		r.Recorder.Event(sqlServer, corev1.EventTypeNormal, "Deleting", "Running cleanup finalizer")

		// Perform cleanup here if needed
		// Note: StatefulSet, Services, ConfigMaps are cleaned up by owner references

		// Remove finalizer
		sqlServer.Finalizers = removeString(sqlServer.Finalizers, SQLServerFinalizer)
		if err := r.Update(ctx, sqlServer); err != nil {
			return ctrl.Result{}, err
		}
	}

	return ctrl.Result{}, nil
}

// labelsForSQLServer returns labels for resources created by this SQLServer
func (r *SQLServerReconciler) labelsForSQLServer(sqlServer *mssqlv1alpha1.SQLServer) map[string]string {
	labels := map[string]string{
		LabelApp:       "mssql",
		LabelInstance:  sqlServer.Name,
		LabelVersion:   sqlServer.Spec.Version,
		LabelComponent: "database",
	}
	// Merge user labels
	if sqlServer.Spec.Metadata != nil && sqlServer.Spec.Metadata.Labels != nil {
		for k, v := range sqlServer.Spec.Metadata.Labels {
			labels[k] = v
		}
	}
	return labels
}

// selectorLabelsForSQLServer returns the selector labels
func (r *SQLServerReconciler) selectorLabelsForSQLServer(sqlServer *mssqlv1alpha1.SQLServer) map[string]string {
	return map[string]string{
		LabelApp:      "mssql",
		LabelInstance: sqlServer.Name,
	}
}

// SetupWithManager sets up the controller with the Manager
func (r *SQLServerReconciler) SetupWithManager(mgr ctrl.Manager, workers int) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&mssqlv1alpha1.SQLServer{}).
		Owns(&appsv1.StatefulSet{}).
		Owns(&corev1.Service{}).
		Owns(&corev1.ConfigMap{}).
		WithOptions(controller.Options{
			MaxConcurrentReconciles: workers,
		}).
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
func (r *SQLServerReconciler) getImageConfiguration(ctx context.Context) *mssqlv1alpha1.ImageConfiguration {
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

// buildImagePullSecrets merges ImagePullSecrets from SQLServer spec and OperatorConfiguration
// SQLServer-level secrets take precedence, then OperatorConfiguration secrets are added
func (r *SQLServerReconciler) buildImagePullSecrets(sqlServer *mssqlv1alpha1.SQLServer, imageConfig *mssqlv1alpha1.ImageConfiguration) []corev1.LocalObjectReference {
	// Start with SQLServer-level secrets
	secrets := sqlServer.Spec.Instance.ImagePullSecrets

	// Add OperatorConfiguration secrets if not already present
	if imageConfig != nil && len(imageConfig.ImagePullSecrets) > 0 {
		existingNames := make(map[string]bool)
		for _, s := range secrets {
			existingNames[s.Name] = true
		}
		for _, name := range imageConfig.ImagePullSecrets {
			if !existingNames[name] {
				secrets = append(secrets, corev1.LocalObjectReference{Name: name})
			}
		}
	}

	return secrets
}
