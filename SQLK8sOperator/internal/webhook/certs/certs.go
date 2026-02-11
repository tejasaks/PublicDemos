/*
Copyright 2026 Microsoft Corporation.

Licensed under the MIT License.
See LICENSE file in the project root for full license information.
*/

// Package certs provides TLS certificate management for the operator's admission
// webhook server. It supports three certificate modes:
//
//   - self-signed (default): Generates a self-signed CA + server certificate at
//     startup and patches the ValidatingWebhookConfiguration caBundle automatically.
//
//   - manual: Uses certificates from a pre-created Kubernetes TLS secret
//     (e.g. "mssql-operator-webhook-tls"). If the secret contains a "ca.crt" key,
//     the operator patches the caBundle automatically. Enterprises can rotate certs
//     by updating the secret and restarting the operator (or using cert-manager).
//
//   - cert-manager: Like manual, but relies on cert-manager to populate the TLS
//     secret and inject the CA bundle via the cert-manager.io/inject-ca-from
//     annotation on the ValidatingWebhookConfiguration. The operator does not
//     patch caBundle in this mode.
package certs

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"time"

	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// CertMode defines how the webhook TLS certificates are provisioned.
type CertMode string

const (
	// CertModeSelfSigned generates a self-signed CA + server cert at startup.
	// The caBundle is patched into the ValidatingWebhookConfiguration automatically.
	// This is the default mode suitable for development and quick-start scenarios.
	CertModeSelfSigned CertMode = "self-signed"

	// CertModeManual uses certificates from a pre-created Kubernetes TLS secret.
	// The secret must contain "tls.crt" and "tls.key". If "ca.crt" is present,
	// it will be used to patch the caBundle in the ValidatingWebhookConfiguration.
	// Enterprises can rotate certs by updating the secret and restarting the operator.
	CertModeManual CertMode = "manual"

	// CertModeCertManager uses certificates provisioned by cert-manager.
	// The TLS secret is created/rotated by cert-manager, and the caBundle is
	// injected via the cert-manager.io/inject-ca-from annotation. The operator
	// does not patch caBundle in this mode.
	CertModeCertManager CertMode = "cert-manager"
)

// ParseCertMode parses a string into a CertMode, returning CertModeSelfSigned
// for empty or unrecognized values.
func ParseCertMode(s string) CertMode {
	switch CertMode(s) {
	case CertModeSelfSigned, CertModeManual, CertModeCertManager:
		return CertMode(s)
	default:
		return CertModeSelfSigned
	}
}

// CertificateArtifacts holds the generated TLS certificate artifacts.
type CertificateArtifacts struct {
	// CACert is the PEM-encoded CA certificate (used for caBundle in webhook config).
	CACert []byte
	// ServerCert is the PEM-encoded server certificate.
	ServerCert []byte
	// ServerKey is the PEM-encoded server private key.
	ServerKey []byte
}

// GenerateSelfSignedCerts generates a self-signed CA and a server certificate
// signed by that CA. The server certificate includes the DNS names derived from
// the webhook service name and namespace:
//   - <serviceName>.<namespace>.svc
//   - <serviceName>.<namespace>.svc.cluster.local
func GenerateSelfSignedCerts(serviceName, namespace string) (*CertificateArtifacts, error) {
	dnsNames := []string{
		fmt.Sprintf("%s.%s.svc", serviceName, namespace),
		fmt.Sprintf("%s.%s.svc.cluster.local", serviceName, namespace),
	}

	// Generate CA private key (ECDSA P-256)
	caKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("generating CA key: %w", err)
	}

	// Create self-signed CA certificate
	caTemplate := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			Organization: []string{"MSSQL Operator"},
			CommonName:   "MSSQL Operator Webhook CA",
		},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(10 * 365 * 24 * time.Hour), // 10 years
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
		BasicConstraintsValid: true,
		IsCA:                  true,
	}

	caCertDER, err := x509.CreateCertificate(rand.Reader, caTemplate, caTemplate, &caKey.PublicKey, caKey)
	if err != nil {
		return nil, fmt.Errorf("creating CA certificate: %w", err)
	}

	caCert, err := x509.ParseCertificate(caCertDER)
	if err != nil {
		return nil, fmt.Errorf("parsing CA certificate: %w", err)
	}

	// Generate server private key (ECDSA P-256)
	serverKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("generating server key: %w", err)
	}

	// Create server certificate signed by the CA
	serverTemplate := &x509.Certificate{
		SerialNumber: big.NewInt(2),
		Subject: pkix.Name{
			Organization: []string{"MSSQL Operator"},
			CommonName:   dnsNames[0],
		},
		DNSNames:  dnsNames,
		NotBefore: time.Now().Add(-time.Hour),
		NotAfter:  time.Now().Add(10 * 365 * 24 * time.Hour), // 10 years
		KeyUsage:  x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage: []x509.ExtKeyUsage{
			x509.ExtKeyUsageServerAuth,
		},
	}

	serverCertDER, err := x509.CreateCertificate(rand.Reader, serverTemplate, caCert, &serverKey.PublicKey, caKey)
	if err != nil {
		return nil, fmt.Errorf("signing server certificate: %w", err)
	}

	// Encode everything to PEM
	caCertPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: caCertDER})
	serverCertPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: serverCertDER})

	serverKeyBytes, err := x509.MarshalECPrivateKey(serverKey)
	if err != nil {
		return nil, fmt.Errorf("marshaling server key: %w", err)
	}
	serverKeyPEM := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: serverKeyBytes})

	return &CertificateArtifacts{
		CACert:     caCertPEM,
		ServerCert: serverCertPEM,
		ServerKey:  serverKeyPEM,
	}, nil
}

