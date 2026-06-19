package database

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Checker implement health.Checker: ping pool để xác định DB sẵn sàng.
type Checker struct {
	name string
	pool *pgxpool.Pool
}

// NewChecker tạo health checker cho pool. name mặc định "database".
func NewChecker(pool *pgxpool.Pool, name string) *Checker {
	if name == "" {
		name = "database"
	}
	return &Checker{name: name, pool: pool}
}

// Name định danh checker trong readiness response.
func (c *Checker) Name() string { return c.name }

// Check ping DB; lỗi nếu không kết nối được.
func (c *Checker) Check(ctx context.Context) error {
	if c.pool == nil {
		return fmt.Errorf("database: pool not initialized")
	}
	return c.pool.Ping(ctx)
}
