package observability_test

import (
	"context"
	"testing"

	"github.com/aupv9/common-go/observability"
)

// Khi observability tắt (no-op global meter) các helper vẫn hoạt động, không panic.
func TestAppMetricsNoopSafe(t *testing.T) {
	m := observability.Meter("test-scope")

	c := observability.MustInt64Counter(m, "test.counter", "desc", "{item}")
	c.Add(context.Background(), 1)

	h := observability.MustFloat64Histogram(m, "test.hist", "desc", "ms")
	h.Record(context.Background(), 12.5)

	ud, err := observability.Int64UpDownCounter(m, "test.gauge", "desc", "{conn}")
	if err != nil {
		t.Fatalf("Int64UpDownCounter: %v", err)
	}
	ud.Add(context.Background(), 1)
	ud.Add(context.Background(), -1)
}
