package observability_test

import (
	"context"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/aupv9/common-go/observability"
	promclient "github.com/prometheus/client_golang/prometheus"
	"go.opentelemetry.io/otel"
)

// TestPrometheusExporterServesMetrics kiểm tra chế độ MetricsExporter="prometheus":
// OTel metric + collector đăng ký thêm đều xuất hiện trên MetricsHandler.
func TestPrometheusExporterServesMetrics(t *testing.T) {
	ctx := context.Background()
	p, err := observability.New(ctx, observability.Config{
		Enabled:         true,
		Endpoint:        "localhost:4317", // trace exporter dial lazy, không cần collector thật
		Protocol:        "grpc",
		Insecure:        true,
		ServiceName:     "test-svc",
		MetricsExporter: "prometheus",
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer func() { _ = p.Shutdown(ctx) }()

	// (1) OTel metric qua global MeterProvider.
	counter, err := otel.Meter("test").Int64Counter("test.calls")
	if err != nil {
		t.Fatalf("Int64Counter: %v", err)
	}
	counter.Add(ctx, 5)

	// (2) Collector client_golang đăng ký thêm qua PrometheusRegisterer.
	reg := p.PrometheusRegisterer()
	if reg == nil {
		t.Fatal("PrometheusRegisterer() = nil, want non-nil in prometheus mode")
	}
	g := promclient.NewGauge(promclient.GaugeOpts{Name: "handrolled_gauge", Help: "test"})
	g.Set(42)
	if err := reg.Register(g); err != nil {
		t.Fatalf("Register: %v", err)
	}

	h := p.MetricsHandler()
	if h == nil {
		t.Fatal("MetricsHandler() = nil, want non-nil in prometheus mode")
	}
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/metrics", nil)
	h.ServeHTTP(rec, req)

	if rec.Code != 200 {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	body := rec.Body.String()
	for _, want := range []string{"test_calls_total", "handrolled_gauge 42"} {
		if !strings.Contains(body, want) {
			t.Errorf("metrics body missing %q\n---\n%s", want, body)
		}
	}
}

// TestMetricsHandlerNilWhenNotPrometheus xác nhận otlp/none không có handler.
func TestMetricsHandlerNilWhenNotPrometheus(t *testing.T) {
	for _, mode := range []string{"", "otlp", "none"} {
		p, err := observability.New(context.Background(), observability.Config{Enabled: false, MetricsExporter: mode})
		if err != nil {
			t.Fatalf("New(%q): %v", mode, err)
		}
		if h := p.MetricsHandler(); h != nil {
			t.Errorf("MetricsHandler() for mode %q = non-nil, want nil", mode)
		}
		if reg := p.PrometheusRegisterer(); reg != nil {
			t.Errorf("PrometheusRegisterer() for mode %q = non-nil, want nil", mode)
		}
		_ = p.Shutdown(context.Background())
	}
}
