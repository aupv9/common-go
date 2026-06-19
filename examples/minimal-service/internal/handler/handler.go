// Package handler chứa HTTP handler mẫu cho minimal-service.
package handler

import (
	"log/slog"

	"github.com/aupv9/common-go/response"
	"github.com/gin-gonic/gin"
)

// Handler nhóm các endpoint nghiệp vụ.
type Handler struct {
	log *slog.Logger
}

// New tạo Handler. Dependency (logger) được fx inject.
func New(log *slog.Logger) *Handler {
	return &Handler{log: log}
}

// Ping trả về một response envelope mẫu.
func (h *Handler) Ping(c *gin.Context) {
	h.log.InfoContext(c.Request.Context(), "ping called")
	response.OK(c.Writer, c.Request, gin.H{"message": "pong"})
}
