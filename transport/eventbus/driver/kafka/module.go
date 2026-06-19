package kafka

import (
	"context"
	"log/slog"

	"github.com/aupv9/common-go/config"
	"github.com/aupv9/common-go/transport/eventbus"
	"go.uber.org/fx"
)

// Module cung cấp eventbus.Publisher + eventbus.Subscriber (driver Kafka) dựng
// từ *config.AppConfig, đóng cả hai khi shutdown qua lifecycle.
//
// Đặt fx wiring ở đây (driver) thay vì ở package eventbus để eventbus core giữ
// nguyên không phụ thuộc fx/kafka — tránh import cycle.
//
// opts cho phép tinh chỉnh Config sau khi map từ AppConfig.
func Module(opts ...func(*Config)) fx.Option {
	return fx.Module("eventbus",
		fx.Provide(func(lc fx.Lifecycle, cfg *config.AppConfig, log *slog.Logger) (eventbus.Publisher, eventbus.Subscriber) {
			kcfg := configFromApp(cfg)
			for _, o := range opts {
				o(&kcfg)
			}

			pub := NewPublisher(kcfg)
			sub := NewSubscriber(kcfg, log)

			lc.Append(fx.Hook{
				OnStop: func(_ context.Context) error {
					log.Info("closing eventbus")
					_ = sub.Close()
					return pub.Close()
				},
			})
			return pub, sub
		}),
	)
}

// configFromApp map cấu hình eventbus từ AppConfig sang Config của driver.
func configFromApp(cfg *config.AppConfig) Config {
	e := cfg.EventBus
	kc := DefaultConfig()
	if len(e.Brokers) > 0 {
		kc.Brokers = e.Brokers
	}
	if e.RetryMax > 0 {
		kc.RetryMax = e.RetryMax
	}
	if e.RetryBackoff > 0 {
		kc.RetryBackoff = e.RetryBackoff
	}
	if e.DLQSuffix != "" {
		kc.DLQSuffix = e.DLQSuffix
	}
	if e.Concurrency > 0 {
		kc.Concurrency = e.Concurrency
	}
	if e.RetrySuffix != "" {
		kc.RetrySuffix = e.RetrySuffix
	}
	kc.DLQEnabled = e.DLQEnabled
	kc.AsyncRetry = e.AsyncRetry
	kc.Tracing = cfg.Otel.Enabled
	return kc
}
