//go:build integration

// Integration test cho cache — chạy với Redis thật.
//
//	TEST_REDIS_ADDR="localhost:6379" go test -tags integration ./cache/...
//
// Nếu chưa set TEST_REDIS_ADDR thì test tự skip.
package cache_test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/aupv9/common-go/cache"
)

func addrOrSkip(t *testing.T) string {
	t.Helper()
	addr := os.Getenv("TEST_REDIS_ADDR")
	if addr == "" {
		t.Skip("TEST_REDIS_ADDR not set; skipping integration test")
	}
	return addr
}

func TestIntegrationSetGet(t *testing.T) {
	addr := addrOrSkip(t)
	ctx := context.Background()
	rdb, err := cache.New(ctx, cache.Config{Addrs: []string{addr}})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer rdb.Close()

	if err := rdb.Set(ctx, "it:k", "v", time.Minute).Err(); err != nil {
		t.Fatalf("set: %v", err)
	}
	if got, _ := rdb.Get(ctx, "it:k").Result(); got != "v" {
		t.Errorf("get = %q, want v", got)
	}
}

func TestIntegrationLockLifecycle(t *testing.T) {
	addr := addrOrSkip(t)
	ctx := context.Background()
	rdb, err := cache.New(ctx, cache.Config{Addrs: []string{addr}})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer rdb.Close()

	lock, err := cache.AcquireLock(ctx, rdb, "it:lock", 5*time.Second)
	if err != nil {
		t.Fatalf("acquire: %v", err)
	}

	// Lần 2 phải thất bại (đang giữ).
	if _, err := cache.AcquireLock(ctx, rdb, "it:lock", 5*time.Second); err == nil {
		t.Error("second acquire should fail while held")
	}

	if err := lock.Refresh(ctx); err != nil {
		t.Errorf("refresh: %v", err)
	}
	if err := lock.Release(ctx); err != nil {
		t.Errorf("release: %v", err)
	}

	// Sau release giành lại được.
	lock2, err := cache.AcquireLock(ctx, rdb, "it:lock", time.Second)
	if err != nil {
		t.Errorf("re-acquire after release: %v", err)
	} else {
		_ = lock2.Release(ctx)
	}
}

func TestIntegrationRedisLimiter(t *testing.T) {
	addr := addrOrSkip(t)
	ctx := context.Background()
	rdb, err := cache.New(ctx, cache.Config{Addrs: []string{addr}})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer rdb.Close()

	lim := cache.NewRedisLimiter(rdb, 1, 2, "it:rl:")
	// Burst 2 → 2 lần đầu allow, lần 3 bị chặn.
	a1, _ := lim.Allow(ctx, "u")
	a2, _ := lim.Allow(ctx, "u")
	a3, _ := lim.Allow(ctx, "u")
	if !a1 || !a2 {
		t.Errorf("first two allow = %v,%v want true,true", a1, a2)
	}
	if a3 {
		t.Error("third should be limited")
	}
}
