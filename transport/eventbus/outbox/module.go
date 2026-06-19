package outbox

import (
	"context"
	"log/slog"

	"github.com/aupv9/common-go/transport/eventbus"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/fx"
)

// Module cung cấp *Relay (dựng từ *pgxpool.Pool + eventbus.Publisher có sẵn trong
// graph) và chạy vòng relay nền qua lifecycle. Yêu cầu service đã bật WithPostgres
// và WithEventBus, đồng thời migration đã tạo bảng outbox (xem Schema).
func Module(opts ...Option) fx.Option {
	return fx.Module("outbox",
		fx.Provide(func(pool *pgxpool.Pool, pub eventbus.Publisher, log *slog.Logger) *Relay {
			return NewRelay(pool, pub, log, opts...)
		}),
		fx.Invoke(func(lc fx.Lifecycle, r *Relay, log *slog.Logger) {
			ctx, cancel := context.WithCancel(context.Background())
			lc.Append(fx.Hook{
				OnStart: func(context.Context) error {
					go r.Run(ctx)
					return nil
				},
				OnStop: func(context.Context) error {
					cancel()
					return nil
				},
			})
		}),
	)
}
