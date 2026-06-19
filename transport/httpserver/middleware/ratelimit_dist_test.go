package middleware

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

type fakeLimiter struct {
	allow bool
	err   error
}

func (f fakeLimiter) Allow(context.Context, string) (bool, error) { return f.allow, f.err }

func runReq(h gin.HandlerFunc) int {
	gin.SetMode(gin.TestMode)
	e := gin.New()
	e.Use(h)
	e.GET("/x", func(c *gin.Context) { c.Status(http.StatusOK) })
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/x", nil))
	return rec.Code
}

func TestRateLimitWithExternalLimiterAllow(t *testing.T) {
	code := runReq(RateLimit(RateLimitConfig{Limiter: fakeLimiter{allow: true}}))
	if code != http.StatusOK {
		t.Errorf("code = %d, want 200", code)
	}
}

func TestRateLimitWithExternalLimiterDeny(t *testing.T) {
	code := runReq(RateLimit(RateLimitConfig{Limiter: fakeLimiter{allow: false}}))
	if code != http.StatusTooManyRequests {
		t.Errorf("code = %d, want 429", code)
	}
}

func TestRateLimitFailOpen(t *testing.T) {
	// Limiter lỗi + FailClosed=false (mặc định) → cho qua.
	code := runReq(RateLimit(RateLimitConfig{Limiter: fakeLimiter{err: errors.New("redis down")}}))
	if code != http.StatusOK {
		t.Errorf("fail-open code = %d, want 200", code)
	}
}

func TestRateLimitFailClosed(t *testing.T) {
	code := runReq(RateLimit(RateLimitConfig{Limiter: fakeLimiter{err: errors.New("redis down")}, FailClosed: true}))
	if code != http.StatusTooManyRequests {
		t.Errorf("fail-closed code = %d, want 429", code)
	}
}
