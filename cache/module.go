package cache

import (
	"context"
	"log/slog"
	"time"

	"github.com/aupv9/common-go/config"
	"github.com/aupv9/common-go/health"
	"github.com/redis/go-redis/v9"
	"go.uber.org/fx"
)

// Module cung cấp redis.UniversalClient dựng từ *config.AppConfig: tạo + ping
// lúc khởi động (fail fast), đăng ký health checker, đóng client khi shutdown.
//
// opts là option bổ sung áp lên Config sau khi map từ AppConfig.
func Module(opts ...func(*Config)) fx.Option {
	return fx.Module("cache",
		fx.Provide(func(lc fx.Lifecycle, cfg *config.AppConfig, log *slog.Logger) (redis.UniversalClient, error) {
			c := configFromApp(cfg)
			for _, o := range opts {
				o(&c)
			}

			startCtx, cancel := context.WithTimeout(context.Background(), startupTimeout(c.DialTimeout))
			defer cancel()
			rdb, err := New(startCtx, c)
			if err != nil {
				return nil, err
			}

			lc.Append(fx.Hook{
				OnStop: func(_ context.Context) error {
					log.Info("closing cache client")
					return rdb.Close()
				},
			})
			return rdb, nil
		}),
		fx.Invoke(func(reg *health.Registry, rdb redis.UniversalClient) {
			reg.Register(NewChecker(rdb, "cache"))
		}),
	)
}

// startupTimeout trả timeout khởi tạo lúc start (gấp đôi dialTimeout, tối thiểu 10s).
func startupTimeout(dialTimeout time.Duration) time.Duration {
	d := dialTimeout * 2
	if d < 10*time.Second {
		return 10 * time.Second
	}
	return d
}

// configFromApp map cấu hình cache từ AppConfig sang Config của package.
func configFromApp(cfg *config.AppConfig) Config {
	c := cfg.Cache
	return Config{
		Addrs:        c.Addrs,
		Password:     config.ResolveSecretOr(c.Password),
		DB:           c.DB,
		PoolSize:     c.PoolSize,
		MinIdleConns: c.MinIdleConns,
		DialTimeout:  c.DialTimeout,
		ReadTimeout:  c.ReadTimeout,
		WriteTimeout: c.WriteTimeout,
		Tracing:      cfg.Otel.Enabled,
	}
}
