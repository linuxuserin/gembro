package gemini

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
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

func (cs *CertStore) pin(host string, cert *x509.Certificate) error {
	data, err := x509.MarshalPKIXPublicKey(cert.PublicKey)
	if err != nil {
		return fmt.Errorf("could not marshal pubKey: %w", err)
	}
	if cs.Certificates == nil {
		cs.Certificates = make(map[string]string)
	}
	cs.Certificates[host] = base64.StdEncoding.EncodeToString(data)
	return cs.save()
}

func (cs *CertStore) Check(host string, cert *x509.Certificate, skipVerify bool) error {
	cs.lock.Lock()
	defer cs.lock.Unlock()

	c, ok := cs.Certificates[host]
	if !ok || skipVerify {
		return cs.pin(host, cert)
	}
	s, err := base64.StdEncoding.DecodeString(c)
	if err != nil {
		return fmt.Errorf("could not base64 decode cert: %w", err)
	}

	sPubKey, err := x509.ParsePKIXPublicKey(s)
	if err != nil {
		return fmt.Errorf("could not parse saved pubKey: %w", err)
	}

	var check bool
	switch sPubKey := sPubKey.(type) {
	case *rsa.PublicKey:
		check = sPubKey.Equal(cert.PublicKey.(crypto.PublicKey))
	case *ecdsa.PublicKey:
		check = sPubKey.Equal(cert.PublicKey.(crypto.PublicKey))
	case *ed25519.PublicKey:
		check = sPubKey.Equal(cert.PublicKey.(crypto.PublicKey))
	default:
		return fmt.Errorf("unknown public key")
	}

	if !check {
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
