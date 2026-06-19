package httpclient

import (
	"net/http"
	"time"
)

// RetryPolicy quyết định có retry một request hay không, dựa trên request,
// response (có thể nil nếu lỗi transport) và lỗi. Trả true để retry.
type RetryPolicy func(req *http.Request, resp *http.Response, err error) bool

// options gom cấu hình factory.
type options struct {
	timeout time.Duration

	// transport tuning
	maxIdleConns        int
	maxIdleConnsPerHost int
	maxConnsPerHost     int
	idleConnTimeout     time.Duration
	dialTimeout         time.Duration
	tlsHandshakeTimeout time.Duration

	// retry
	maxRetries     int
	baseBackoff    time.Duration
	maxBackoff     time.Duration
	retryPolicy    RetryPolicy
	perAttemptTime time.Duration // timeout cho mỗi lần thử (0 = không)

	// otel bật bọc transport bằng otelhttp (propagation + client span).
	otel bool

	// circuit breaker
	cbEnabled bool
	cb        CircuitBreakerConfig

	// base cho phép service tự cấp transport gốc; nếu set thì bỏ qua tuning + otel.
	base http.RoundTripper
}

func defaultOptions() options {
	return options{
		timeout:             30 * time.Second,
		maxIdleConns:        100,
		maxIdleConnsPerHost: 10,
		maxConnsPerHost:     0,
		idleConnTimeout:     90 * time.Second,
		dialTimeout:         5 * time.Second,
		tlsHandshakeTimeout: 5 * time.Second,
		maxRetries:          2,
		baseBackoff:         100 * time.Millisecond,
		maxBackoff:          2 * time.Second,
		retryPolicy:         DefaultRetryPolicy,
		otel:                true,
	}
}

// Option tinh chỉnh client.
type Option func(*options)

// WithTimeout đặt timeout tổng cho mỗi request (gồm cả retry). 0 = không timeout.
func WithTimeout(d time.Duration) Option { return func(o *options) { o.timeout = d } }

// WithMaxIdleConns đặt tổng số idle connection của pool.
func WithMaxIdleConns(n int) Option { return func(o *options) { o.maxIdleConns = n } }

// WithMaxIdleConnsPerHost đặt số idle connection mỗi host.
func WithMaxIdleConnsPerHost(n int) Option { return func(o *options) { o.maxIdleConnsPerHost = n } }

// WithMaxConnsPerHost giới hạn tổng connection mỗi host (0 = không giới hạn).
func WithMaxConnsPerHost(n int) Option { return func(o *options) { o.maxConnsPerHost = n } }

// WithIdleConnTimeout đặt thời gian giữ idle connection.
func WithIdleConnTimeout(d time.Duration) Option { return func(o *options) { o.idleConnTimeout = d } }

// WithDialTimeout đặt timeout cho việc mở kết nối TCP.
func WithDialTimeout(d time.Duration) Option { return func(o *options) { o.dialTimeout = d } }

// WithRetry cấu hình retry: số lần thử lại tối đa và khoảng backoff.
func WithRetry(maxRetries int, baseBackoff, maxBackoff time.Duration) Option {
	return func(o *options) {
		o.maxRetries = maxRetries
		o.baseBackoff = baseBackoff
		o.maxBackoff = maxBackoff
	}
}

// WithRetryPolicy thay policy quyết định retry (mặc định DefaultRetryPolicy).
func WithRetryPolicy(p RetryPolicy) Option { return func(o *options) { o.retryPolicy = p } }

// WithPerAttemptTimeout đặt timeout cho MỖI lần thử (gồm cả đọc body của lần
// trả về cuối). 0 = chỉ dùng timeout tổng của Client. Hữu ích để một attempt
// chậm không nuốt hết ngân sách thời gian của các retry.
func WithPerAttemptTimeout(d time.Duration) Option {
	return func(o *options) { o.perAttemptTime = d }
}

// WithNoRetry tắt retry hoàn toàn.
func WithNoRetry() Option { return func(o *options) { o.maxRetries = 0 } }

// WithoutOTel tắt bọc otelhttp (mặc định bật: tự inject trace context và tạo
// client span).
func WithoutOTel() Option { return func(o *options) { o.otel = false } }

// WithCircuitBreaker bật circuit breaker (mở mạch sau N lỗi transport liên tiếp).
func WithCircuitBreaker(cfg CircuitBreakerConfig) Option {
	return func(o *options) { o.cbEnabled = true; o.cb = cfg }
}

// WithBaseTransport đặt transport gốc (mặc định tạo từ tuning ở trên). Khi set,
// tuning và otelhttp bị bỏ qua — caller tự chịu trách nhiệm instrument.
func WithBaseTransport(rt http.RoundTripper) Option {
	return func(o *options) { o.base = rt }
}
