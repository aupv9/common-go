// Package internal nối route cho minimal-service.
package internal

import (
	"github.com/aupv9/common-go/examples/minimal-service/internal/handler"
	"github.com/gin-gonic/gin"
)

// Register gắn route nghiệp vụ vào engine. Health endpoint (/healthz, /readyz,
// /info) đã được foundation.WithHTTP() tự gắn nên service không cần lo.
//
// Hàm này được gọi qua fx.Invoke — *gin.Engine và *handler.Handler tự được inject.
func Register(e *gin.Engine, h *handler.Handler) {
	v1 := e.Group("/api/v1")
	v1.GET("/ping", h.Ping)
}
