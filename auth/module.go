package auth

import (
	"github.com/aupv9/common-go/config"
	"go.uber.org/fx"
)

// Module cung cấp một Verifier (JWT) dựng từ *config.AppConfig. Việc gắn
// Middleware (HTTP) hay Interceptor (gRPC) là của service vì cần chọn path/method
// và scope cụ thể:
//
//	fx.Invoke(func(e *gin.Engine, v auth.Verifier) {
//	    api := e.Group("/api", auth.Middleware(v))
//	    api.POST("/orders", auth.RequireScopes("orders:write"), h.Create)
//	})
var Module = fx.Module("auth",
	fx.Provide(newVerifierFromConfig),
)

func newVerifierFromConfig(cfg *config.AppConfig) (Verifier, error) {
	a := cfg.Auth
	return NewJWTVerifier(JWTConfig{
		Algorithm:    a.Algorithm,
		Secret:       config.ResolveSecretOr(a.Secret),
		PublicKeyPEM: config.ResolveSecretOr(a.PublicKeyPEM),
		Issuer:       a.Issuer,
		Audience:     a.Audience,
		ScopeClaim:   a.ScopeClaim,
	})
}
