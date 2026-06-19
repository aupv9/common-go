package health_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/aupv9/common-go/health"
)

func slowChecker(name string, d time.Duration, err error) health.Checker {
	return health.CheckerFunc{
		CheckerName: name,
		CheckFn: func(ctx context.Context) error {
			select {
			case <-time.After(d):
				return err
			case <-ctx.Done():
				return ctx.Err()
			}
		},
	}
}

func TestReadinessParallel(t *testing.T) {
	reg := health.NewRegistry("svc", "test")
	// 3 checker mỗi cái ~50ms; chạy song song nên tổng ~50ms, không phải 150ms.
	reg.Register(
		slowChecker("a", 50*time.Millisecond, nil),
		slowChecker("b", 50*time.Millisecond, nil),
		slowChecker("c", 50*time.Millisecond, nil),
	)

	rec := httptest.NewRecorder()
	start := time.Now()
	reg.ReadinessHandler()(rec, httptest.NewRequest(http.MethodGet, "/readyz", nil))
	elapsed := time.Since(start)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if elapsed > 120*time.Millisecond {
		t.Errorf("elapsed = %v, expected ~50ms (parallel), not sequential", elapsed)
	}
}

func TestReadinessDetectsDown(t *testing.T) {
	reg := health.NewRegistry("svc", "test")
	reg.Register(
		slowChecker("ok", time.Millisecond, nil),
		slowChecker("bad", time.Millisecond, context.DeadlineExceeded),
	)

	rec := httptest.NewRecorder()
	reg.ReadinessHandler()(rec, httptest.NewRequest(http.MethodGet, "/readyz", nil))
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503", rec.Code)
	}

	var body struct {
		Status string `json:"status"`
		Checks map[string]struct {
			Status string `json:"status"`
		} `json:"checks"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &body)
	if body.Status != "down" {
		t.Errorf("status = %q, want down", body.Status)
	}
	if body.Checks["ok"].Status != "up" || body.Checks["bad"].Status != "down" {
		t.Errorf("checks = %+v", body.Checks)
	}
}
