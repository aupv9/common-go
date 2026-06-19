package httpserver_test

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/aupv9/common-go/transport/httpserver"
	"github.com/aupv9/common-go/transport/httpserver/middleware"
	"github.com/gin-gonic/gin"
)

func newTestServer(t *testing.T, opts ...httpserver.Option) *httpserver.Server {
	t.Helper()
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	all := append([]httpserver.Option{httpserver.WithMode(gin.TestMode)}, opts...)
	srv, err := httpserver.New(log, all...)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return srv
}

func TestNewRequiresLogger(t *testing.T) {
	if _, err := httpserver.New(nil); err == nil {
		t.Fatal("expected error when logger is nil")
	}
}

func TestRequestIDGeneratedAndEchoed(t *testing.T) {
	srv := newTestServer(t)
	srv.Engine().GET("/ping", func(c *gin.Context) {
		if id := middleware.RequestIDFromGin(c); id == "" {
			t.Error("expected request id set in gin context")
		}
		c.String(http.StatusOK, "pong")
	})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/ping", nil)
	srv.Engine().ServeHTTP(rec, req)

	if got := rec.Header().Get(middleware.HeaderRequestID); got == "" {
		t.Error("expected X-Request-ID header in response")
	}
}

func TestRequestIDPropagatedFromHeader(t *testing.T) {
	srv := newTestServer(t)
	srv.Engine().GET("/ping", func(c *gin.Context) { c.Status(http.StatusOK) })

	const want = "caller-supplied-id"
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/ping", nil)
	req.Header.Set(middleware.HeaderRequestID, want)
	srv.Engine().ServeHTTP(rec, req)

	if got := rec.Header().Get(middleware.HeaderRequestID); got != want {
		t.Errorf("request id = %q, want %q", got, want)
	}
}

func TestRecoveryReturns500Envelope(t *testing.T) {
	srv := newTestServer(t)
	srv.Engine().GET("/boom", func(_ *gin.Context) { panic("boom") })

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/boom", nil)
	srv.Engine().ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "INTERNAL") {
		t.Errorf("body = %q, want it to contain error code INTERNAL", body)
	}
	if strings.Contains(body, "boom") {
		t.Errorf("panic detail leaked to client: %q", body)
	}
}

func TestCORSPreflight(t *testing.T) {
	srv := newTestServer(t, httpserver.WithCORS(middleware.CORSConfig{
		AllowedOrigins: []string{"https://app.example.com"},
	}))
	srv.Engine().POST("/x", func(c *gin.Context) { c.Status(http.StatusOK) })

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodOptions, "/x", nil)
	req.Header.Set("Origin", "https://app.example.com")
	srv.Engine().ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("preflight status = %d, want 204", rec.Code)
	}
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "https://app.example.com" {
		t.Errorf("allow-origin = %q", got)
	}
}

func TestCORSDisallowedOrigin(t *testing.T) {
	srv := newTestServer(t, httpserver.WithCORS(middleware.CORSConfig{
		AllowedOrigins: []string{"https://app.example.com"},
	}))
	srv.Engine().GET("/x", func(c *gin.Context) { c.Status(http.StatusOK) })

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	req.Header.Set("Origin", "https://evil.example.com")
	srv.Engine().ServeHTTP(rec, req)

	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Errorf("allow-origin should be empty for disallowed origin, got %q", got)
	}
}
