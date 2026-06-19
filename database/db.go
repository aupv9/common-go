// Package database cung cấp pool kết nối Postgres (pgxpool) production-ready:
// pool tuning, tracing query qua OTel (otelpgx), health checker và (tuỳ chọn)
// chạy migration lúc khởi động.
//
//	pool, err := database.New(ctx, database.Config{
//	    Host: "localhost", Port: 5432, User: "app", Password: "secret",
//	    Name: "app", SSLMode: "disable", Tracing: true,
//	})
//	defer pool.Close()
package database

import (
	"context"
	"fmt"

	"github.com/exaring/otelpgx"
	"github.com/jackc/pgx/v5/pgxpool"
)

// New tạo *pgxpool.Pool theo Config, ping kiểm tra kết nối trước khi trả về.
// Bật otelpgx khi Config.Tracing = true.
func New(ctx context.Context, cfg Config) (*pgxpool.Pool, error) {
	poolCfg, err := pgxpool.ParseConfig(cfg.DSNString())
	if err != nil {
		return nil, fmt.Errorf("database: parse config: %w", err)
	}

	if cfg.MaxConns > 0 {
		poolCfg.MaxConns = cfg.MaxConns
	}
	if cfg.MinConns > 0 {
		poolCfg.MinConns = cfg.MinConns
	}
	if cfg.MaxConnLifetime > 0 {
		poolCfg.MaxConnLifetime = cfg.MaxConnLifetime
	}
	if cfg.MaxConnIdleTime > 0 {
		poolCfg.MaxConnIdleTime = cfg.MaxConnIdleTime
	}
	if cfg.ConnectTimeout > 0 {
		poolCfg.ConnConfig.ConnectTimeout = cfg.ConnectTimeout
	}

	if cfg.Tracing {
		poolCfg.ConnConfig.Tracer = otelpgx.NewTracer()
	}

	pool, err := pgxpool.NewWithConfig(ctx, poolCfg)
	if err != nil {
		return nil, fmt.Errorf("database: create pool: %w", err)
	}

	pingCtx := ctx
	if cfg.ConnectTimeout > 0 {
		var cancel context.CancelFunc
		pingCtx, cancel = context.WithTimeout(ctx, cfg.ConnectTimeout)
		defer cancel()
	}
	if err := pool.Ping(pingCtx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("database: ping: %w", err)
	}

	return pool, nil
}
