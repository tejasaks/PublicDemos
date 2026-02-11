/*
Copyright 2026 Microsoft Corporation.

Licensed under the MIT License.
See LICENSE file in the project root for full license information.
*/

package main

import (
	"context"
	"flag"
	"os"
	"time"

	// Import all Kubernetes client auth plugins (e.g. Azure, GCP, OIDC, etc.)
	_ "k8s.io/client-go/plugin/pkg/client/auth"

	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"
	crwebhook "sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	sqlservercontroller "github.com/microsoft/mssql-operator/internal/controller/sqlserver"
	agcontroller "github.com/microsoft/mssql-operator/internal/controller/sqlserverag"
	webhookhandlers "github.com/microsoft/mssql-operator/internal/webhook"
	webhookcerts "github.com/microsoft/mssql-operator/internal/webhook/certs"
	mssqlv1alpha1 "github.com/microsoft/mssql-operator/pkg/apis/mssql.microsoft.com/v1alpha1"
)

var (
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(mssqlv1alpha1.AddToScheme(scheme))
	// +kubebuilder:scaffold:scheme
}

func main() {
	var metricsAddr string
	var enableLeaderElection bool
	var probeAddr string
	var namespace string
	var resyncPeriod time.Duration
	var workers int

	flag.StringVar(&metricsAddr, "metrics-bind-address", ":8080", "The address the metric endpoint binds to.")
	flag.StringVar(&probeAddr, "health-probe-bind-address", ":8081", "The address the probe endpoint binds to.")
	flag.BoolVar(&enableLeaderElection, "leader-elect", false,
		"Enable leader election for controller manager. "+
			"Enabling this will ensure there is only one active controller manager.")
	flag.StringVar(&namespace, "namespace", "",
		"The namespace to watch for SQLServer resources. Empty means all namespaces.")
	flag.DurationVar(&resyncPeriod, "resync-period", 30*time.Minute,
		"The resync period for the controller.")
	flag.IntVar(&workers, "workers", 8, "The number of concurrent workers.")

	opts := zap.Options{
		Development: true,
	}
	opts.BindFlags(flag.CommandLine)
	flag.Parse()

	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&opts)))

	// Determine operator namespace (for webhook cert DNS names)
	operatorNamespace := os.Getenv("OPERATOR_NAMESPACE")
	if operatorNamespace == "" {
		operatorNamespace = "mssql-system"
	}

	// --- Webhook TLS Setup ---
	// WEBHOOK_CERT_MODE controls how TLS certificates are provisioned:
	//   "self-signed" (default) - auto-generate CA + server cert at startup
	//   "manual"                - use certs from a pre-created TLS secret
	//   "cert-manager"          - use certs from a cert-manager-managed secret
	//
	// For "manual" and "cert-manager" modes:
	//   WEBHOOK_TLS_SECRET_NAME (default: "mssql-operator-webhook-tls")
	//
	// For "manual" mode: if the secret contains "ca.crt", the operator patches
	//   the caBundle automatically. Otherwise, manage caBundle externally.
	//
	// For "cert-manager" mode: caBundle is managed by cert-manager via the
	//   cert-manager.io/inject-ca-from annotation on the webhook config.

	webhookCertDir := "/tmp/webhook-certs"
	webhookServiceName := "mssql-operator-webhook"
	webhookConfigName := "mssql-operator-validating-webhook"
	webhookTLSSecretName := os.Getenv("WEBHOOK_TLS_SECRET_NAME")
	if webhookTLSSecretName == "" {
		webhookTLSSecretName = "mssql-operator-webhook-tls"
	}

	certMode := webhookcerts.ParseCertMode(os.Getenv("WEBHOOK_CERT_MODE"))
	setupLog.Info("webhook TLS certificate mode",
		"mode", string(certMode),
		"namespace", operatorNamespace,
	)

	cfg := ctrl.GetConfigOrDie()

	directClient, err := client.New(cfg, client.Options{Scheme: scheme})
	if err != nil {
		setupLog.Error(err, "unable to create client for webhook setup")
		os.Exit(1)
	}

	switch certMode {
	case webhookcerts.CertModeSelfSigned:
		// Generate self-signed CA + server certificate
		setupLog.Info("generating self-signed webhook TLS certificates",
			"serviceName", webhookServiceName,
			"certDir", webhookCertDir,
		)

		certs, err := webhookcerts.GenerateSelfSignedCerts(webhookServiceName, operatorNamespace)
		if err != nil {
			setupLog.Error(err, "unable to generate webhook TLS certificates")
			os.Exit(1)
		}

		if err := webhookcerts.WriteCertsToDir(certs, webhookCertDir); err != nil {
			setupLog.Error(err, "unable to write webhook TLS certificates")
			os.Exit(1)
		}
		setupLog.Info("self-signed webhook TLS certificates generated successfully")

		// Patch the ValidatingWebhookConfiguration with the CA bundle
		if err := webhookcerts.PatchWebhookCABundle(context.Background(), directClient,
			webhookConfigName, certs.CACert); err != nil {
			setupLog.Info("could not patch webhook CA bundle (webhook config may not be deployed yet)",
				"error", err.Error())
		} else {
			setupLog.Info("webhook CA bundle patched successfully",
				"webhookConfig", webhookConfigName)
		}

	case webhookcerts.CertModeManual:
		// Load certificates from a pre-created Kubernetes TLS secret
		setupLog.Info("loading webhook TLS certificates from secret",
			"secret", webhookTLSSecretName,
			"namespace", operatorNamespace,
		)

		certs, err := webhookcerts.LoadCertsFromSecret(context.Background(), directClient,
			webhookTLSSecretName, operatorNamespace)
		if err != nil {
			setupLog.Error(err, "unable to load webhook TLS certificates from secret",
				"secret", webhookTLSSecretName)
			os.Exit(1)
		}

		if err := webhookcerts.WriteCertsToDir(certs, webhookCertDir); err != nil {
			setupLog.Error(err, "unable to write webhook TLS certificates")
			os.Exit(1)
		}
		setupLog.Info("webhook TLS certificates loaded from secret successfully")

		// If the secret contains ca.crt, patch the caBundle automatically
		if len(certs.CACert) > 0 {
			if err := webhookcerts.PatchWebhookCABundle(context.Background(), directClient,
				webhookConfigName, certs.CACert); err != nil {
				setupLog.Info("could not patch webhook CA bundle",
					"error", err.Error())
			} else {
				setupLog.Info("webhook CA bundle patched from ca.crt in secret",
					"webhookConfig", webhookConfigName)
			}
		} else {
			setupLog.Info("no ca.crt found in TLS secret; caBundle must be managed externally",
				"secret", webhookTLSSecretName)
		}

	case webhookcerts.CertModeCertManager:
		// Load certificates from the cert-manager-managed TLS secret
		setupLog.Info("loading webhook TLS certificates from cert-manager secret",
			"secret", webhookTLSSecretName,
			"namespace", operatorNamespace,
		)

		certs, err := webhookcerts.LoadCertsFromSecret(context.Background(), directClient,
			webhookTLSSecretName, operatorNamespace)
		if err != nil {
			setupLog.Error(err, "unable to load webhook TLS certificates from cert-manager secret",
				"secret", webhookTLSSecretName)
			os.Exit(1)
		}

		if err := webhookcerts.WriteCertsToDir(certs, webhookCertDir); err != nil {
			setupLog.Error(err, "unable to write webhook TLS certificates")
			os.Exit(1)
		}
		setupLog.Info("webhook TLS certificates loaded from cert-manager secret successfully")

		// In cert-manager mode, caBundle is injected via the
		// cert-manager.io/inject-ca-from annotation on the
		// ValidatingWebhookConfiguration — no patching needed here.
		setupLog.Info("caBundle managed by cert-manager annotation (no operator patching)")
	}

	// --- Manager Setup ---
	mgr, err := ctrl.NewManager(cfg, ctrl.Options{
		Scheme: scheme,
		Metrics: metricsserver.Options{
			BindAddress: metricsAddr,
		},
		HealthProbeBindAddress: probeAddr,
		LeaderElection:         enableLeaderElection,
		LeaderElectionID:       "mssql-operator-leader-election",
		// LeaderElectionReleaseOnCancel defines if the leader should step down voluntarily
		// when the Manager ends. This requires the binary to immediately end when the
		// Manager is stopped, otherwise, this setting is unsafe.
		LeaderElectionReleaseOnCancel: true,
		// Webhook server serves admission webhooks on port 9443 with self-signed TLS certs
		WebhookServer: crwebhook.NewServer(crwebhook.Options{
			Port:    9443,
			CertDir: webhookCertDir,
		}),
	})
	if err != nil {
		setupLog.Error(err, "unable to create manager")
		os.Exit(1)
	}

	// SQLServer controller
	if err = (&sqlservercontroller.SQLServerReconciler{
		Client:   mgr.GetClient(),
		Scheme:   mgr.GetScheme(),
		Recorder: mgr.GetEventRecorderFor("sqlserver-controller"),
	}).SetupWithManager(mgr, workers); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "SQLServer")
		os.Exit(1)
	}

	// SQLServerAG controller
	if err = (&agcontroller.SQLServerAGReconciler{
		Client:   mgr.GetClient(),
		Scheme:   mgr.GetScheme(),
		Recorder: mgr.GetEventRecorderFor("sqlserverag-controller"),
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "SQLServerAG")
		os.Exit(1)
	}

	// +kubebuilder:scaffold:builder

	// --- Webhook Handler Registration ---
	// Create a decoder for the admission webhooks
	decoder := admission.NewDecoder(scheme)

	sqlServerValidator := webhookhandlers.NewSQLServerValidator(mgr.GetClient(), nil)
	if err := sqlServerValidator.InjectDecoder(decoder); err != nil {
		setupLog.Error(err, "unable to inject decoder into SQLServer webhook")
		os.Exit(1)
	}

	agValidator := webhookhandlers.NewSQLServerAGValidator(mgr.GetClient(), nil)
	if err := agValidator.InjectDecoder(decoder); err != nil {
		setupLog.Error(err, "unable to inject decoder into SQLServerAG webhook")
		os.Exit(1)
	}

	hookServer := mgr.GetWebhookServer()
	hookServer.Register("/validate-mssql-microsoft-com-v1alpha1-sqlserver",
		&admission.Webhook{Handler: sqlServerValidator})
	hookServer.Register("/validate-mssql-microsoft-com-v1alpha1-sqlserverag",
		&admission.Webhook{Handler: agValidator})
	setupLog.Info("webhook handlers registered")

	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up health check")
		os.Exit(1)
	}
	if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up ready check")
		os.Exit(1)
	}

	setupLog.Info("starting manager",
		"version", "v0.1.0",
		"workers", workers,
		"namespace", namespace,
		"resyncPeriod", resyncPeriod,
	)
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		setupLog.Error(err, "problem running manager")
		os.Exit(1)
	}
}
