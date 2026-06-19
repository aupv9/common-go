package auth_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/aupv9/common-go/auth"
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

const testSecret = "test-secret-please-change"

func makeToken(t *testing.T, claims jwt.MapClaims) string {
	t.Helper()
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	s, err := tok.SignedString([]byte(testSecret))
	if err != nil {
		t.Fatalf("sign: %v", err)
	}
	return s
}

func newVerifier(t *testing.T) auth.Verifier {
	t.Helper()
	v, err := auth.NewJWTVerifier(auth.JWTConfig{Algorithm: "HS256", Secret: testSecret, Issuer: "iss"})
	if err != nil {
		t.Fatalf("NewJWTVerifier: %v", err)
	}
	return v
}

func TestVerifyValidToken(t *testing.T) {
	v := newVerifier(t)
	tok := makeToken(t, jwt.MapClaims{
		"sub":   "user-1",
		"iss":   "iss",
		"scope": "orders:read orders:write",
		"exp":   time.Now().Add(time.Hour).Unix(),
	})

	p, err := v.Verify(context.Background(), tok)
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}
	if p.Subject != "user-1" {
		t.Errorf("subject = %q", p.Subject)
	}
	if !p.HasScope("orders:write") || !p.HasScope("orders:read") {
		t.Errorf("scopes = %v", p.Scopes)
	}
	if p.HasScope("admin") {
		t.Error("should not have admin scope")
	}
}

func TestVerifyExpiredToken(t *testing.T) {
	v := newVerifier(t)
	tok := makeToken(t, jwt.MapClaims{"sub": "u", "iss": "iss", "exp": time.Now().Add(-time.Hour).Unix()})
	if _, err := v.Verify(context.Background(), tok); err == nil {
		t.Fatal("expected error for expired token")
	}
}

func TestVerifyWrongIssuer(t *testing.T) {
	v := newVerifier(t)
	tok := makeToken(t, jwt.MapClaims{"sub": "u", "iss": "other", "exp": time.Now().Add(time.Hour).Unix()})
	if _, err := v.Verify(context.Background(), tok); err == nil {
		t.Fatal("expected error for wrong issuer")
	}
}

func TestVerifyEmptyToken(t *testing.T) {
	v := newVerifier(t)
	if _, err := v.Verify(context.Background(), ""); err == nil {
		t.Fatal("expected error for empty token")
	}
}

func TestNewJWTVerifierRequiresKey(t *testing.T) {
	if _, err := auth.NewJWTVerifier(auth.JWTConfig{Algorithm: "HS256"}); err == nil {
		t.Error("expected error when HMAC secret missing")
	}
	if _, err := auth.NewJWTVerifier(auth.JWTConfig{Algorithm: "ZZ999"}); err == nil {
		t.Error("expected error for unsupported algorithm")
	}
}

func TestHTTPMiddleware(t *testing.T) {
	gin.SetMode(gin.TestMode)
	v := newVerifier(t)
	e := gin.New()
	e.Use(auth.Middleware(v, "/public"))
	e.GET("/private", func(c *gin.Context) {
		p, ok := auth.FromContext(c.Request.Context())
		if !ok {
			t.Error("expected principal in context")
		}
		c.String(http.StatusOK, p.Subject)
	})
	e.GET("/public", func(c *gin.Context) { c.String(http.StatusOK, "ok") })

	// Thiếu token → 401.
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/private", nil))
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("no-token status = %d, want 401", rec.Code)
	}

	// Path bỏ qua → 200 không cần token.
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/public", nil))
	if rec.Code != http.StatusOK {
		t.Errorf("public status = %d, want 200", rec.Code)
	}

	// Token hợp lệ → 200 + subject.
	tok := makeToken(t, jwt.MapClaims{"sub": "u9", "iss": "iss", "exp": time.Now().Add(time.Hour).Unix()})
	req := httptest.NewRequest(http.MethodGet, "/private", nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || rec.Body.String() != "u9" {
		t.Errorf("valid status = %d body = %q", rec.Code, rec.Body.String())
	}
}

func TestRequireScopesMiddleware(t *testing.T) {
	gin.SetMode(gin.TestMode)
	v := newVerifier(t)
	e := gin.New()
	api := e.Group("/", auth.Middleware(v))
	api.GET("/write", auth.RequireScopes("orders:write"), func(c *gin.Context) { c.Status(http.StatusOK) })

	// Token chỉ có read → 403.
	tok := makeToken(t, jwt.MapClaims{"sub": "u", "iss": "iss", "scope": "orders:read", "exp": time.Now().Add(time.Hour).Unix()})
	req := httptest.NewRequest(http.MethodGet, "/write", nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Errorf("status = %d, want 403", rec.Code)
	}
}

func TestCheckScopes(t *testing.T) {
	ctx := auth.IntoContext(context.Background(), &auth.Principal{Scopes: []string{"a"}})
	if err := auth.CheckScopes(ctx, "a"); err != nil {
		t.Errorf("CheckScopes(a) = %v, want nil", err)
	}
	if err := auth.CheckScopes(ctx, "b"); err == nil {
		t.Error("CheckScopes(b) should fail")
	}
	if err := auth.CheckScopes(context.Background(), "a"); err == nil {
		t.Error("CheckScopes without principal should fail")
	}
}
