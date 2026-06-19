package logger

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"
)

// LevelControl cho phép đọc/đổi log level lúc runtime mà không cần restart.
// An toàn cho goroutine (slog.LevelVar dùng atomic).
type LevelControl struct {
	v *slog.LevelVar
}

// Level trả level hiện tại.
func (c *LevelControl) Level() slog.Level { return c.v.Level() }

// SetString đặt level theo tên ("debug"|"info"|"warn"|"error").
func (c *LevelControl) SetString(level string) { c.v.Set(parseLevel(level)) }

// Set đặt level trực tiếp.
func (c *LevelControl) Set(level slog.Level) { c.v.Set(level) }

// Handler trả http.Handler quản trị log level:
//   - GET  → {"level":"info"}
//   - PUT/POST với body {"level":"debug"} hoặc query ?level=debug → đổi level
//
// Endpoint này nên đặt sau auth/giới hạn mạng vì là API quản trị.
func (c *LevelControl) Handler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			writeLevel(w, c.Level())
		case http.MethodPut, http.MethodPost:
			lvl := r.URL.Query().Get("level")
			if lvl == "" {
				var body struct {
					Level string `json:"level"`
				}
				_ = json.NewDecoder(r.Body).Decode(&body)
				lvl = body.Level
			}
			if strings.TrimSpace(lvl) == "" {
				http.Error(w, `{"error":"missing level"}`, http.StatusBadRequest)
				return
			}
			c.SetString(lvl)
			writeLevel(w, c.Level())
		default:
			http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
		}
	}
}

func writeLevel(w http.ResponseWriter, lvl slog.Level) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(map[string]string{"level": strings.ToLower(lvl.String())})
}
