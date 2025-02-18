package util

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"time"
)

// LoadOrGenerateCert generates a self-signed certificate and private key if they do not exist at the specified paths.
// If the files already exist, they are loaded and returned.
func LoadOrGenerateCert(certPath, keyPath string) (tls.Certificate, error) {
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("failed to generate private key: %v", err)
	}

	template := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			Organization: []string{"Self-Signed"},
		},
		NotBefore: time.Now(),
		NotAfter:  time.Now().Add(365 * 24 * time.Hour),

		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
	}

	derBytes, err := x509.CreateCertificate(rand.Reader, &template, &template, &priv.PublicKey, priv)
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("failed to create certificate: %v", err)
	}

	cert := tls.Certificate{
		Certificate: [][]byte{derBytes},
		PrivateKey:  priv,
	}

	// Ensure the directory exists
	err = os.MkdirAll(filepath.Dir(certPath), os.ModePerm)
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("failed to create tls directory: %v", err)
	}

	// Write the cert and key to files
	err = writeCert(certPath, derBytes)
	if err != nil {
		return tls.Certificate{}, err
	}

	err = writeKey(keyPath, priv)
	if err != nil {
		return tls.Certificate{}, err
	}

	return cert, nil
}

// LoadCertFromFiles loads a TLS certificate from the specified paths.
func loadCertFromFiles(certPath, keyPath string) (tls.Certificate, error) {
	cert, err := tls.LoadX509KeyPair(certPath, keyPath)
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("failed to load TLS certificate: %v", err)
	}
	return cert, nil
}

func writeCert(certPath string, derBytes []byte) error {
	certOut, err := os.Create(certPath)
	if err != nil {
		return fmt.Errorf("failed to create cert file: %v", err)
	}
	defer certOut.Close()

	err = pem.Encode(certOut, &pem.Block{Type: "CERTIFICATE", Bytes: derBytes})
	if err != nil {
		return fmt.Errorf("failed to write certificate to file: %v", err)
	}
	return nil
}

func writeKey(keyPath string, priv *ecdsa.PrivateKey) error {
	keyOut, err := os.Create(keyPath)
	if err != nil {
		return fmt.Errorf("failed to create key file: %v", err)
	}
	defer keyOut.Close()

	privBytes, err := x509.MarshalECPrivateKey(priv)
	if err != nil {
		return fmt.Errorf("failed to marshal private key: %v", err)
	}

	err = pem.Encode(keyOut, &pem.Block{Type: "EC PRIVATE KEY", Bytes: privBytes})
	if err != nil {
		return fmt.Errorf("failed to write private key to file: %v", err)
	}
	return nil
}
