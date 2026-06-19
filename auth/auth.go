// Package auth cung cấp xác thực (authentication) chuẩn cho service: xác minh
// JWT và gắn Principal vào context, dùng được cho cả HTTP (gin middleware) lẫn
// gRPC (interceptor).
//
// Verifier tách rời transport: HTTP/gRPC chỉ rút token rồi gọi Verify. Mặc định
// dùng JWTVerifier (HMAC hoặc RSA), nhưng service có thể cấp Verifier riêng
// (vd introspection OAuth2) qua cùng interface.
//
//	p, _ := auth.FromContext(ctx)   // trong handler đã qua middleware
//	if !p.HasScope("orders:write") { return errors.Forbidden(...) }
package auth

import (
	"context"
	"strings"
)

// Principal là chủ thể đã xác thực (người dùng/service).
type Principal struct {
	Subject string         // "sub" — định danh chủ thể
	Issuer  string         // "iss"
	Scopes  []string       // quyền/scope đã cấp
	Claims  map[string]any // toàn bộ claim gốc (đọc thêm nếu cần)
}

// HasScope trả true nếu principal có scope cho trước.
func (p *Principal) HasScope(scope string) bool {
	if p == nil {
		return false
	}
	for _, s := range p.Scopes {
		if s == scope {
			return true
		}
	}
	return false
}

// HasAnyScope trả true nếu có ít nhất một trong các scope.
func (p *Principal) HasAnyScope(scopes ...string) bool {
	for _, s := range scopes {
		if p.HasScope(s) {
			return true
		}
	}
	return false
}

// Verifier xác minh một access token và trả Principal. Trả lỗi nếu token không
// hợp lệ/hết hạn.
type Verifier interface {
	Verify(ctx context.Context, token string) (*Principal, error)
}

type ctxKey int

const principalKey ctxKey = iota

// IntoContext gắn principal vào context.
func IntoContext(ctx context.Context, p *Principal) context.Context {
	return context.WithValue(ctx, principalKey, p)
}

// FromContext lấy principal đã xác thực; (nil, false) nếu chưa xác thực.
func FromContext(ctx context.Context) (*Principal, bool) {
	p, ok := ctx.Value(principalKey).(*Principal)
	return p, ok && p != nil
}

// bearerToken rút token từ giá trị header Authorization ("Bearer <token>").
// Chấp nhận cả không có prefix (trả nguyên chuỗi).
func bearerToken(authHeader string) string {
	const prefix = "Bearer "
	if strings.HasPrefix(authHeader, prefix) {
		return strings.TrimSpace(authHeader[len(prefix):])
	}
	if strings.HasPrefix(authHeader, "bearer ") {
		return strings.TrimSpace(authHeader[len("bearer "):])
	}
	return strings.TrimSpace(authHeader)
}
