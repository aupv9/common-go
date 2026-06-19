package middleware

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

func TestRequestIDGenerateAndPropagate(t *testing.T) {
	e := newEngine(RequestID())
	e.GET("/x", func(c *gin.Context) {
		if RequestIDFromGin(c) == "" {
			t.Error("request id not set in gin context")
		}
		c.Status(200)
	})

	// Sinh mới khi không có header.
	rec := do(e, http.MethodGet, "/x")
	if rec.Header().Get(HeaderRequestID) == "" {
		t.Error("expected generated X-Request-ID in response")
	}

	// Propagate khi client gửi.
	rec2 := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	req.Header.Set(HeaderRequestID, "incoming-id")
	e.ServeHTTP(rec2, req)
	if rec2.Header().Get(HeaderRequestID) != "incoming-id" {
		t.Errorf("request id = %q, want incoming-id", rec2.Header().Get(HeaderRequestID))
	}
}

func TestInMemoryRateLimit(t *testing.T) {
	e := newEngine(RateLimit(RateLimitConfig{RPS: 1, Burst: 2}))
	e.GET("/x", func(c *gin.Context) { c.Status(200) })

	codes := make([]int, 0, 4)
	for i := 0; i < 4; i++ {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/x", nil)
		req.RemoteAddr = "10.1.2.3:5555"
		e.ServeHTTP(rec, req)
		codes = append(codes, rec.Code)
	}
	if codes[0] != 200 || codes[1] != 200 {
		t.Errorf("first two = %v, want 200,200", codes[:2])
	}
	if codes[3] != http.StatusTooManyRequests {
		t.Errorf("4th = %d, want 429", codes[3])
	}
}

func TestBodyLimitMiddleware(t *testing.T) {
	e := newEngine(BodyLimit(8))
	e.POST("/x", func(c *gin.Context) {
		if _, err := io.ReadAll(c.Request.Body); err != nil {
			c.Status(http.StatusRequestEntityTooLarge)
			return
		}
		c.Status(200)
	})
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/x", strings.NewReader("0123456789ABCDEF"))
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Errorf("status = %d, want 413", rec.Code)
	}
}

func TestCORSCredentialsAndVary(t *testing.T) {
	e := newEngine(CORS(CORSConfig{
		AllowedOrigins:   []string{"https://app.example.com"},
		AllowCredentials: true,
	}))
	e.GET("/x", func(c *gin.Context) { c.Status(200) })

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	req.Header.Set("Origin", "https://app.example.com")
	e.ServeHTTP(rec, req)

	if rec.Header().Get("Access-Control-Allow-Origin") != "https://app.example.com" {
		t.Errorf("allow-origin = %q", rec.Header().Get("Access-Control-Allow-Origin"))
	}
	if rec.Header().Get("Access-Control-Allow-Credentials") != "true" {
		t.Error("missing allow-credentials")
	}
	if !strings.Contains(rec.Header().Get("Vary"), "Origin") {
		t.Errorf("Vary should contain Origin, got %q", rec.Header().Get("Vary"))
	}
}

func TestCORSWildcard(t *testing.T) {
	e := newEngine(CORS(CORSConfig{AllowedOrigins: []string{"*"}}))
	e.GET("/x", func(c *gin.Context) { c.Status(200) })
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	req.Header.Set("Origin", "https://anything.example")
	e.ServeHTTP(rec, req)
	if rec.Header().Get("Access-Control-Allow-Origin") != "*" {
		t.Errorf("wildcard allow-origin = %q", rec.Header().Get("Access-Control-Allow-Origin"))
	}
}

func TestDefaultCORSConfig(t *testing.T) {
	c := DefaultCORSConfig()
	if len(c.AllowedMethods) == 0 || len(c.AllowedHeaders) == 0 {
		t.Error("default CORS should have methods/headers")
	}
	if c.MaxAge != 12*time.Hour {
		t.Errorf("max age = %v", c.MaxAge)
	}
}
