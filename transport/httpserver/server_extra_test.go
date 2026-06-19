package httpserver_test

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/aupv9/common-go/tlsconfig"
	"github.com/aupv9/common-go/transport/httpserver"
	"github.com/gin-gonic/gin"
)

func freePort(t *testing.T) int {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	p := l.Addr().(*net.TCPAddr).Port
	_ = l.Close()
	return p
}

// TestListenBindError xác minh A1: lỗi bind cổng được trả về ĐỒNG BỘ (không nuốt).
func TestListenBindError(t *testing.T) {
	port := freePort(t)
	srv1 := newTestServer(t, httpserver.WithAddr("127.0.0.1", port))
	lis1, err := srv1.Listen()
	if err != nil {
		t.Fatalf("first Listen: %v", err)
	}
	defer lis1.Close()

	srv2 := newTestServer(t, httpserver.WithAddr("127.0.0.1", port))
	if _, err := srv2.Listen(); err == nil {
		t.Fatal("second Listen on busy port should fail (bind error must surface)")
	}
}

func TestServeAndShutdown(t *testing.T) {
	port := freePort(t)
	srv := newTestServer(t, httpserver.WithAddr("127.0.0.1", port))
	srv.Engine().GET("/ping", func(c *gin.Context) { c.String(http.StatusOK, "pong") })

	lis, err := srv.Listen()
	if err != nil {
		t.Fatalf("Listen: %v", err)
	}
	done := make(chan error, 1)
	go func() { done <- srv.Serve(lis) }()

	// chờ server sẵn sàng
	waitHTTP(t, srv.Addr())
	resp, err := http.Get("http://" + srv.Addr() + "/ping")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d", resp.StatusCode)
	}

	if err := srv.Shutdown(context.Background()); err != nil {
		t.Errorf("Shutdown: %v", err)
	}
	if err := <-done; err != nil {
		t.Errorf("Serve returned error: %v", err)
	}
}

func TestServeTLS(t *testing.T) {
	cert, key := writeCert(t)
	port := freePort(t)
	srv := newTestServer(t,
		httpserver.WithAddr("127.0.0.1", port),
		httpserver.WithTLS(tlsconfig.Config{Enabled: true, CertFile: cert, KeyFile: key}),
	)
	srv.Engine().GET("/ping", func(c *gin.Context) { c.String(http.StatusOK, "pong") })

	lis, err := srv.Listen()
	if err != nil {
		t.Fatalf("Listen: %v", err)
	}
	go func() { _ = srv.Serve(lis) }()
	t.Cleanup(func() { _ = srv.Shutdown(context.Background()) })
	waitHTTP(t, srv.Addr())

	client := &http.Client{Transport: &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}}}
	resp, err := client.Get("https://" + srv.Addr() + "/ping")
	if err != nil {
		t.Fatalf("HTTPS GET: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d", resp.StatusCode)
	}
}

func waitHTTP(t *testing.T, addr string) {
	t.Helper()
	for i := 0; i < 50; i++ {
		c, err := net.DialTimeout("tcp", addr, 100*time.Millisecond)
		if err == nil {
			_ = c.Close()
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("server at %s not ready", addr)
}

func writeCert(t *testing.T) (certFile, keyFile string) {
	t.Helper()
	key, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	tmpl := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "localhost"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
		IPAddresses:  []net.IP{net.ParseIP("127.0.0.1")},
	}
	der, _ := x509.CreateCertificate(rand.Reader, &tmpl, &tmpl, &key.PublicKey, key)
	dir := t.TempDir()
	certFile = filepath.Join(dir, "c.pem")
	keyFile = filepath.Join(dir, "k.pem")
	cf, _ := os.Create(certFile)
	_ = pem.Encode(cf, &pem.Block{Type: "CERTIFICATE", Bytes: der})
	_ = cf.Close()
	kb, _ := x509.MarshalECPrivateKey(key)
	kf, _ := os.Create(keyFile)
	_ = pem.Encode(kf, &pem.Block{Type: "EC PRIVATE KEY", Bytes: kb})
	_ = kf.Close()
	return certFile, keyFile
}
