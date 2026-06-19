package middleware

import (
	"log/slog"
	"net/url"
	"time"

	"github.com/aupv9/common-go/logger"
	"github.com/gin-gonic/gin"
)

// sensitiveQueryKeys là các query param sẽ bị che giá trị khi log access (tránh
// rò token/secret nằm trong URL). So khớp không phân biệt hoa thường.
var sensitiveQueryKeys = map[string]struct{}{
	"token":         {},
	"access_token":  {},
	"refresh_token": {},
	"password":      {},
	"secret":        {},
	"api_key":       {},
	"apikey":        {},
	"key":           {},
	"authorization": {},
	"code":          {},
}

// AccessLog ghi một dòng log có cấu trúc cho mỗi request sau khi xử lý xong:
// method, path, status, latency, client ip, kích thước response. Level theo
// status: 5xx → error, 4xx → warn, còn lại → info.
//
// skip chứa các path bỏ qua không log (vd health endpoint) để tránh nhiễu.
func AccessLog(skip ...string) gin.HandlerFunc {
	skipSet := make(map[string]struct{}, len(skip))
	for _, p := range skip {
		skipSet[p] = struct{}{}
	}

	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path
		query := c.Request.URL.RawQuery

		c.Next()

		if _, ok := skipSet[path]; ok {
			return
		}

		ctx := c.Request.Context()
		status := c.Writer.Status()
		attrs := []any{
			slog.String("method", c.Request.Method),
			slog.String("path", path),
			slog.Int("status", status),
			slog.Duration("latency", time.Since(start)),
			slog.String("client_ip", c.ClientIP()),
			slog.Int("bytes", c.Writer.Size()),
		}
		if query != "" {
			attrs = append(attrs, slog.String("query", redactQuery(query)))
		}
		if len(c.Errors) > 0 {
			attrs = append(attrs, slog.String("gin_errors", c.Errors.String()))
		}

		log := logger.FromContext(ctx)
		msg := "http request"
		switch {
		case status >= 500:
			log.LogAttrs(ctx, slog.LevelError, msg, toAttrs(attrs)...)
		case status >= 400:
			log.LogAttrs(ctx, slog.LevelWarn, msg, toAttrs(attrs)...)
		default:
			log.LogAttrs(ctx, slog.LevelInfo, msg, toAttrs(attrs)...)
		}
	}
}

// redactQuery che giá trị các param nhạy cảm trong query string. Nếu query
// không parse được thì trả "[redacted]" cho an toàn.
func redactQuery(raw string) string {
	values, err := url.ParseQuery(raw)
	if err != nil {
		return "[redacted]"
	}
	changed := false
	for k, vs := range values {
		if _, ok := sensitiveQueryKeys[lowerASCII(k)]; ok {
			for i := range vs {
				vs[i] = "***"
			}
			changed = true
		}
	}
	if !changed {
		return raw
	}
	return values.Encode()
}

// lowerASCII hạ thường nhanh (tránh import strings cho một chỗ nóng).
func lowerASCII(s string) string {
	b := []byte(s)
	for i, c := range b {
		if c >= 'A' && c <= 'Z' {
			b[i] = c + 32
		}
	}
	return string(b)
}

// toAttrs ép []any (toàn slog.Attr) về []slog.Attr cho LogAttrs.
func toAttrs(in []any) []slog.Attr {
	out := make([]slog.Attr, 0, len(in))
	for _, v := range in {
		if a, ok := v.(slog.Attr); ok {
			out = append(out, a)
		}
	}
	return out
}
