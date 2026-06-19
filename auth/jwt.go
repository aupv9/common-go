package auth

import (
	"context"
	"crypto/rsa"
	"fmt"
	"strings"

	apperrors "github.com/aupv9/common-go/errors"
	"github.com/golang-jwt/jwt/v5"
)

// JWTConfig cấu hình JWTVerifier.
type JWTConfig struct {
	// Algorithm: "HS256"/"HS384"/"HS512" (HMAC) hoặc "RS256"/"RS384"/"RS512" (RSA).
	Algorithm string
	// Secret cho HMAC.
	Secret string
	// PublicKeyPEM cho RSA (PEM PKIX/PKCS1).
	PublicKeyPEM string
	// Issuer/Audience để validate (rỗng = bỏ qua).
	Issuer   string
	Audience string
	// ScopeClaim là claim chứa scope (mặc định "scope", space-separated; cũng hỗ
	// trợ mảng).
	ScopeClaim string
}

// JWTVerifier xác minh JWT bằng HMAC hoặc RSA.
type JWTVerifier struct {
	cfg       JWTConfig
	hmacKey   []byte
	rsaPubKey *rsa.PublicKey
	parserOpt []jwt.ParserOption
}

// NewJWTVerifier dựng verifier từ cfg, fail nếu thiếu key hoặc PEM sai.
func NewJWTVerifier(cfg JWTConfig) (*JWTVerifier, error) {
	if cfg.ScopeClaim == "" {
		cfg.ScopeClaim = "scope"
	}
	v := &JWTVerifier{cfg: cfg}

	alg := strings.ToUpper(cfg.Algorithm)
	switch {
	case strings.HasPrefix(alg, "HS"):
		if cfg.Secret == "" {
			return nil, fmt.Errorf("auth: HMAC secret is required for %s", alg)
		}
		v.hmacKey = []byte(cfg.Secret)
	case strings.HasPrefix(alg, "RS"):
		if cfg.PublicKeyPEM == "" {
			return nil, fmt.Errorf("auth: RSA public key is required for %s", alg)
		}
		key, err := jwt.ParseRSAPublicKeyFromPEM([]byte(cfg.PublicKeyPEM))
		if err != nil {
			return nil, fmt.Errorf("auth: parse RSA public key: %w", err)
		}
		v.rsaPubKey = key
	default:
		return nil, fmt.Errorf("auth: unsupported algorithm %q", cfg.Algorithm)
	}

	opts := []jwt.ParserOption{jwt.WithValidMethods([]string{alg})}
	if cfg.Issuer != "" {
		opts = append(opts, jwt.WithIssuer(cfg.Issuer))
	}
	if cfg.Audience != "" {
		opts = append(opts, jwt.WithAudience(cfg.Audience))
	}
	v.parserOpt = opts
	return v, nil
}

// Verify parse + validate token, trả Principal. Lỗi → AppError Unauthorized.
func (v *JWTVerifier) Verify(_ context.Context, tokenStr string) (*Principal, error) {
	if tokenStr == "" {
		return nil, apperrors.Unauthorized("UNAUTHENTICATED", "missing access token")
	}

	claims := jwt.MapClaims{}
	_, err := jwt.ParseWithClaims(tokenStr, claims, v.keyFunc, v.parserOpt...)
	if err != nil {
		return nil, apperrors.Unauthorized("UNAUTHENTICATED", "invalid access token").WithMetadata("reason", err.Error())
	}

	return v.principalFromClaims(claims), nil
}

func (v *JWTVerifier) keyFunc(_ *jwt.Token) (any, error) {
	if v.hmacKey != nil {
		return v.hmacKey, nil
	}
	return v.rsaPubKey, nil
}

// principalFromClaims rút Subject/Issuer/Scopes từ claim.
func (v *JWTVerifier) principalFromClaims(claims jwt.MapClaims) *Principal {
	return principalFromClaims(claims, v.cfg.ScopeClaim)
}

// principalFromClaims (free func) dùng chung cho JWTVerifier và JWKSVerifier.
func principalFromClaims(claims jwt.MapClaims, scopeClaim string) *Principal {
	if scopeClaim == "" {
		scopeClaim = "scope"
	}
	p := &Principal{Claims: claims}
	if sub, ok := claims["sub"].(string); ok {
		p.Subject = sub
	}
	if iss, ok := claims["iss"].(string); ok {
		p.Issuer = iss
	}
	p.Scopes = parseScopes(claims[scopeClaim])
	return p
}

// parseScopes hỗ trợ scope dạng chuỗi space-separated hoặc mảng.
func parseScopes(raw any) []string {
	switch v := raw.(type) {
	case string:
		if v == "" {
			return nil
		}
		return strings.Fields(v)
	case []string:
		return v
	case []any:
		out := make([]string, 0, len(v))
		for _, e := range v {
			if s, ok := e.(string); ok {
				out = append(out, s)
			}
		}
		return out
	default:
		return nil
	}
}
