package gemini

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"io/ioutil"
)

func GenerateClientCertificate(certFile, keyFile string, config x509.Certificate) error {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return fmt.Errorf("Private key cannot be created: %w", err)
	}

	// Generate a pem block with the private key
	keyPem := pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(key),
	})

	cert, err := x509.CreateCertificate(rand.Reader, &config, &config, &key.PublicKey, key)
	if err != nil {
		return fmt.Errorf("Certificate cannot be created: %w", err)
	}

	// Generate a pem block with the certificate
	certPem := pem.EncodeToMemory(&pem.Block{
		Type:  "CERTIFICATE",
		Bytes: cert,
	})

	if err := ioutil.WriteFile(keyFile, keyPem, 0600); err != nil {
		return fmt.Errorf("error writing key file: %w", err)
	}
	if err := ioutil.WriteFile(certFile, certPem, 0644); err != nil {
		return fmt.Errorf("error writing certificate file: %w", err)
	}
	return nil
}
