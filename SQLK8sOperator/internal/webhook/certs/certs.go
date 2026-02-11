/*
Copyright 2026 Microsoft Corporation.

Licensed under the MIT License.
See LICENSE file in the project root for full license information.
*/

// Package certs provides self-signed TLS certificate generation for the
// operator's admission webhook server. Certificates are generated at startup
// and the CA bundle is patched into the ValidatingWebhookConfiguration so the
// Kubernetes API server trusts the webhook endpoint.
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
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

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
// CA certificate bundle so the Kubernetes API server trusts the webhook's self-signed
// certificate. This must be called after the webhook configuration is deployed.
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
