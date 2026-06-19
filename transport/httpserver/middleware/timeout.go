package middleware

import (
	"context"
	"time"

	"github.com/gin-gonic/gin"
)

// Timeout gắn deadline vào request context. Handler tôn trọng ctx.Done() sẽ bị
// huỷ khi quá hạn; việc trả lỗi 504 cho client là tuỳ handler/logic downstream
// (middleware này không tự ghi response để tránh ghi đè khi handler đã phản hồi).
//
// d <= 0 thì middleware là no-op.
func Timeout(d time.Duration) gin.HandlerFunc {
	return func(c *gin.Context) {
		if d <= 0 {
			c.Next()
			return
		}
		ctx, cancel := context.WithTimeout(c.Request.Context(), d)
		defer cancel()
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	}
}
