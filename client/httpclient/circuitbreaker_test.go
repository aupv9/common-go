package httpclient_test

import (
	"errors"
	"testing"
	"time"

	"github.com/aupv9/common-go/client/httpclient"
	"github.com/sony/gobreaker/v2"
)

func TestCircuitBreakerOpens(t *testing.T) {
	cli := httpclient.New(
		httpclient.WithNoRetry(),
		httpclient.WithoutOTel(),
		httpclient.WithCircuitBreaker(httpclient.CircuitBreakerConfig{
			Name:                "test",
			MaxRequests:         1,
			Timeout:             time.Minute,
			ConsecutiveFailures: 3,
		}),
	)

	// Địa chỉ không định tuyến → lỗi transport mỗi lần gọi.
	const dead = "http://127.0.0.1:1/x"

	var lastErr error
	for i := 0; i < 5; i++ {
		_, lastErr = cli.Get(dead)
	}
	// Sau >=3 lỗi liên tiếp, mạch mở → lỗi ErrOpenState.
	if !errors.Is(lastErr, gobreaker.ErrOpenState) {
		t.Fatalf("last error = %v, want ErrOpenState (circuit open)", lastErr)
	}
}

func TestDefaultCircuitBreakerConfig(t *testing.T) {
	c := httpclient.DefaultCircuitBreakerConfig()
	if c.ConsecutiveFailures == 0 || c.Timeout == 0 {
		t.Errorf("unexpected defaults: %+v", c)
	}
}
