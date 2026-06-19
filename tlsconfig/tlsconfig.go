// Package tlsconfig dựng *tls.Config cho server (HTTP/gRPC) từ đường dẫn file,
// hỗ trợ cả TLS thường lẫn mTLS (xác minh client cert). Là package lá: chỉ dùng
// stdlib.
package tlsconfig

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"os"
)

// Config cấu hình TLS cho server.
type Config struct {
	Enabled  bool
	CertFile string // server certificate (PEM)
	KeyFile  string // server private key (PEM)
	// ClientCAFile: set → bật mTLS (yêu cầu + xác minh client cert bằng CA này).
	ClientCAFile string
}

// Build trả *tls.Config dựng từ Config; trả (nil, nil) nếu không Enabled.
// MinVersion TLS 1.2. Nếu có ClientCAFile thì ClientAuth =
// RequireAndVerifyClientCert (mTLS).
func (c Config) Build() (*tls.Config, error) {
	if !c.Enabled {
		return nil, nil
	}
	if c.CertFile == "" || c.KeyFile == "" {
		return nil, fmt.Errorf("tlsconfig: cert_file and key_file are required when TLS enabled")
	}

	cert, err := tls.LoadX509KeyPair(c.CertFile, c.KeyFile)
	if err != nil {
		return nil, fmt.Errorf("tlsconfig: load key pair: %w", err)
	}

	cfg := &tls.Config{
		MinVersion:   tls.VersionTLS12,
		Certificates: []tls.Certificate{cert},
	}

	if c.ClientCAFile != "" {
		pem, err := os.ReadFile(c.ClientCAFile)
		if err != nil {
			return nil, fmt.Errorf("tlsconfig: read client CA: %w", err)
		}
		pool := x509.NewCertPool()
		if !pool.AppendCertsFromPEM(pem) {
			return nil, fmt.Errorf("tlsconfig: no valid certs in client CA %q", c.ClientCAFile)
		}
		cfg.ClientCAs = pool
		cfg.ClientAuth = tls.RequireAndVerifyClientCert
	}

	return cfg, nil
}

// MTLS cho biết có bật xác minh client cert hay không.
func (c Config) MTLS() bool { return c.Enabled && c.ClientCAFile != "" }