// WriteCertsToDir writes the server certificate and key to the specified directory
// using the filenames tls.crt and tls.key (expected by controller-runtime's webhook server).
func WriteCertsToDir(artifacts *CertificateArtifacts, dir string) error {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("creating cert directory %s: %w", dir, err)
	}

	if err := os.WriteFile(filepath.Join(dir, "tls.crt"), artifacts.ServerCert, 0644); err != nil {
		return fmt.Errorf("writing tls.crt: %w", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "tls.key"), artifacts.ServerKey, 0600); err != nil {
		return fmt.Errorf("writing tls.key: %w", err)
	}

	return nil
}

// PatchWebhookCABundle patches the named ValidatingWebhookConfiguration with the
// CA certificate bundle so the Kubernetes API server trusts the webhook's TLS
// certificate. This is called automatically in self-signed mode and in manual mode
// (when ca.crt is present in the secret). It is not called in cert-manager mode.
func PatchWebhookCABundle(ctx context.Context, c client.Client, webhookConfigName string, caCert []byte) error {
	webhookConfig := &admissionregistrationv1.ValidatingWebhookConfiguration{}
	if err := c.Get(ctx, types.NamespacedName{Name: webhookConfigName}, webhookConfig); err != nil {
		return fmt.Errorf("getting ValidatingWebhookConfiguration %q: %w", webhookConfigName, err)
	}

	// Patch each webhook's CA bundle
	for i := range webhookConfig.Webhooks {
		webhookConfig.Webhooks[i].ClientConfig.CABundle = caCert
	}

	if err := c.Update(ctx, webhookConfig); err != nil {
		return fmt.Errorf("updating ValidatingWebhookConfiguration %q: %w", webhookConfigName, err)
	}

	return nil
}

// LoadCertsFromSecret reads TLS certificates from an existing Kubernetes secret.
// The secret must contain "tls.crt" and "tls.key" entries. If "ca.crt" is also
// present, it is returned as the CACert so the operator can patch the webhook
// caBundle. This supports enterprise cert rotation: update the secret, then
// restart the operator (or let controller-runtime's cert watcher detect changes).
func LoadCertsFromSecret(ctx context.Context, c client.Client, secretName, namespace string) (*CertificateArtifacts, error) {
	secret := &corev1.Secret{}
	if err := c.Get(ctx, types.NamespacedName{Name: secretName, Namespace: namespace}, secret); err != nil {
		return nil, fmt.Errorf("getting TLS secret %q in namespace %q: %w", secretName, namespace, err)
	}

	serverCert, hasCert := secret.Data["tls.crt"]
	serverKey, hasKey := secret.Data["tls.key"]

	if !hasCert || len(serverCert) == 0 {
		return nil, fmt.Errorf("TLS secret %q is missing or has empty 'tls.crt'", secretName)
	}
	if !hasKey || len(serverKey) == 0 {
		return nil, fmt.Errorf("TLS secret %q is missing or has empty 'tls.key'", secretName)
	}

	// ca.crt is optional — if present, operator will use it to patch caBundle
	caCert := secret.Data["ca.crt"]

	return &CertificateArtifacts{
		CACert:     caCert,
		ServerCert: serverCert,
		ServerKey:  serverKey,
	}, nil
}
