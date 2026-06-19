package httpserver

import (
	"time"

	"github.com/aupv9/common-go/tlsconfig"
	"github.com/aupv9/common-go/transport/httpserver/middleware"
	"github.com/gin-gonic/gin"
)

// options gom cấu hình runtime của Server.
type options struct {
	host            string
	port            int
	readTimeout     time.Duration
	writeTimeout    time.Duration
	idleTimeout     time.Duration
	shutdownTimeout time.Duration
	requestTimeout  time.Duration

	mode string // gin mode: debug | release | test

	tracingEnabled bool
	serviceName    string

	corsEnabled  bool
	cors         middleware.CORSConfig
	logSkipPaths []string

	securityHeaders bool
	hsts            bool
	bodyLimit       int64
	gzipEnabled     bool
	gzipLevel       int
	rateLimitOn     bool
	rateLimit       middleware.RateLimitConfig

	tls tlsconfig.Config

	// extra là middleware do service thêm, gắn SAU bộ middleware chuẩn.
	extra []gin.HandlerFunc

	// adminGuards là middleware bảo vệ nhóm /admin (vd auth). Rỗng = không bảo vệ.
	adminGuards []gin.HandlerFunc
}

func defaultOptions() options {
	return options{
		host:            "0.0.0.0",
		port:            8080,
		readTimeout:     15 * time.Second,
		writeTimeout:    15 * time.Second,
		idleTimeout:     60 * time.Second,
		shutdownTimeout: 10 * time.Second,
		requestTimeout:  0, // 0 = tắt timeout middleware
		mode:            gin.ReleaseMode,
		corsEnabled:     false,
		cors:            middleware.DefaultCORSConfig(),
		logSkipPaths:    []string{"/healthz", "/readyz"},
	}
}

// Option tinh chỉnh Server.
type Option func(*options)

// WithAddr đặt host và port.
func WithAddr(host string, port int) Option {
	return func(o *options) { o.host = host; o.port = port }
}

// WithPort đặt port (giữ host hiện tại).
func WithPort(port int) Option { return func(o *options) { o.port = port } }

// WithReadTimeout đặt read timeout của http.Server.
func WithReadTimeout(d time.Duration) Option { return func(o *options) { o.readTimeout = d } }

// WithWriteTimeout đặt write timeout của http.Server.
func WithWriteTimeout(d time.Duration) Option { return func(o *options) { o.writeTimeout = d } }

// WithIdleTimeout đặt idle timeout của http.Server.
func WithIdleTimeout(d time.Duration) Option { return func(o *options) { o.idleTimeout = d } }

// WithShutdownTimeout đặt hạn graceful shutdown.
func WithShutdownTimeout(d time.Duration) Option { return func(o *options) { o.shutdownTimeout = d } }

// WithRequestTimeout bật middleware timeout cho mỗi request (0 = tắt).
func WithRequestTimeout(d time.Duration) Option { return func(o *options) { o.requestTimeout = d } }

// WithMode đặt gin mode ("debug" | "release" | "test").
func WithMode(mode string) Option { return func(o *options) { o.mode = mode } }

// WithTracing bật OTel tracing (otelgin) cho server. serviceName dùng đặt tên
// span. Module tự bật khi Otel.Enabled trong config.
func WithTracing(serviceName string) Option {
	return func(o *options) { o.tracingEnabled = true; o.serviceName = serviceName }
}

// WithCORS bật CORS với cấu hình cho trước.
func WithCORS(cfg middleware.CORSConfig) Option {
	return func(o *options) { o.corsEnabled = true; o.cors = cfg }
}

// WithLogSkipPaths đặt danh sách path bỏ qua access log (mặc định health endpoints).
func WithLogSkipPaths(paths ...string) Option {
	return func(o *options) { o.logSkipPaths = paths }
}

// WithMiddleware thêm middleware của service, gắn sau bộ chuẩn.
func WithMiddleware(mw ...gin.HandlerFunc) Option {
	return func(o *options) { o.extra = append(o.extra, mw...) }
}

// WithSecurityHeaders bật header bảo mật mặc định. hsts chỉ bật khi phục vụ HTTPS.
func WithSecurityHeaders(hsts bool) Option {
	return func(o *options) { o.securityHeaders = true; o.hsts = hsts }
}

// WithBodyLimit giới hạn kích thước request body (byte). 0 = không giới hạn.
func WithBodyLimit(maxBytes int64) Option {
	return func(o *options) { o.bodyLimit = maxBytes }
}

// WithGzip bật nén gzip cho response. level theo compress/gzip (-1 = mặc định).
func WithGzip(level int) Option {
	return func(o *options) { o.gzipEnabled = true; o.gzipLevel = level }
}

// WithRateLimit bật giới hạn tần suất theo client (token bucket).
func WithRateLimit(cfg middleware.RateLimitConfig) Option {
	return func(o *options) { o.rateLimitOn = true; o.rateLimit = cfg }
}

// WithTLS bật TLS/mTLS cho server (xem tlsconfig.Config).
func WithTLS(cfg tlsconfig.Config) Option {
	return func(o *options) { o.tls = cfg }
}

// WithAdminGuard gắn middleware bảo vệ nhóm endpoint /admin (vd /admin/loglevel)
// như auth/giới hạn IP. Không set → /admin công khai (chỉ nên dùng ở mạng nội bộ).
func WithAdminGuard(guards ...gin.HandlerFunc) Option {
	return func(o *options) { o.adminGuards = append(o.adminGuards, guards...) }
}
