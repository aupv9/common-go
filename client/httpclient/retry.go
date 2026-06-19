package httpclient

import (
	"bytes"
	"context"
	"io"
	"math/rand/v2"
	"net/http"
	"strconv"
	"time"
)

// idempotentMethods là tập method an toàn để retry mặc định (không có side
// effect thay đổi theo số lần gọi).
var idempotentMethods = map[string]struct{}{
	http.MethodGet:     {},
	http.MethodHead:    {},
	http.MethodPut:     {},
	http.MethodDelete:  {},
	http.MethodOptions: {},
	http.MethodTrace:   {},
}

// DefaultRetryPolicy retry khi: lỗi transport (resp nil), hoặc status 429 /
// 502 / 503 / 504, VÀ method là idempotent. POST/PATCH không retry mặc định để
// tránh nhân đôi side effect (service tự dùng WithRetryPolicy nếu cần).
func DefaultRetryPolicy(req *http.Request, resp *http.Response, err error) bool {
	if _, ok := idempotentMethods[req.Method]; !ok {
		return false
	}
	if err != nil {
		return true
	}
	switch resp.StatusCode {
	case http.StatusTooManyRequests, // 429
		http.StatusBadGateway,         // 502
		http.StatusServiceUnavailable, // 503
		http.StatusGatewayTimeout:     // 504
		return true
	}
	return false
}

// retryTransport bọc một RoundTripper và retry theo policy + exponential backoff.
type retryTransport struct {
	base        http.RoundTripper
	maxRetries  int
	baseBackoff time.Duration
	maxBackoff  time.Duration
	policy      RetryPolicy
	perAttempt  time.Duration // 0 = không giới hạn từng lần thử
}

// RoundTrip thực hiện request, retry theo policy. Tôn trọng ctx.Done() và header
// Retry-After (nếu có) khi tính thời gian chờ.
//
// Theo hợp đồng http.RoundTripper, KHÔNG được sửa request của caller: ta clone
// một bản và chỉ thao tác (buffer/replay body) trên bản clone.
func (t *retryTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	ctx := req.Context()
	outReq := req.Clone(ctx)

	// Buffer body để replay được qua các lần thử (nếu có body và GetBody khả dụng).
	if outReq.Body != nil && outReq.Body != http.NoBody && outReq.GetBody == nil {
		body, err := io.ReadAll(outReq.Body)
		_ = outReq.Body.Close()
		if err != nil {
			return nil, err
		}
		outReq.GetBody = func() (io.ReadCloser, error) {
			return io.NopCloser(bytes.NewReader(body)), nil
		}
		outReq.Body, _ = outReq.GetBody()
	}

	var resp *http.Response
	var err error
	for attempt := 0; ; attempt++ {
		// Per-attempt timeout (nếu bật): bao thời gian của riêng lần thử này.
		attemptReq := outReq
		var cancel context.CancelFunc
		if t.perAttempt > 0 {
			var actx context.Context
			actx, cancel = context.WithTimeout(ctx, t.perAttempt)
			attemptReq = outReq.WithContext(actx)
		}

		resp, err = t.base.RoundTrip(attemptReq)

		if attempt >= t.maxRetries || !t.policy(outReq, resp, err) {
			// Lần trả về cuối: chuyển quyền huỷ timeout sang lúc đóng body để
			// deadline vẫn bao việc đọc body nhưng được giải phóng khi xong.
			return finalizeAttempt(resp, err, cancel)
		}

		// Sẽ retry → giải phóng timeout của lần thử này ngay sau khi drain.
		drain(resp)
		if cancel != nil {
			cancel()
		}

		wait := t.backoff(attempt, resp)
		if !sleepCtx(ctx, wait) {
			// context huỷ trong lúc chờ.
			return resp, ctx.Err()
		}

		// Chuẩn bị lại body cho lần thử kế.
		if outReq.GetBody != nil {
			newBody, gErr := outReq.GetBody()
			if gErr != nil {
				return resp, gErr
			}
			outReq.Body = newBody
		}
	}
}

// finalizeAttempt trả response cuối cùng, gắn cancel của per-attempt timeout vào
// việc đóng body (nếu có body) để không huỷ context trước khi caller đọc xong.
func finalizeAttempt(resp *http.Response, err error, cancel context.CancelFunc) (*http.Response, error) {
	if cancel == nil {
		return resp, err
	}
	if resp == nil || resp.Body == nil {
		cancel()
		return resp, err
	}
	resp.Body = &cancelOnClose{ReadCloser: resp.Body, cancel: cancel}
	return resp, err
}

// cancelOnClose gọi cancel khi body được đóng.
type cancelOnClose struct {
	io.ReadCloser
	cancel context.CancelFunc
}

func (c *cancelOnClose) Close() error {
	err := c.ReadCloser.Close()
	c.cancel()
	return err
}

// backoff tính thời gian chờ: ưu tiên Retry-After (server chỉ định, không jitter),
// nếu không dùng exponential (base * 2^attempt) chặn trên bởi maxBackoff, có
// "equal jitter" để tránh thundering herd khi nhiều client cùng retry.
func (t *retryTransport) backoff(attempt int, resp *http.Response) time.Duration {
	if resp != nil {
		if d, ok := parseRetryAfter(resp.Header.Get("Retry-After")); ok {
			return d
		}
	}
	d := t.baseBackoff << attempt
	if d <= 0 || d > t.maxBackoff {
		d = t.maxBackoff
	}
	return withJitter(d)
}

// withJitter trả nửa d cố định + ngẫu nhiên trong [0, d/2] (equal jitter).
func withJitter(d time.Duration) time.Duration {
	if d <= 0 {
		return 0
	}
	half := d / 2
	return half + time.Duration(rand.Int64N(int64(half)+1))
}

// parseRetryAfter đọc header Retry-After dạng số giây (bỏ qua dạng HTTP-date).
func parseRetryAfter(v string) (time.Duration, bool) {
	if v == "" {
		return 0, false
	}
	if secs, err := strconv.Atoi(v); err == nil && secs >= 0 {
		return time.Duration(secs) * time.Second, true
	}
	return 0, false
}

// drain đọc hết và đóng body để connection được tái dùng từ pool.
func drain(resp *http.Response) {
	if resp == nil || resp.Body == nil {
		return
	}
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))
	_ = resp.Body.Close()
}

// sleepCtx chờ d hoặc tới khi ctx huỷ; trả false nếu ctx huỷ trước.
func sleepCtx(ctx context.Context, d time.Duration) bool {
	if d <= 0 {
		return ctx.Err() == nil
	}
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}
