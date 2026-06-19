package cache

import (
	"context"
	"fmt"

	"github.com/redis/go-redis/v9"
)

// Checker implement health.Checker: ping Redis để xác định sẵn sàng.
type Checker struct {
	name string
	rdb  redis.UniversalClient
}

// NewChecker tạo health checker cho client. name mặc định "cache".
func NewChecker(rdb redis.UniversalClient, name string) *Checker {
	if name == "" {
		name = "cache"
	}
	return &Checker{name: name, rdb: rdb}
}

// Name định danh checker trong readiness response.
func (c *Checker) Name() string { return c.name }

// Check ping Redis; lỗi nếu không kết nối được.
func (c *Checker) Check(ctx context.Context) error {
	if c.rdb == nil {
		return fmt.Errorf("cache: client not initialized")
	}
	return c.rdb.Ping(ctx).Err()
}
