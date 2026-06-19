package auth

import (
	"context"
	"crypto/rsa"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"sync"
	"time"

	apperrors "github.com/aupv9/common-go/errors"
	"github.com/golang-jwt/jwt/v5"
)

// JWKSConfig cấu hình JWKSVerifier (xác thực JWT bằng key công khai lấy từ JWKS
// endpoint, hỗ trợ xoay key — phù hợp OIDC provider).
type JWKSConfig struct {
	URL             string        // JWKS endpoint (vd https://issuer/.well-known/jwks.json)
	Issuer          string        // validate iss (rỗng = bỏ qua)
	Audience        string        // validate aud (rỗng = bỏ qua)
	ScopeClaim      string        // claim chứa scope (mặc định "scope")
	RefreshInterval time.Duration // hạn tối thiểu giữa 2 lần refresh khi gặp kid lạ (mặc định 5 phút)
	HTTPClient      *http.Client  // client gọi JWKS (mặc định http.DefaultClient)
}

// JWKSVerifier xác minh JWT RSA bằng key lấy từ JWKS, tự refresh khi gặp kid lạ.
type JWKSVerifier struct {
	cfg       JWKSConfig
	client    *http.Client
	parserOpt []jwt.ParserOption

	mu          sync.RWMutex
	keys        map[string]*rsa.PublicKey
	lastRefresh time.Time
}

// NewJWKSVerifier tạo verifier và nạp key lần đầu (fail nếu không lấy được JWKS).
func NewJWKSVerifier(ctx context.Context, cfg JWKSConfig) (*JWKSVerifier, error) {
	if cfg.URL == "" {
		return nil, fmt.Errorf("auth: JWKS URL is required")
	}
	if cfg.ScopeClaim == "" {
		cfg.ScopeClaim = "scope"
	}
	if cfg.RefreshInterval <= 0 {
		cfg.RefreshInterval = 5 * time.Minute
	}
	client := cfg.HTTPClient
	if client == nil {
		client = http.DefaultClient
	}

	opts := []jwt.ParserOption{jwt.WithValidMethods([]string{"RS256", "RS384", "RS512"})}
	if cfg.Issuer != "" {
		opts = append(opts, jwt.WithIssuer(cfg.Issuer))
	}
	if cfg.Audience != "" {
		opts = append(opts, jwt.WithAudience(cfg.Audience))
	}

	v := &JWKSVerifier{cfg: cfg, client: client, parserOpt: opts, keys: map[string]*rsa.PublicKey{}}
	if err := v.refresh(ctx); err != nil {
		return nil, err
	}
	return v, nil
}

// Verify parse + validate token; chọn key theo "kid" trong header, refresh JWKS
// nếu gặp kid lạ (có throttle theo RefreshInterval).
func (v *JWKSVerifier) Verify(ctx context.Context, tokenStr string) (*Principal, error) {
	if tokenStr == "" {
		return nil, apperrors.Unauthorized("UNAUTHENTICATED", "missing access token")
	}

	claims := jwt.MapClaims{}
	_, err := jwt.ParseWithClaims(tokenStr, claims, v.keyFunc(ctx), v.parserOpt...)
	if err != nil {
		return nil, apperrors.Unauthorized("UNAUTHENTICATED", "invalid access token").WithMetadata("reason", err.Error())
	}
	return principalFromClaims(claims, v.cfg.ScopeClaim), nil
}

func (v *JWKSVerifier) keyFunc(ctx context.Context) jwt.Keyfunc {
	return func(token *jwt.Token) (any, error) {
		kid, _ := token.Header["kid"].(string)
		if key := v.lookup(kid); key != nil {
			return key, nil
		}
		// kid lạ → thử refresh (có throttle) rồi tra lại.
		if err := v.maybeRefresh(ctx); err != nil {
			return nil, err
		}
		if key := v.lookup(kid); key != nil {
			return key, nil
		}
		return nil, fmt.Errorf("auth: no JWKS key for kid %q", kid)
	}
}

func (v *JWKSVerifier) lookup(kid string) *rsa.PublicKey {
	v.mu.RLock()
	defer v.mu.RUnlock()
	if kid != "" {
		return v.keys[kid]
	}
	// Không có kid: nếu chỉ có 1 key thì dùng nó.
	if len(v.keys) == 1 {
		for _, k := range v.keys {
			return k
		}
	}
	return nil
}

// maybeRefresh refresh nếu đã qua RefreshInterval kể từ lần cuối (tránh hammer).
func (v *JWKSVerifier) maybeRefresh(ctx context.Context) error {
	v.mu.RLock()
	recent := time.Since(v.lastRefresh) < v.cfg.RefreshInterval
	v.mu.RUnlock()
	if recent {
		return nil
	}
	return v.refresh(ctx)
}

// refresh tải JWKS và thay bộ key.
func (v *JWKSVerifier) refresh(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, v.cfg.URL, nil)
	if err != nil {
		return fmt.Errorf("auth: build JWKS request: %w", err)
	}
	resp, err := v.client.Do(req)
	if err != nil {
		return fmt.Errorf("auth: fetch JWKS: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("auth: JWKS endpoint status %d", resp.StatusCode)
	}

	var jwks struct {
		Keys []struct {
			Kty string `json:"kty"`
			Kid string `json:"kid"`
			N   string `json:"n"`
			E   string `json:"e"`
		} `json:"keys"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&jwks); err != nil {
		return fmt.Errorf("auth: decode JWKS: %w", err)
	}

	keys := make(map[string]*rsa.PublicKey, len(jwks.Keys))
	for _, k := range jwks.Keys {
		if k.Kty != "RSA" {
			continue
		}
		pub, err := rsaPublicKey(k.N, k.E)
		if err != nil {
			return fmt.Errorf("auth: parse JWKS key %q: %w", k.Kid, err)
		}
		keys[k.Kid] = pub
	}
	if len(keys) == 0 {
		return fmt.Errorf("auth: no RSA keys in JWKS")
	}

	v.mu.Lock()
	v.keys = keys
	v.lastRefresh = time.Now()
	v.mu.Unlock()
	return nil
}

// rsaPublicKey dựng *rsa.PublicKey từ modulus (n) và exponent (e) base64url.
func rsaPublicKey(nB64, eB64 string) (*rsa.PublicKey, error) {
	nBytes, err := base64.RawURLEncoding.DecodeString(nB64)
	if err != nil {
		return nil, fmt.Errorf("decode modulus: %w", err)
	}
	eBytes, err := base64.RawURLEncoding.DecodeString(eB64)
	if err != nil {
		return nil, fmt.Errorf("decode exponent: %w", err)
	}
	// E thường 3 byte (AQAB = 65537); pad trái về 8 byte để đọc uint64.
	if len(eBytes) > 8 {
		return nil, fmt.Errorf("exponent too large")
	}
	var eb [8]byte
	copy(eb[8-len(eBytes):], eBytes)
	e := int(binary.BigEndian.Uint64(eb[:]))

	return &rsa.PublicKey{N: new(big.Int).SetBytes(nBytes), E: e}, nil
}
