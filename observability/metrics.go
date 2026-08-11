package observability

import (
	"context"
	"fmt"
	"net/http"
	"time"

	promclient "github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.opentelemetry.io/contrib/instrumentation/runtime"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp"
	otelprom "go.opentelemetry.io/otel/exporters/prometheus"
	"go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
)

// setupMetrics dựng MeterProvider theo exporter đã chọn, đặt làm global và bật
// runtime metrics (Go runtime: GC, goroutine, memory...).
//
//   - "otlp" (mặc định): periodic reader push tới OTLP endpoint (như trước).
//   - "prometheus": expose registry client_golang qua MetricsHandler để scrape,
//     KHÔNG push đi đâu.
//   - "none": không dựng metrics (traces vẫn chạy).
func (p *Provider) setupMetrics(ctx context.Context, cfg Config, res *resource.Resource) error {
	switch normalizeMetricsExporter(cfg.MetricsExporter) {
	case "none":
		return nil
	case "prometheus":
		return p.setupPrometheusMetrics(res)
	default: // "otlp"
		return p.setupOTLPMetrics(ctx, cfg, res)
	}
}

// normalizeMetricsExporter quy giá trị rỗng/không hợp lệ về "otlp".
func normalizeMetricsExporter(v string) string {
	switch v {
	case "prometheus":
		return "prometheus"
	case "none":
		return "none"
	default:
		return "otlp"
	}
}

// setupOTLPMetrics dựng periodic reader + OTLP exporter (push).
func (p *Provider) setupOTLPMetrics(ctx context.Context, cfg Config, res *resource.Resource) error {
	exp, err := newMetricExporter(ctx, cfg)
	if err != nil {
		return fmt.Errorf("observability: metric exporter: %w", err)
	}

	interval := cfg.MetricInterval
	if interval <= 0 {
		interval = 15 * time.Second
	}
	reader := metric.NewPeriodicReader(exp, metric.WithInterval(interval))
	mp := metric.NewMeterProvider(
		metric.WithResource(res),
		metric.WithReader(reader),
	)

	otel.SetMeterProvider(mp)
	p.shutdownFns = append(p.shutdownFns, mp.Shutdown)

	return p.startRuntimeMetrics()
}

// setupPrometheusMetrics dựng MeterProvider với reader kiểu prometheus (pull) ghi
// vào một registry client_golang riêng — phục vụ qua MetricsHandler (không push).
func (p *Provider) setupPrometheusMetrics(res *resource.Resource) error {
	reg := promclient.NewRegistry()
	exp, err := otelprom.New(otelprom.WithRegisterer(reg))
	if err != nil {
		return fmt.Errorf("observability: prometheus exporter: %w", err)
	}
	mp := metric.NewMeterProvider(
		metric.WithResource(res),
		metric.WithReader(exp),
	)

	otel.SetMeterProvider(mp)
	p.shutdownFns = append(p.shutdownFns, mp.Shutdown)
	p.promRegistry = reg

	return p.startRuntimeMetrics()
}

// startRuntimeMetrics bật runtime metrics trên global MeterProvider vừa set.
func (p *Provider) startRuntimeMetrics() error {
	if err := runtime.Start(runtime.WithMinimumReadMemStatsInterval(15 * time.Second)); err != nil {
		return fmt.Errorf("observability: start runtime metrics: %w", err)
	}
	return nil
}

// MetricsHandler trả HTTP handler phục vụ metrics dạng Prometheus text khi
// MetricsExporter="prometheus"; trả nil ở các chế độ khác (otlp/none) để caller
// biết không có gì để mount.
//
//	if h := provider.MetricsHandler(); h != nil {
//	    mux.Handle("/metrics", h)
//	}
func (p *Provider) MetricsHandler() http.Handler {
	if p.promRegistry == nil {
		return nil
	}
	return promhttp.HandlerFor(p.promRegistry, promhttp.HandlerOpts{})
}

// PrometheusRegisterer trả registry để service đăng ký thêm collector (vd metric
// hand-rolled) vào CÙNG endpoint /metrics với OTel metrics; nil nếu không ở chế
// độ prometheus.
func (p *Provider) PrometheusRegisterer() promclient.Registerer {
	if p.promRegistry == nil {
		return nil
	}
	return p.promRegistry
}

// newMetricExporter chọn exporter theo protocol (grpc mặc định).
func newMetricExporter(ctx context.Context, cfg Config) (metric.Exporter, error) {
	switch cfg.Protocol {
	case "http":
		opts := []otlpmetrichttp.Option{otlpmetrichttp.WithEndpoint(cfg.Endpoint)}
		if cfg.Insecure {
			opts = append(opts, otlpmetrichttp.WithInsecure())
		}
		if len(cfg.Headers) > 0 {
			opts = append(opts, otlpmetrichttp.WithHeaders(cfg.Headers))
		}
		return otlpmetrichttp.New(ctx, opts...)
	default: // grpc
		opts := []otlpmetricgrpc.Option{otlpmetricgrpc.WithEndpoint(cfg.Endpoint)}
		if cfg.Insecure {
			opts = append(opts, otlpmetricgrpc.WithInsecure())
		}
		if len(cfg.Headers) > 0 {
			opts = append(opts, otlpmetricgrpc.WithHeaders(cfg.Headers))
		}
		return otlpmetricgrpc.New(ctx, opts...)
	}
}
