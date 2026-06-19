package httpclient

import (
	"net/http"
	"time"

	"github.com/sony/gobreaker/v2"
)

// CircuitBreakerConfig cấu hình circuit breaker cho client.
type CircuitBreakerConfig struct {
	Name string
	// MaxRequests: số request cho qua khi ở trạng thái half-open.
	MaxRequests uint32
	// Interval: chu kỳ reset bộ đếm khi đang closed (0 = không reset định kỳ).
	Interval time.Duration
	// Timeout: thời gian ở open trước khi chuyển half-open.
	Timeout time.Duration
	// ConsecutiveFailures: số lỗi liên tiếp để trip (mở mạch).
	ConsecutiveFailures uint32
}

// DefaultCircuitBreakerConfig trả cấu hình mặc định hợp lý.
func DefaultCircuitBreakerConfig() CircuitBreakerConfig {
	return CircuitBreakerConfig{
		Name:                "httpclient",
		MaxRequests:         1,
		Interval:            0,
		Timeout:             30 * time.Second,
		ConsecutiveFailures: 5,
	}
}

// cbTransport bọc một RoundTripper bằng circuit breaker. Chỉ lỗi transport (kết
// nối hỏng/timeout) tính là thất bại; response 5xx KHÔNG tự tính là lỗi (dùng
// retryPolicy/handler để xử lý) — tránh mở mạch vì lỗi nghiệp vụ phía server.
type cbTransport struct {
	base http.RoundTripper
	cb   *gobreaker.CircuitBreaker[*http.Response]
}

func newCBTransport(base http.RoundTripper, cfg CircuitBreakerConfig) *cbTransport {
	settings := gobreaker.Settings{
		Name:        cfg.Name,
		MaxRequests: cfg.MaxRequests,
		Interval:    cfg.Interval,
		Timeout:     cfg.Timeout,
		ReadyToTrip: func(counts gobreaker.Counts) bool {
			return counts.ConsecutiveFailures >= cfg.ConsecutiveFailures
		},
	}
	return &cbTransport{
		base: base,
		cb:   gobreaker.NewCircuitBreaker[*http.Response](settings),
	}
}

func (t *cbTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	return t.cb.Execute(func() (*http.Response, error) {
		return t.base.RoundTrip(req)
	})
}
