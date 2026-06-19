package database

import (
	"context"
	"log/slog"
	"time"

	"github.com/aupv9/common-go/config"
	"github.com/aupv9/common-go/health"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/fx"
)

// Module cung cấp *pgxpool.Pool dựng từ *config.AppConfig:
//   - tạo pool + ping lúc khởi động (fail fast nếu DB không sẵn sàng);
//   - (tuỳ config) chạy migration "up" trước khi pool phục vụ;
//   - đăng ký health checker vào readiness;
//   - đóng pool khi shutdown.
//
// opts là option bổ sung áp lên Config sau khi map từ AppConfig.
func Module(opts ...func(*Config)) fx.Option {
	return fx.Module("database",
		fx.Provide(func(lc fx.Lifecycle, cfg *config.AppConfig, log *slog.Logger) (*pgxpool.Pool, error) {
			c := configFromApp(cfg)
			for _, o := range opts {
				o(&c)
			}

			// Migration chạy trước khi mở pool phục vụ (dùng DSN trực tiếp).
			if cfg.Database.MigrateOnStart {
				log.Info("running database migrations", slog.String("path", cfg.Database.MigrationsPath))
				if err := Migrate(c.DSNString(), cfg.Database.MigrationsPath); err != nil {
					return nil, err
				}
			}

			// Bound việc khởi tạo + ping pool bằng một timeout startup tường minh.
			startCtx, cancel := context.WithTimeout(context.Background(), startupTimeout(c.ConnectTimeout))
			defer cancel()
			pool, err := New(startCtx, c)
			if err != nil {
				return nil, err
			}

			lc.Append(fx.Hook{
				OnStop: func(_ context.Context) error {
					log.Info("closing database pool")
					pool.Close()
					return nil
				},
			})
			return pool, nil
		}),
		// Đăng ký health checker vào registry (eager qua Invoke).
		fx.Invoke(func(reg *health.Registry, pool *pgxpool.Pool) {
			reg.Register(NewChecker(pool, "database"))
		}),
	)
}

// startupTimeout trả timeout cho việc khởi tạo lúc start: ưu tiên gấp đôi
// connectTimeout (để cả connect + ping kịp), tối thiểu 10s.
func startupTimeout(connectTimeout time.Duration) time.Duration {
	d := connectTimeout * 2
	if d < 10*time.Second {
		return 10 * time.Second
	}
	return d
}

// configFromApp map cấu hình DB từ AppConfig sang Config của package.
func configFromApp(cfg *config.AppConfig) Config {
	d := cfg.Database
	return Config{
		DSN:             config.ResolveSecretOr(d.DSN),
		Host:            d.Host,
		Port:            d.Port,
		User:            d.User,
		Password:        config.ResolveSecretOr(d.Password),
		Name:            d.Name,
		SSLMode:         d.SSLMode,
		MaxConns:        d.MaxConns,
		MinConns:        d.MinConns,
		MaxConnLifetime: d.MaxConnLifetime,
		MaxConnIdleTime: d.MaxConnIdleTime,
		ConnectTimeout:  d.ConnectTimeout,
		AppName:         cfg.App.Name,
		Tracing:         cfg.Otel.Enabled,
	}
}
