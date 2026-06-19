package observability

import (
	"context"
	"fmt"
	"time"

	"go.opentelemetry.io/contrib/instrumentation/runtime"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp"
	"go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
)

// setupMetrics dựng MeterProvider với periodic reader + OTLP exporter, đặt làm
// global và bật runtime metrics (Go runtime: GC, goroutine, memory...).
func (p *Provider) setupMetrics(ctx context.Context, cfg Config, res *resource.Resource) error {
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

	// Runtime metrics (dùng global MeterProvider vừa set).
	if err := runtime.Start(runtime.WithMinimumReadMemStatsInterval(15 * time.Second)); err != nil {
		return fmt.Errorf("observability: start runtime metrics: %w", err)
	}
	return nil
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
