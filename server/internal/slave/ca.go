package slave

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"time"
)

// CertificateAuthority manages slave certificates
type CertificateAuthority struct {
	caCert   *x509.Certificate
	caKey    *rsa.PrivateKey
	certPath string
	keyPath  string
}

// NewCertificateAuthority creates or loads a certificate authority
func NewCertificateAuthority(dataDir string) (*CertificateAuthority, error) {
	certPath := filepath.Join(dataDir, "slave-ca-cert.pem")
	keyPath := filepath.Join(dataDir, "slave-ca-key.pem")

	// Try to load existing CA
	if fileExists(certPath) && fileExists(keyPath) {
		return loadCA(certPath, keyPath)
	}

	// Generate new CA
	return generateCA(certPath, keyPath)
}

// generateCA creates a new certificate authority
func generateCA(certPath, keyPath string) (*CertificateAuthority, error) {
	// Generate private key
	caKey, err := rsa.GenerateKey(rand.Reader, 4096)
	if err != nil {
		return nil, fmt.Errorf("failed to generate CA key: %w", err)
	}

	// Create CA certificate template
	serialNumber, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return nil, fmt.Errorf("failed to generate serial number: %w", err)
	}

	caCertTemplate := &x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			Organization: []string{"Diane"},
			CommonName:   "Diane Slave CA",
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().AddDate(10, 0, 0), // Valid for 10 years
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
		BasicConstraintsValid: true,
		IsCA:                  true,
	}

	// Self-sign the CA certificate
	caCertDER, err := x509.CreateCertificate(rand.Reader, caCertTemplate, caCertTemplate, &caKey.PublicKey, caKey)
	if err != nil {
		return nil, fmt.Errorf("failed to create CA certificate: %w", err)
	}

	// Parse the certificate
	caCert, err := x509.ParseCertificate(caCertDER)
	if err != nil {
		return nil, fmt.Errorf("failed to parse CA certificate: %w", err)
	}

	// Save certificate
	certFile, err := os.OpenFile(certPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		return nil, fmt.Errorf("failed to create cert file: %w", err)
	}
	defer certFile.Close()

	if err := pem.Encode(certFile, &pem.Block{Type: "CERTIFICATE", Bytes: caCertDER}); err != nil {
		return nil, fmt.Errorf("failed to write cert file: %w", err)
	}

	// Save private key
	keyFile, err := os.OpenFile(keyPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return nil, fmt.Errorf("failed to create key file: %w", err)
	}
	defer keyFile.Close()

	keyBytes := x509.MarshalPKCS1PrivateKey(caKey)
	if err := pem.Encode(keyFile, &pem.Block{Type: "RSA PRIVATE KEY", Bytes: keyBytes}); err != nil {
		return nil, fmt.Errorf("failed to write key file: %w", err)
	}

	return &CertificateAuthority{
		caCert:   caCert,
		caKey:    caKey,
		certPath: certPath,
		keyPath:  keyPath,
	}, nil
}

// loadCA loads an existing certificate authority
func loadCA(certPath, keyPath string) (*CertificateAuthority, error) {
	// Load certificate
	certPEM, err := os.ReadFile(certPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read cert file: %w", err)
	}

	certBlock, _ := pem.Decode(certPEM)
	if certBlock == nil {
		return nil, fmt.Errorf("failed to decode cert PEM")
	}

	caCert, err := x509.ParseCertificate(certBlock.Bytes)
	if err != nil {
		return nil, fmt.Errorf("failed to parse certificate: %w", err)
	}

	// Load private key
	keyPEM, err := os.ReadFile(keyPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read key file: %w", err)
	}

	keyBlock, _ := pem.Decode(keyPEM)
	if keyBlock == nil {
		return nil, fmt.Errorf("failed to decode key PEM")
	}

	caKey, err := x509.ParsePKCS1PrivateKey(keyBlock.Bytes)
	if err != nil {
		return nil, fmt.Errorf("failed to parse private key: %w", err)
	}

	return &CertificateAuthority{
		caCert:   caCert,
		caKey:    caKey,
		certPath: certPath,
		keyPath:  keyPath,
	}, nil
}

// SignCSR signs a certificate signing request
func (ca *CertificateAuthority) SignCSR(csrPEM []byte, hostID string, validDays int) (certPEM []byte, serialNumber string, err error) {
	// Decode CSR
	csrBlock, _ := pem.Decode(csrPEM)
	if csrBlock == nil {
		return nil, "", fmt.Errorf("failed to decode CSR PEM")
	}

	csr, err := x509.ParseCertificateRequest(csrBlock.Bytes)
	if err != nil {
		return nil, "", fmt.Errorf("failed to parse CSR: %w", err)
	}

	// Verify CSR signature
	if err := csr.CheckSignature(); err != nil {
		return nil, "", fmt.Errorf("invalid CSR signature: %w", err)
	}

	// Generate serial number
	serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return nil, "", fmt.Errorf("failed to generate serial number: %w", err)
	}

	// Create certificate template
	certTemplate := &x509.Certificate{
		SerialNumber: serial,
		Subject:      csr.Subject,
		NotBefore:    time.Now(),
		NotAfter:     time.Now().AddDate(0, 0, validDays),
		KeyUsage:     x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
	}

	// Sign the certificate
	certDER, err := x509.CreateCertificate(rand.Reader, certTemplate, ca.caCert, csr.PublicKey, ca.caKey)
	if err != nil {
		return nil, "", fmt.Errorf("failed to create certificate: %w", err)
	}

	// Encode to PEM
	certPEM = pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})
	serialNumber = serial.Text(16)

	return certPEM, serialNumber, nil
}

// GetCACertPEM returns the CA certificate in PEM format
func (ca *CertificateAuthority) GetCACertPEM() ([]byte, error) {
	return os.ReadFile(ca.certPath)
}

// VerifyClientCert verifies a client certificate against the CA
func (ca *CertificateAuthority) VerifyClientCert(certPEM []byte) (*x509.Certificate, error) {
	// Decode certificate
	certBlock, _ := pem.Decode(certPEM)
	if certBlock == nil {
		return nil, fmt.Errorf("failed to decode cert PEM")
	}

	cert, err := x509.ParseCertificate(certBlock.Bytes)
	if err != nil {
		return nil, fmt.Errorf("failed to parse certificate: %w", err)
	}

	// Create cert pool with CA
	certPool := x509.NewCertPool()
	certPool.AddCert(ca.caCert)

	// Verify certificate
	opts := x509.VerifyOptions{
		Roots:     certPool,
		KeyUsages: []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
	}

	if _, err := cert.Verify(opts); err != nil {
		return nil, fmt.Errorf("certificate verification failed: %w", err)
	}

	// Check expiration
	now := time.Now()
	if now.Before(cert.NotBefore) {
		return nil, fmt.Errorf("certificate not yet valid")
	}
	if now.After(cert.NotAfter) {
		return nil, fmt.Errorf("certificate expired")
	}

	return cert, nil
}

// GetCertificate returns the CA certificate
func (ca *CertificateAuthority) GetCertificate() (*x509.Certificate, error) {
	if ca.caCert == nil {
		return nil, fmt.Errorf("CA certificate not loaded")
	}
	return ca.caCert, nil
}

// GetPaths returns the paths to the CA certificate and key files
func (ca *CertificateAuthority) GetPaths() (certPath, keyPath string, err error) {
	if ca.certPath == "" || ca.keyPath == "" {
		return "", "", fmt.Errorf("CA paths not set")
	}
	return ca.certPath, ca.keyPath, nil
}

// fileExists checks if a file exists
func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
