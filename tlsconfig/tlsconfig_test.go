package tlsconfig_test

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/aupv9/common-go/tlsconfig"
)

// writeCertPair sinh self-signed cert + key vào thư mục tạm, trả path.
func writeCertPair(t *testing.T) (certFile, keyFile string) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	tmpl := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "test"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
	}
	der, err := x509.CreateCertificate(rand.Reader, &tmpl, &tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	certFile = filepath.Join(dir, "cert.pem")
	keyFile = filepath.Join(dir, "key.pem")

	certPEM, _ := os.Create(certFile)
	_ = pem.Encode(certPEM, &pem.Block{Type: "CERTIFICATE", Bytes: der})
	_ = certPEM.Close()

	keyBytes, _ := x509.MarshalECPrivateKey(key)
	keyPEM, _ := os.Create(keyFile)
	_ = pem.Encode(keyPEM, &pem.Block{Type: "EC PRIVATE KEY", Bytes: keyBytes})
	_ = keyPEM.Close()
	return certFile, keyFile
}

func TestBuildDisabled(t *testing.T) {
	cfg, err := tlsconfig.Config{Enabled: false}.Build()
	if err != nil || cfg != nil {
		t.Fatalf("disabled Build = (%v, %v), want (nil, nil)", cfg, err)
	}
}

func TestBuildTLS(t *testing.T) {
	cert, key := writeCertPair(t)
	cfg, err := tlsconfig.Config{Enabled: true, CertFile: cert, KeyFile: key}.Build()
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if len(cfg.Certificates) != 1 {
		t.Errorf("certificates = %d, want 1", len(cfg.Certificates))
	}
	if cfg.MinVersion == 0 {
		t.Error("MinVersion not set")
	}
}

func TestBuildMTLS(t *testing.T) {
	cert, key := writeCertPair(t)
	caCert, _ := writeCertPair(t) // dùng làm client CA
	cfg, err := tlsconfig.Config{Enabled: true, CertFile: cert, KeyFile: key, ClientCAFile: caCert}.Build()
	if err != nil {
		t.Fatalf("Build mTLS: %v", err)
	}
	if cfg.ClientCAs == nil {
		t.Error("ClientCAs not set for mTLS")
	}
}

func TestBuildMissingFiles(t *testing.T) {
	if _, err := (tlsconfig.Config{Enabled: true}).Build(); err == nil {
		t.Error("expected error when cert/key missing")
	}
}
