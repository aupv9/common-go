// Package observability cấu hình OpenTelemetry cho service: TracerProvider,
// MeterProvider, propagator (W3C TraceContext + Baggage) và runtime metrics.
//
// Khi tắt (Otel.Enabled=false) provider chạy ở chế độ no-op: không exporter,
// không kết nối collector — service local chạy được mà không cần hạ tầng OTel.
// Khi bật, export qua OTLP (gRPC hoặc HTTP) tới endpoint trong config.
//
// Trace id/span id được bơm vào log qua logger.RegisterContextExtractor nên
// mọi log record trong một span tự có trace_id/span_id mà không cần code thủ công.
package observability

import (
	"context"
	"fmt"
	"time"

	promclient "github.com/prometheus/client_golang/prometheus"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
)

// Config là cấu hình runtime của observability (map từ config.AppConfig).
type Config struct {
	Enabled        bool
	Endpoint       string  // OTLP endpoint, vd "localhost:4317"
	Protocol       string  // "grpc" | "http"
	Insecure       bool    // true = không TLS
	SampleRatio    float64 // tỉ lệ sample trace 0..1
	ServiceName    string
	ServiceVersion string
	Env            string
	Headers        map[string]string // header gửi kèm OTLP (vd Authorization)
	MetricInterval time.Duration     // chu kỳ export metric (0 = mặc định 15s; chỉ dùng khi exporter otlp)
	// MetricsExporter: "otlp" (mặc định, push) | "prometheus" (expose /metrics
	// để scrape) | "none" (tắt metrics). Rỗng = "otlp". Traces không bị ảnh hưởng.
	MetricsExporter string
}

// Provider gom các thành phần OTel đã khởi tạo và hàm shutdown để flush.
type Provider struct {
	cfg         Config
	shutdownFns []func(context.Context) error
	// promRegistry != nil khi MetricsExporter="prometheus": registry client_golang
	// mà OTel metrics ghi vào, phục vụ qua MetricsHandler và mở cho service đăng ký
	// thêm collector qua PrometheusRegisterer.
	promRegistry *promclient.Registry
}

// New khởi tạo OTel theo cfg. Luôn cài propagator W3C (TraceContext + Baggage)
// làm global. Nếu cfg.Enabled, dựng tracer + meter exporter OTLP và bật runtime
// metrics; ngược lại để global ở chế độ no-op mặc định của OTel.
func New(ctx context.Context, cfg Config) (*Provider, error) {
	// Propagator global — luôn cài để trace context truyền qua HTTP/gRPC headers.
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	p := &Provider{cfg: cfg}
	if !cfg.Enabled {
		return p, nil
	}

	res, err := buildResource(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("observability: build resource: %w", err)
	}

	if err := p.setupTracing(ctx, cfg, res); err != nil {
		return nil, err
	}
	if err := p.setupMetrics(ctx, cfg, res); err != nil {
		// rollback những gì đã mở trước khi báo lỗi.
		_ = p.Shutdown(ctx)
		return nil, err
	}

	return p, nil
}

// Shutdown flush + đóng mọi exporter/provider đã mở, gọi theo thứ tự ngược.
func (p *Provider) Shutdown(ctx context.Context) error {
	var firstErr error
	for i := len(p.shutdownFns) - 1; i >= 0; i-- {
		if err := p.shutdownFns[i](ctx); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	p.shutdownFns = nil
	return firstErr
}

// Enabled cho biết observability có thực sự export hay đang no-op.
func (p *Provider) Enabled() bool { return p.cfg.Enabled }

// buildResource tạo resource OTel mô tả service (name/version/env).
func buildResource(ctx context.Context, cfg Config) (*resource.Resource, error) {
	attrs := []attribute.KeyValue{
		semconv.ServiceName(cfg.ServiceName),
	}
	if cfg.ServiceVersion != "" {
		attrs = append(attrs, semconv.ServiceVersion(cfg.ServiceVersion))
	}
	if cfg.Env != "" {
		attrs = append(attrs, semconv.DeploymentEnvironment(cfg.Env))
	}
	return resource.New(ctx,
		resource.WithFromEnv(),
		resource.WithTelemetrySDK(),
		resource.WithProcess(),
		resource.WithAttributes(attrs...),
	)
}
