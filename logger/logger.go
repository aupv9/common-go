// Package logger cấu hình log có cấu trúc dựa trên log/slog (stdlib).
//
// Logger phát ra JSON (production) hoặc text (local) và tự đính kèm các field
// lấy từ context: service, request_id, trace_id. Trace_id để trống ở M1 và
// được nối với OpenTelemetry ở M3.
package logger

import (
	"io"
	"log/slog"
	"os"
	"strings"
)

// Format định dạng output log.
type Format string

const (
	FormatJSON Format = "json"
	FormatText Format = "text"
)

// Config cấu hình logger.
type Config struct {
	Level   string // debug | info | warn | error
	Format  Format // json | text
	Service string // tên service, đính vào mọi record
	Writer  io.Writer
}

// New tạo *slog.Logger theo Config. Mặc định: level info, format json,
// writer os.Stdout. Logger trả về đã bọc contextHandler nên FromContext sẽ
// tự thêm field từ context.
func New(cfg Config) *slog.Logger {
	l, _ := NewWithControl(cfg)
	return l
}

// NewWithControl như New nhưng trả thêm *LevelControl để chỉnh log level lúc
// runtime (qua endpoint admin hoặc code). Cùng một LevelControl điều khiển level
// của logger trả về.
func NewWithControl(cfg Config) (*slog.Logger, *LevelControl) {
	w := cfg.Writer
	if w == nil {
		w = os.Stdout
	}

	lvar := new(slog.LevelVar)
	lvar.Set(parseLevel(cfg.Level))
	opts := &slog.HandlerOptions{Level: lvar}

	var base slog.Handler
	switch Format(strings.ToLower(string(cfg.Format))) {
	case FormatText:
		base = slog.NewTextHandler(w, opts)
	default:
		base = slog.NewJSONHandler(w, opts)
	}

	h := &contextHandler{Handler: base}
	l := slog.New(h)
	if cfg.Service != "" {
		l = l.With(slog.String("service", cfg.Service))
	}
	return l, &LevelControl{v: lvar}
}

func parseLevel(s string) slog.Level {
	switch strings.ToLower(s) {
	case "debug":
		return slog.LevelDebug
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
