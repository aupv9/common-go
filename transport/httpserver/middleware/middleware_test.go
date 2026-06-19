package middleware

import (
	"bytes"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/aupv9/common-go/logger"
	"github.com/gin-gonic/gin"
)

func newEngine(mw ...gin.HandlerFunc) *gin.Engine {
	gin.SetMode(gin.TestMode)
	e := gin.New()
	e.Use(mw...)
	return e
}

func do(e *gin.Engine, method, path string) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, httptest.NewRequest(method, path, nil))
	return rec
}

func TestContextLoggerInjects(t *testing.T) {
	base := slog.New(slog.NewTextHandler(nil, nil))
	e := newEngine(ContextLogger(base))
	var got bool
	e.GET("/x", func(c *gin.Context) {
		got = logger.FromContext(c.Request.Context()) == base
		c.Status(200)
	})
	do(e, http.MethodGet, "/x")
	if !got {
		t.Error("ContextLogger did not inject base logger into request context")
	}
}

func TestRecoveryReturns500NoLeak(t *testing.T) {
	var buf bytes.Buffer
	base := slog.New(slog.NewJSONHandler(&buf, nil))
	e := newEngine(ContextLogger(base), Recovery())
	e.GET("/boom", func(_ *gin.Context) { panic("secret-detail") })

	rec := do(e, http.MethodGet, "/boom")
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", rec.Code)
	}
	if strings.Contains(rec.Body.String(), "secret-detail") {
		t.Errorf("panic detail leaked to client: %s", rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "INTERNAL") {
		t.Errorf("expected INTERNAL error code, got %s", rec.Body.String())
	}
	// Stack được log server-side.
	if !strings.Contains(buf.String(), "panic") {
		t.Errorf("expected panic logged server-side")
	}
}

func TestTimeoutSetsDeadline(t *testing.T) {
	e := newEngine(Timeout(50 * time.Millisecond))
	var hasDeadline bool
	e.GET("/x", func(c *gin.Context) {
		_, hasDeadline = c.Request.Context().Deadline()
		c.Status(200)
	})
	do(e, http.MethodGet, "/x")
	if !hasDeadline {
		t.Error("Timeout did not set a deadline on request context")
	}
}

func TestTimeoutZeroNoop(t *testing.T) {
	e := newEngine(Timeout(0))
	var hasDeadline bool
	e.GET("/x", func(c *gin.Context) {
		_, hasDeadline = c.Request.Context().Deadline()
		c.Status(200)
	})
	do(e, http.MethodGet, "/x")
	if hasDeadline {
		t.Error("Timeout(0) should be no-op")
	}
}

func TestSecurityHeaders(t *testing.T) {
	e := newEngine(SecurityHeaders(true))
	e.GET("/x", func(c *gin.Context) { c.Status(200) })
	rec := do(e, http.MethodGet, "/x")
	h := rec.Header()
	if h.Get("X-Content-Type-Options") != "nosniff" {
		t.Error("missing nosniff")
	}
	if h.Get("Strict-Transport-Security") == "" {
		t.Error("missing HSTS when enabled")
	}

	e2 := newEngine(SecurityHeaders(false))
	e2.GET("/x", func(c *gin.Context) { c.Status(200) })
	rec2 := do(e2, http.MethodGet, "/x")
	if rec2.Header().Get("Strict-Transport-Security") != "" {
		t.Error("HSTS should be absent when disabled")
	}
}

func TestAccessLogLevels(t *testing.T) {
	cases := []struct {
		status    int
		wantLevel string
	}{
		{200, "INFO"}, {404, "WARN"}, {500, "ERROR"},
	}
	for _, tc := range cases {
		var buf bytes.Buffer
		base := slog.New(slog.NewJSONHandler(&buf, nil))
		e := newEngine(ContextLogger(base), AccessLog("/skip"))
		e.GET("/x", func(c *gin.Context) { c.Status(tc.status) })
		do(e, http.MethodGet, "/x")
		if !strings.Contains(buf.String(), `"level":"`+tc.wantLevel+`"`) {
			t.Errorf("status %d: want level %s, log=%s", tc.status, tc.wantLevel, buf.String())
		}
	}
}

func TestAccessLogSkip(t *testing.T) {
	var buf bytes.Buffer
	base := slog.New(slog.NewJSONHandler(&buf, nil))
	e := newEngine(ContextLogger(base), AccessLog("/skip"))
	e.GET("/skip", func(c *gin.Context) { c.Status(200) })
	do(e, http.MethodGet, "/skip")
	if strings.Contains(buf.String(), "http request") {
		t.Errorf("skip path should not be access-logged: %s", buf.String())
	}
}

func TestRequestIDFromGinEmpty(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodGet, "/", nil)
	if RequestIDFromGin(c) != "" {
		t.Error("expected empty request id when unset")
	}
}
