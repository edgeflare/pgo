package util

import (
	"os"
	"path/filepath"
	"testing"
)

// TestGenerateSelfSignedCert tests the LoadOrGenerateCert function for correctness.
func TestGenerateSelfSignedCert(t *testing.T) {
	certPath := "./test_tls/tls.crt"
	keyPath := "./test_tls/tls.key"

	// Clean up the test files after the test
	defer func() {
		os.Remove(certPath)
		os.Remove(keyPath)
		os.Remove(filepath.Dir(certPath))
	}()

	cert, err := LoadOrGenerateCert(certPath, keyPath)
	if err != nil {
		t.Fatalf("LoadOrGenerateCert() failed: %v", err)
	}

	// Check if the certificate and key files were created
	if _, err := os.Stat(certPath); os.IsNotExist(err) {
		t.Errorf("Expected certificate file %s does not exist", certPath)
	}

	if _, err := os.Stat(keyPath); os.IsNotExist(err) {
		t.Errorf("Expected key file %s does not exist", keyPath)
	}

	// Validate the certificate structure
	if len(cert.Certificate) == 0 {
		t.Errorf("Generated certificate has no data")
	}

	// Validate the private key structure
	if cert.PrivateKey == nil {
		t.Errorf("Generated certificate has no private key")
	}
}

// TestLoadCertFromFiles tests the LoadCertFromFiles function for correctness.
func TestLoadCertFromFiles(t *testing.T) {
	certPath := "./test_tls/tls.crt"
	keyPath := "./test_tls/tls.key"

	// Generate a self-signed certificate to be loaded later
	_, err := LoadOrGenerateCert(certPath, keyPath)
	if err != nil {
		t.Fatalf("LoadOrGenerateCert() failed: %v", err)
	}

	// Clean up the test files after the test
	defer func() {
		os.Remove(certPath)
		os.Remove(keyPath)
		os.Remove(filepath.Dir(certPath))
	}()

	cert, err := loadCertFromFiles(certPath, keyPath)
	if err != nil {
		t.Fatalf("LoadCertFromFiles() failed: %v", err)
	}

	// Validate the loaded certificate
	if len(cert.Certificate) == 0 {
		t.Errorf("Loaded certificate has no data")
	}

	// Validate the private key structure
	if cert.PrivateKey == nil {
		t.Errorf("Loaded certificate has no private key")
	}
}

// TestGenerateSelfSignedCert_DirectoryCreation tests the directory creation for the certificate and key files.
func TestGenerateSelfSignedCert_DirectoryCreation(t *testing.T) {
	certPath := "./test_tls_subdir/tls.crt"
	keyPath := "./test_tls_subdir/tls.key"

	// Clean up the test files after the test
	defer func() {
		os.Remove(certPath)
		os.Remove(keyPath)
		os.Remove(filepath.Dir(certPath))
	}()

	cert, err := LoadOrGenerateCert(certPath, keyPath)
	if err != nil {
		t.Fatalf("LoadOrGenerateCert() failed: %v", err)
	}

	// Check if the directory was created
	if _, err := os.Stat(filepath.Dir(certPath)); os.IsNotExist(err) {
		t.Errorf("Expected directory %s does not exist", filepath.Dir(certPath))
	}

	// Check if the certificate and key files were created
	if _, err := os.Stat(certPath); os.IsNotExist(err) {
		t.Errorf("Expected certificate file %s does not exist", certPath)
	}

	if _, err := os.Stat(keyPath); os.IsNotExist(err) {
		t.Errorf("Expected key file %s does not exist", keyPath)
	}

	// Validate the certificate structure
	if len(cert.Certificate) == 0 {
		t.Errorf("Generated certificate has no data")
	}

	// Validate the private key structure
	if cert.PrivateKey == nil {
		t.Errorf("Generated certificate has no private key")
	}
}

// TestLoadCertFromFiles_InvalidPath tests loading certificates from invalid file paths.
func TestLoadCertFromFiles_InvalidPath(t *testing.T) {
	invalidCertPath := "./test_tls/invalid.crt"
	invalidKeyPath := "./test_tls/invalid.key"

	_, err := loadCertFromFiles(invalidCertPath, invalidKeyPath)
	if err == nil {
		t.Error("Expected error loading certificate from invalid paths, got nil")
	}
}
