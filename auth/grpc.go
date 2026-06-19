package auth

import (
	"context"

	apperrors "github.com/aupv9/common-go/errors"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

// metadataAuthorization là khoá metadata mang token (gRPC lowercase).
const metadataAuthorization = "authorization"

// UnaryInterceptor xác thực mỗi RPC unary, gắn Principal vào context. skipMethods
// là FullMethod ("/pkg.Service/Method") được bỏ qua (vd health/reflection).
func UnaryInterceptor(v Verifier, skipMethods ...string) grpc.UnaryServerInterceptor {
	skip := toSet(skipMethods)
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		if _, ok := skip[info.FullMethod]; ok {
			return handler(ctx, req)
		}
		ctx, err := authenticate(ctx, v)
		if err != nil {
			return nil, err
		}
		return handler(ctx, req)
	}
}

// StreamInterceptor như UnaryInterceptor nhưng cho stream.
func StreamInterceptor(v Verifier, skipMethods ...string) grpc.StreamServerInterceptor {
	skip := toSet(skipMethods)
	return func(srv any, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		if _, ok := skip[info.FullMethod]; ok {
			return handler(srv, ss)
		}
		ctx, err := authenticate(ss.Context(), v)
		if err != nil {
			return err
		}
		return handler(srv, &wrappedStream{ServerStream: ss, ctx: ctx})
	}
}

// authenticate rút token từ metadata, Verify và trả ctx có Principal.
func authenticate(ctx context.Context, v Verifier) (context.Context, error) {
	token := tokenFromMD(ctx)
	p, err := v.Verify(ctx, token)
	if err != nil {
		return ctx, err
	}
	return IntoContext(ctx, p), nil
}

func tokenFromMD(ctx context.Context) string {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return ""
	}
	if vals := md.Get(metadataAuthorization); len(vals) > 0 {
		return bearerToken(vals[0])
	}
	return ""
}

// CheckScopes trả về lỗi Forbidden nếu principal trong ctx thiếu scope. Dùng
// trong gRPC handler hoặc interceptor tuỳ biến (HTTP dùng RequireScopes middleware).
func CheckScopes(ctx context.Context, scopes ...string) error {
	p, ok := FromContext(ctx)
	if !ok {
		return apperrors.Unauthorized("UNAUTHENTICATED", "authentication required")
	}
	for _, s := range scopes {
		if !p.HasScope(s) {
			return apperrors.Forbidden("FORBIDDEN", "insufficient scope").WithMetadata("required_scope", s)
		}
	}
	return nil
}

func toSet(items []string) map[string]struct{} {
	m := make(map[string]struct{}, len(items))
	for _, i := range items {
		m[i] = struct{}{}
	}
	return m
}

// wrappedStream thay context của ServerStream (cho stream interceptor).
type wrappedStream struct {
	grpc.ServerStream
	ctx context.Context
}

func (w *wrappedStream) Context() context.Context { return w.ctx }
