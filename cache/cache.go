// Package cache cung cấp client Redis production-ready (go-redis): pool tuning,
// tracing/metrics qua OTel (redisotel), health checker và distributed lock.
//
// Hỗ trợ cả standalone (1 addr) lẫn cluster (nhiều addr) qua UniversalClient.
//
//	rdb, err := cache.New(ctx, cache.Config{Addrs: []string{"localhost:6379"}})
//	rdb.Set(ctx, "k", "v", time.Minute)
package cache

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/extra/redisotel/v9"
	"github.com/redis/go-redis/v9"
)

// Config cấu hình client Redis.
type Config struct {
	Addrs        []string // 1 = standalone, nhiều = cluster
	Password     string
	DB           int // chỉ standalone
	PoolSize     int
	MinIdleConns int
	DialTimeout  time.Duration
	ReadTimeout  time.Duration
	WriteTimeout time.Duration

	// Tracing bật redisotel (span + metrics).
	Tracing bool
}

// DefaultConfig trả cấu hình mặc định hợp lý.
func DefaultConfig() Config {
	return Config{
		Addrs:        []string{"localhost:6379"},
		PoolSize:     10,
		MinIdleConns: 2,
		DialTimeout:  5 * time.Second,
		ReadTimeout:  3 * time.Second,
		WriteTimeout: 3 * time.Second,
	}
}

// New tạo redis.UniversalClient theo Config, ping kiểm tra kết nối. Bật
// redisotel khi Config.Tracing = true.
func New(ctx context.Context, cfg Config) (redis.UniversalClient, error) {
	rdb := redis.NewUniversalClient(&redis.UniversalOptions{
		Addrs:        cfg.Addrs,
		Password:     cfg.Password,
		DB:           cfg.DB,
		PoolSize:     cfg.PoolSize,
		MinIdleConns: cfg.MinIdleConns,
		DialTimeout:  cfg.DialTimeout,
		ReadTimeout:  cfg.ReadTimeout,
		WriteTimeout: cfg.WriteTimeout,
	})

	if cfg.Tracing {
		if err := redisotel.InstrumentTracing(rdb); err != nil {
			_ = rdb.Close()
			return nil, fmt.Errorf("cache: instrument tracing: %w", err)
		}
		if err := redisotel.InstrumentMetrics(rdb); err != nil {
			_ = rdb.Close()
			return nil, fmt.Errorf("cache: instrument metrics: %w", err)
		}
	}

	pingCtx := ctx
	if cfg.DialTimeout > 0 {
		var cancel context.CancelFunc
		pingCtx, cancel = context.WithTimeout(ctx, cfg.DialTimeout)
		defer cancel()
	}
	if err := rdb.Ping(pingCtx).Err(); err != nil {
		_ = rdb.Close()
		return nil, fmt.Errorf("cache: ping: %w", err)
	}

	return rdb, nil
}
