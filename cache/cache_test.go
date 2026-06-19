package cache

import (
	"context"
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	c := DefaultConfig()
	if len(c.Addrs) == 0 {
		t.Error("expected default addr")
	}
	if c.PoolSize == 0 {
		t.Error("expected default pool size")
	}
}

func TestCheckerNilClient(t *testing.T) {
	c := NewChecker(nil, "")
	if c.Name() != "cache" {
		t.Errorf("Name() = %q, want default 'cache'", c.Name())
	}
	if err := c.Check(context.Background()); err == nil {
		t.Error("expected error for nil client")
	}
}

func TestNewTokenUnique(t *testing.T) {
	a, err := newToken()
	if err != nil {
		t.Fatalf("newToken: %v", err)
	}
	b, _ := newToken()
	if a == b {
		t.Error("tokens should be unique")
	}
	if len(a) != 32 { // 16 byte hex
		t.Errorf("token len = %d, want 32", len(a))
	}
}
