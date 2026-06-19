package auth_test

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/aupv9/common-go/auth"
	"github.com/golang-jwt/jwt/v5"
)

// jwksServer phục vụ JWKS từ public key của key, đếm số lần được gọi.
func jwksServer(t *testing.T, key *rsa.PrivateKey, kid string, hits *int32) *httptest.Server {
	t.Helper()
	eBytes := make([]byte, 8)
	binary.BigEndian.PutUint64(eBytes, uint64(key.PublicKey.E))
	// cắt zero đầu
	i := 0
	for i < len(eBytes)-1 && eBytes[i] == 0 {
		i++
	}
	jwks := map[string]any{
		"keys": []map[string]string{{
			"kty": "RSA",
			"kid": kid,
			"n":   base64.RawURLEncoding.EncodeToString(key.PublicKey.N.Bytes()),
			"e":   base64.RawURLEncoding.EncodeToString(eBytes[i:]),
		}},
	}
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		atomic.AddInt32(hits, 1)
		_ = json.NewEncoder(w).Encode(jwks)
	}))
}

func signRS256(t *testing.T, key *rsa.PrivateKey, kid string, claims jwt.MapClaims) string {
	t.Helper()
	tok := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	tok.Header["kid"] = kid
	s, err := tok.SignedString(key)
	if err != nil {
		t.Fatalf("sign: %v", err)
	}
	return s
}

func TestJWKSVerifyValid(t *testing.T) {
	key, _ := rsa.GenerateKey(rand.Reader, 2048)
	var hits int32
	srv := jwksServer(t, key, "kid-1", &hits)
	defer srv.Close()

	v, err := auth.NewJWKSVerifier(context.Background(), auth.JWKSConfig{URL: srv.URL, Issuer: "iss"})
	if err != nil {
		t.Fatalf("NewJWKSVerifier: %v", err)
	}

	tok := signRS256(t, key, "kid-1", jwt.MapClaims{
		"sub": "u1", "iss": "iss", "scope": "a b", "exp": time.Now().Add(time.Hour).Unix(),
	})
	p, err := v.Verify(context.Background(), tok)
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}
	if p.Subject != "u1" || !p.HasScope("a") {
		t.Errorf("principal = %+v", p)
	}
}

func TestJWKSUnknownKidFails(t *testing.T) {
	key, _ := rsa.GenerateKey(rand.Reader, 2048)
	var hits int32
	srv := jwksServer(t, key, "kid-1", &hits)
	defer srv.Close()

	v, err := auth.NewJWKSVerifier(context.Background(), auth.JWKSConfig{URL: srv.URL, RefreshInterval: time.Nanosecond})
	if err != nil {
		t.Fatalf("NewJWKSVerifier: %v", err)
	}

	// Token ký bằng key khác kid → không tìm thấy key, refresh cũng không có.
	tok := signRS256(t, key, "kid-unknown", jwt.MapClaims{"sub": "u", "exp": time.Now().Add(time.Hour).Unix()})
	if _, err := v.Verify(context.Background(), tok); err == nil {
		t.Fatal("expected error for unknown kid")
	}
}

func TestJWKSMissingURL(t *testing.T) {
	if _, err := auth.NewJWKSVerifier(context.Background(), auth.JWKSConfig{}); err == nil {
		t.Error("expected error when URL missing")
	}
}
