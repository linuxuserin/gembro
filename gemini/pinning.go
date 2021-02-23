package gemini

import (
	"crypto/sha256"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sync"
)

var NotFound = errors.New("cert not found")
var CertChanged = errors.New("cert has changed")

type CertStore struct {
	lock         sync.Mutex
	Certificates map[string]string
	savePath     string
}

func fingerprint(cert *x509.Certificate) string {
	hasher := sha256.New()
	_, _ = hasher.Write(cert.Raw)
	return fmt.Sprintf("%x", hasher.Sum(nil))
}

func (cs *CertStore) pin(host string, cert *x509.Certificate) error {
	if cs.Certificates == nil {
		cs.Certificates = make(map[string]string)
	}
	cs.Certificates[host] = fingerprint(cert)
	return cs.save()
}

func (cs *CertStore) Check(host string, cert *x509.Certificate, skipVerify bool) error {
	cs.lock.Lock()
	defer cs.lock.Unlock()

	c, ok := cs.Certificates[host]
	if !ok || skipVerify {
		return cs.pin(host, cert)
	}

	if c != fingerprint(cert) {
		return CertChanged
	}
	return nil
}

func (cs *CertStore) save() error {
	f, err := os.Create(cs.savePath)
	if err != nil {
		return fmt.Errorf("could not open file file for writing: %w", err)
	}
	defer f.Close()
	if err := json.NewEncoder(f).Encode(cs); err != nil {
		return fmt.Errorf("could not encode certs: %w", err)
	}
	return nil
}

func Load(savePath string) (*CertStore, error) {
	f, err := os.Open(savePath)
	if err != nil {
		if os.IsNotExist(err) {
			return &CertStore{savePath: savePath}, nil
		}
		return nil, fmt.Errorf("could not open cert file for reading: %w", err)
	}
	defer f.Close()
	cs := CertStore{savePath: savePath}
	if err := json.NewDecoder(f).Decode(&cs); err != nil {
		return nil, fmt.Errorf("could not decode certs: %w", err)
	}
	return &cs, nil
}
