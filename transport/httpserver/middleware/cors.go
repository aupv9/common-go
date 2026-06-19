package middleware

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

// CORSConfig cấu hình CORS. Giá trị rỗng → dùng mặc định an toàn (chỉ same-origin
// thực tế là không set header gì khi AllowedOrigins rỗng).
type CORSConfig struct {
	AllowedOrigins   []string      // danh sách origin cho phép; ["*"] = mọi origin
	AllowedMethods   []string      // mặc định GET,POST,PUT,PATCH,DELETE,OPTIONS
	AllowedHeaders   []string      // mặc định Origin,Content-Type,Authorization,X-Request-ID
	ExposedHeaders   []string      // header lộ cho client đọc
	AllowCredentials bool          // cho phép gửi cookie/credential
	MaxAge           time.Duration // thời gian cache preflight
}

// DefaultCORSConfig trả cấu hình CORS mặc định (không origin nào — phải khai báo).
func DefaultCORSConfig() CORSConfig {
	return CORSConfig{
		AllowedMethods: []string{http.MethodGet, http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete, http.MethodOptions},
		AllowedHeaders: []string{"Origin", "Content-Type", "Authorization", HeaderRequestID},
		MaxAge:         12 * time.Hour,
	}
}

// CORS trả middleware áp dụng CORSConfig. Tự xử lý preflight OPTIONS (204).
func CORS(cfg CORSConfig) gin.HandlerFunc {
	if len(cfg.AllowedMethods) == 0 {
		cfg.AllowedMethods = DefaultCORSConfig().AllowedMethods
	}
	if len(cfg.AllowedHeaders) == 0 {
		cfg.AllowedHeaders = DefaultCORSConfig().AllowedHeaders
	}

	allowAll := false
	originSet := make(map[string]struct{}, len(cfg.AllowedOrigins))
	for _, o := range cfg.AllowedOrigins {
		if o == "*" {
			allowAll = true
		}
		originSet[o] = struct{}{}
	}

	methods := strings.Join(cfg.AllowedMethods, ", ")
	headers := strings.Join(cfg.AllowedHeaders, ", ")
	exposed := strings.Join(cfg.ExposedHeaders, ", ")
	maxAge := strconv.Itoa(int(cfg.MaxAge.Seconds()))

	return func(c *gin.Context) {
		origin := c.GetHeader("Origin")
		if origin != "" && (allowAll || isAllowedOrigin(origin, originSet)) {
			h := c.Writer.Header()
			if allowAll && !cfg.AllowCredentials {
				h.Set("Access-Control-Allow-Origin", "*")
			} else {
				h.Set("Access-Control-Allow-Origin", origin)
				h.Add("Vary", "Origin")
			}
			if cfg.AllowCredentials {
				h.Set("Access-Control-Allow-Credentials", "true")
			}
			if exposed != "" {
				h.Set("Access-Control-Expose-Headers", exposed)
			}

			if c.Request.Method == http.MethodOptions {
				h.Set("Access-Control-Allow-Methods", methods)
				h.Set("Access-Control-Allow-Headers", headers)
				h.Set("Access-Control-Max-Age", maxAge)
				c.AbortWithStatus(http.StatusNoContent)
				return
			}
		}
		c.Next()
	}
}

func isAllowedOrigin(origin string, set map[string]struct{}) bool {
	_, ok := set[origin]
	return ok
}
