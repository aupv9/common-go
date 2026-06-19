// Package httpclient là factory tạo *http.Client production-ready: timeout,
// connection pool đã tinh chỉnh, retry (exponential backoff cho method
// idempotent) và sẵn sàng gắn instrumentation OTel (WithBaseTransport ở M3).
//
//	cli := httpclient.New(
//	    httpclient.WithTimeout(10*time.Second),
//	    httpclient.WithRetry(3, 100*time.Millisecond, 2*time.Second),
//	)
//	resp, err := cli.Get("https://api.example.com/health")
package httpclient

import (
	"net"
	"net/http"
	"time"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
)

// New tạo *http.Client theo option. Pipeline transport: retryTransport bọc
// base transport (mặc định *http.Transport đã tune; hoặc transport do
// WithBaseTransport cấp).
func New(opts ...Option) *http.Client {
	o := defaultOptions()
	for _, opt := range opts {
		opt(&o)
	}

	base := o.base
	if base == nil {
		base = &http.Transport{
			Proxy: http.ProxyFromEnvironment,
			DialContext: (&net.Dialer{
				Timeout:   o.dialTimeout,
				KeepAlive: 30 * time.Second,
			}).DialContext,
			ForceAttemptHTTP2:   true,
			MaxIdleConns:        o.maxIdleConns,
			MaxIdleConnsPerHost: o.maxIdleConnsPerHost,
			MaxConnsPerHost:     o.maxConnsPerHost,
			IdleConnTimeout:     o.idleConnTimeout,
			TLSHandshakeTimeout: o.tlsHandshakeTimeout,
		}
		// otelhttp bọc transport thật → mỗi attempt inject trace context + span.
		if o.otel {
			base = otelhttp.NewTransport(base)
		}
	}

	var rt http.RoundTripper = base
	// Bọc retryTransport khi có retry HOẶC per-attempt timeout (timeout vẫn áp
	// dụng cho lần thử duy nhất khi maxRetries=0).
	if (o.maxRetries > 0 && o.retryPolicy != nil) || o.perAttemptTime > 0 {
		rt = &retryTransport{
			base:        base,
			maxRetries:  o.maxRetries,
			baseBackoff: o.baseBackoff,
			maxBackoff:  o.maxBackoff,
			policy:      o.retryPolicy,
			perAttempt:  o.perAttemptTime,
		}
	}
	// Circuit breaker là lớp ngoài cùng: bao cả retry để mở mạch khi backend hỏng.
	if o.cbEnabled {
		rt = newCBTransport(rt, o.cb)
	}

	return &http.Client{
		Transport: rt,
		Timeout:   o.timeout,
	}
}
