//go:build integration

// Integration test cho database — chạy với Postgres thật.
//
//	TEST_POSTGRES_DSN="postgres://app:app@localhost:5432/app?sslmode=disable" \
//	    go test -tags integration ./database/...
//
// Nếu chưa set TEST_POSTGRES_DSN thì test tự skip (không fail). Dễ ghép CI với
// docker-compose hoặc testcontainers ở tầng CI.
package database_test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/aupv9/common-go/database"
)

func dsnOrSkip(t *testing.T) string {
	t.Helper()
	dsn := os.Getenv("TEST_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("TEST_POSTGRES_DSN not set; skipping integration test")
	}
	return dsn
}

func TestIntegrationPoolPingAndQuery(t *testing.T) {
	dsn := dsnOrSkip(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	pool, err := database.New(ctx, database.Config{DSN: dsn, MaxConns: 4, ConnectTimeout: 5 * time.Second})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer pool.Close()

	var one int
	if err := pool.QueryRow(ctx, "select 1").Scan(&one); err != nil {
		t.Fatalf("query: %v", err)
	}
	if one != 1 {
		t.Errorf("select 1 = %d", one)
	}
}

func TestIntegrationHealthChecker(t *testing.T) {
	dsn := dsnOrSkip(t)
	ctx := context.Background()
	pool, err := database.New(ctx, database.Config{DSN: dsn})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer pool.Close()

	if err := database.NewChecker(pool, "db").Check(ctx); err != nil {
		t.Errorf("checker: %v", err)
	}
}
