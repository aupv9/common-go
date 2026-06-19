package auth

import (
	apperrors "github.com/aupv9/common-go/errors"
	"github.com/aupv9/common-go/response"
	"github.com/gin-gonic/gin"
)

// Middleware xác thực request HTTP: rút Bearer token, Verify, gắn Principal vào
// context. Path trong skipPaths được bỏ qua (vd health endpoint). Lỗi xác thực
// → 401 qua response envelope.
func Middleware(v Verifier, skipPaths ...string) gin.HandlerFunc {
	skip := make(map[string]struct{}, len(skipPaths))
	for _, p := range skipPaths {
		skip[p] = struct{}{}
	}

	return func(c *gin.Context) {
		if _, ok := skip[c.Request.URL.Path]; ok {
			c.Next()
			return
		}

		token := bearerToken(c.GetHeader("Authorization"))
		p, err := v.Verify(c.Request.Context(), token)
		if err != nil {
			c.Abort()
			response.Error(c.Writer, c.Request, err)
			return
		}

		ctx := IntoContext(c.Request.Context(), p)
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	}
}

// RequireScopes trả middleware đảm bảo principal có ĐỦ tất cả scope cho trước.
// Phải đặt SAU Middleware. Thiếu xác thực → 401; thiếu scope → 403.
func RequireScopes(scopes ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		p, ok := FromContext(c.Request.Context())
		if !ok {
			c.Abort()
			response.Error(c.Writer, c.Request, apperrors.Unauthorized("UNAUTHENTICATED", "authentication required"))
			return
		}
		for _, s := range scopes {
			if !p.HasScope(s) {
				c.Abort()
				response.Error(c.Writer, c.Request,
					apperrors.Forbidden("FORBIDDEN", "insufficient scope").WithMetadata("required_scope", s))
				return
			}
		}
		c.Next()
	}
}
