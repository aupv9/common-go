package cache

import (
	"context"
	"testing"
	"time"
)

func TestLockAccessors(t *testing.T) {
	l := &Lock{key: "k1", token: "tok-abc", ttl: 30 * time.Second}
	if l.Key() != "k1" {
		t.Errorf("Key() = %q", l.Key())
	}
	if l.Token() != "tok-abc" {
		t.Errorf("Token() = %q", l.Token())
	}
}

// limiterIface khớp middleware.Limiter — kiểm RedisLimiter thoả mãn ở compile-time
// mà không import middleware (tránh phụ thuộc chéo).
type limiterIface interface {
	Allow(ctx context.Context, key string) (bool, error)
}

func TestRedisLimiterSatisfiesInterface(t *testing.T) {
	var _ limiterIface = (*RedisLimiter)(nil)
}

func TestNewRedisLimiterDefaultPrefix(t *testing.T) {
	l := NewRedisLimiter(nil, 10, 20, "")
	if l.prefix != "ratelimit:" {
		t.Errorf("prefix = %q, want default ratelimit:", l.prefix)
	}
	l2 := NewRedisLimiter(nil, 10, 20, "rl:")
	if l2.prefix != "rl:" {
		t.Errorf("prefix = %q, want rl:", l2.prefix)
	}
}
