package middleware

import (
	"log/slog"
	"net/http"
	"net/http/httputil"
	"runtime/debug"
	"strings"

	apperrors "github.com/aupv9/common-go/errors"
	"github.com/aupv9/common-go/logger"
	"github.com/aupv9/common-go/response"
	"github.com/gin-gonic/gin"
)

// Recovery bắt panic trong chuỗi handler, log đầy đủ stack (server-side) và trả
// về một AppError internal (500) qua response envelope — KHÔNG lộ chi tiết panic
// hay stack ra client.
//
// Trường hợp broken pipe (client đã ngắt kết nối) chỉ log debug và không cố ghi
// response (sẽ vô ích).
func Recovery() gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			rec := recover()
			if rec == nil {
				return
			}

			ctx := c.Request.Context()
			log := logger.FromContext(ctx)

			if isBrokenPipe(rec) {
				log.DebugContext(ctx, "http connection broken", slog.Any("panic", rec))
				c.Abort()
				return
			}

			log.ErrorContext(ctx, "http handler panic",
				slog.Any("panic", rec),
				slog.String("method", c.Request.Method),
				slog.String("path", c.Request.URL.Path),
				slog.String("request", string(safeDumpRequest(c.Request))),
				slog.String("stack", string(debug.Stack())),
			)

			c.Abort()
			err := apperrors.Internal("INTERNAL", "internal server error")
			response.Error(c.Writer, c.Request, err)
		}()

		c.Next()
	}
}

// isBrokenPipe nhận diện lỗi kết nối đứt (client gone) để tránh spam log error.
func isBrokenPipe(rec any) bool {
	err, ok := rec.(error)
	if !ok {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "broken pipe") || strings.Contains(msg, "connection reset by peer")
}

// safeDumpRequest dump request (không body) để log; lỗi dump thì trả rỗng.
func safeDumpRequest(r *http.Request) []byte {
	dump, err := httputil.DumpRequest(r, false)
	if err != nil {
		return nil
	}
	return dump
}
