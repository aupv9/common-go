package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// SecurityHeaders gắn các header bảo mật mặc định hợp lý cho API:
//   - X-Content-Type-Options: nosniff
//   - X-Frame-Options: DENY
//   - Referrer-Policy: no-referrer
//   - X-XSS-Protection: 0 (tắt bộ lọc cũ, dựa CSP)
//
// hsts=true thêm Strict-Transport-Security (chỉ bật khi phục vụ qua HTTPS).
func SecurityHeaders(hsts bool) gin.HandlerFunc {
	return func(c *gin.Context) {
		h := c.Writer.Header()
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("X-Frame-Options", "DENY")
		h.Set("Referrer-Policy", "no-referrer")
		h.Set("X-XSS-Protection", "0")
		if hsts {
			h.Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
		}
		c.Next()
	}
}

// BodyLimit giới hạn kích thước request body (byte). Vượt giới hạn thì việc đọc
// body trong handler trả lỗi (qua http.MaxBytesReader). maxBytes <= 0 = no-op.
func BodyLimit(maxBytes int64) gin.HandlerFunc {
	return func(c *gin.Context) {
		if maxBytes > 0 {
			c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxBytes)
		}
		c.Next()
	}
}
