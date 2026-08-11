package observability

import (
	"context"
	"log/slog"
	"time"

	"github.com/aupv9/common-go/config"
	"github.com/aupv9/common-go/logger"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/fx"
)

// Module cung cấp *Provider dựng từ *config.AppConfig, đăng ký bơm trace_id/
// span_id vào log và flush provider khi shutdown.
var Module = fx.Module("observability",
	fx.Provide(newFromConfig),
	fx.Invoke(registerLogExtractor),
	fx.Invoke(registerLifecycle),
)

func newFromConfig(cfg *config.AppConfig) (*Provider, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	return New(ctx, Config{
		Enabled:         cfg.Otel.Enabled,
		Endpoint:        cfg.Otel.Endpoint,
		Protocol:        cfg.Otel.Protocol,
		Insecure:        cfg.Otel.Insecure,
		SampleRatio:     cfg.Otel.SampleRatio,
		ServiceName:     cfg.App.Name,
		ServiceVersion:  cfg.Otel.Version,
		Env:             cfg.App.Env,
		Headers:         cfg.Otel.Headers,
		MetricInterval:  cfg.Otel.MetricInterval,
		MetricsExporter: cfg.Otel.MetricsExporter,
	})
}

// registerLogExtractor bơm trace_id/span_id từ span context vào mọi log record.
// Đăng ký một lần lúc khởi động.
func registerLogExtractor(_ *Provider) {
	logger.RegisterContextExtractor(SpanLogAttrs)
}

// SpanLogAttrs trả trace_id/span_id của span đang active trong ctx; nil nếu
// không có span hợp lệ. Dùng làm logger.ContextExtractor.
func SpanLogAttrs(ctx context.Context) []slog.Attr {
	sc := trace.SpanContextFromContext(ctx)
	if !sc.IsValid() {
		return nil
	}
	return []slog.Attr{
		slog.String("trace_id", sc.TraceID().String()),
		slog.String("span_id", sc.SpanID().String()),
	}
}

// registerLifecycle gắn flush provider vào OnStop.
func registerLifecycle(lc fx.Lifecycle, p *Provider, log *slog.Logger) {
	lc.Append(fx.Hook{
		OnStop: func(ctx context.Context) error {
			if err := p.Shutdown(ctx); err != nil {
				log.Error("observability shutdown error", slog.Any("error", err))
				return err
			}
			return nil
		},
	})
}
