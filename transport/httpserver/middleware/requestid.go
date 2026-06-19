// Package middleware chứa các gin middleware chuẩn của common-go: request id,
// context logger, recovery, access log, timeout, CORS.
//
// Thứ tự khuyến nghị khi gắn vào engine (xem httpserver.New):
//
//	ContextLogger → RequestID → Recovery → AccessLog → Timeout → CORS → (OTel ở M3)
package middleware

import (
	"crypto/rand"
	"encoding/hex"
	"log/slog"

	"github.com/aupv9/common-go/logger"
	"github.com/gin-gonic/gin"
)

// HeaderRequestID là tên header mang request id (vào và ra).
const HeaderRequestID = "X-Request-ID"

// ginRequestIDKey là khoá lưu request id trong gin.Context (tiện cho handler).
const ginRequestIDKey = "request_id"

// ContextLogger gắn logger nền vào request context để FromContext lấy ra dùng.
// Phải đứng trước các middleware có log để chúng dùng đúng logger (và nhờ đó
// nhận luôn các field context như request_id/trace_id).
func ContextLogger(base *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := logger.IntoContext(c.Request.Context(), base)
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	}
}

// RequestID lấy request id từ header X-Request-ID nếu có, ngược lại sinh mới.
// Id được: (1) gắn vào request context (mọi log record sau đó có request_id),
// (2) set vào gin.Context cho handler, (3) phản hồi qua header response.
func RequestID() gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.GetHeader(HeaderRequestID)
		if id == "" {
			id = newID()
		}

		ctx := logger.WithRequestID(c.Request.Context(), id)
		c.Request = c.Request.WithContext(ctx)
		c.Set(ginRequestIDKey, id)
		c.Header(HeaderRequestID, id)

		c.Next()
	}
}

// RequestIDFromGin lấy request id đã gắn từ gin.Context (rỗng nếu chưa có).
func RequestIDFromGin(c *gin.Context) string {
	if v, ok := c.Get(ginRequestIDKey); ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

// newID sinh id ngẫu nhiên 16 byte (hex). Dùng crypto/rand; cực hiếm khi lỗi,
// khi đó fallback chuỗi cố định để không chặn request.
func newID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "req-unknown"
	}
	return hex.EncodeToString(b[:])
}
