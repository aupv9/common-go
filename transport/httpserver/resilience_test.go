package httpserver_test

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/aupv9/common-go/transport/httpserver"
	"github.com/aupv9/common-go/transport/httpserver/middleware"
	"github.com/gin-gonic/gin"
)

func TestSecurityHeaders(t *testing.T) {
	srv := newTestServer(t, httpserver.WithSecurityHeaders(false))
	srv.Engine().GET("/x", func(c *gin.Context) { c.Status(http.StatusOK) })

	rec := httptest.NewRecorder()
	srv.Engine().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/x", nil))
	if rec.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Error("missing X-Content-Type-Options")
	}
	if rec.Header().Get("X-Frame-Options") != "DENY" {
		t.Error("missing X-Frame-Options")
	}
}

func TestRateLimit(t *testing.T) {
	srv := newTestServer(t, httpserver.WithRateLimit(middleware.RateLimitConfig{
		RPS:   1,
		Burst: 2,
		TTL:   time.Minute,
	}))
	srv.Engine().GET("/x", func(c *gin.Context) { c.Status(http.StatusOK) })

	codes := make([]int, 0, 4)
	for i := 0; i < 4; i++ {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/x", nil)
		req.RemoteAddr = "10.0.0.1:1234"
		srv.Engine().ServeHTTP(rec, req)
		codes = append(codes, rec.Code)
	}
	// Burst 2 → 2 request đầu OK, sau đó 429.
	if codes[0] != 200 || codes[1] != 200 {
		t.Errorf("first two = %v, want 200,200", codes[:2])
	}
	if codes[3] != http.StatusTooManyRequests {
		t.Errorf("4th = %d, want 429", codes[3])
	}
}

func TestBodyLimit(t *testing.T) {
	srv := newTestServer(t, httpserver.WithBodyLimit(8))
	srv.Engine().POST("/x", func(c *gin.Context) {
		if _, err := io.ReadAll(c.Request.Body); err != nil {
			c.Status(http.StatusRequestEntityTooLarge)
			return
		}
		c.Status(http.StatusOK)
	})

	// Body 16 byte > giới hạn 8 → handler đọc lỗi → 413.
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/x", strings.NewReader("0123456789ABCDEF"))
	srv.Engine().ServeHTTP(rec, req)
	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Errorf("over-limit status = %d, want 413", rec.Code)
	}

	// Body 4 byte < giới hạn → 200.
	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/x", strings.NewReader("0123"))
	srv.Engine().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("under-limit status = %d, want 200", rec.Code)
	}
}
