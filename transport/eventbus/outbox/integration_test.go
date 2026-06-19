//go:build integration

// Integration test cho outbox — chạy với Postgres thật.
//
//	TEST_POSTGRES_DSN="postgres://app:app@localhost:5432/app?sslmode=disable" \
//	    go test -tags integration ./transport/eventbus/outbox/...
package outbox_test

import (
	"context"
	"io"
	"log/slog"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/aupv9/common-go/transport/eventbus"
	"github.com/aupv9/common-go/transport/eventbus/outbox"
	"github.com/jackc/pgx/v5/pgxpool"
)

type memPublisher struct {
	mu        sync.Mutex
	published []string
}

func (m *memPublisher) Publish(_ context.Context, topic string, _ eventbus.Message) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.published = append(m.published, topic)
	return nil
}
func (m *memPublisher) Close() error { return nil }

func TestOutboxSaveAndRelay(t *testing.T) {
	dsn := os.Getenv("TEST_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("TEST_POSTGRES_DSN not set")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("pool: %v", err)
	}
	defer pool.Close()

	if _, err := pool.Exec(ctx, outbox.Schema); err != nil {
		t.Fatalf("schema: %v", err)
	}
	_, _ = pool.Exec(ctx, "TRUNCATE outbox_messages")

	// Ghi message trong transaction.
	tx, _ := pool.Begin(ctx)
	if err := outbox.Save(ctx, tx, outbox.Record{Topic: "it.topic", Value: []byte("v")}); err != nil {
		t.Fatalf("Save: %v", err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatalf("commit: %v", err)
	}

	pub := &memPublisher{}
	relay := outbox.NewRelay(pool, pub, slog.New(slog.NewTextHandler(io.Discard, nil)))
	n, err := relay.RelayOnce(ctx)
	if err != nil {
		t.Fatalf("RelayOnce: %v", err)
	}
	if n != 1 || len(pub.published) != 1 || pub.published[0] != "it.topic" {
		t.Fatalf("relayed %d topics=%v", n, pub.published)
	}

	// Lượt 2 không còn gì để publish (đã đánh dấu).
	n2, _ := relay.RelayOnce(ctx)
	if n2 != 0 {
		t.Errorf("second relay = %d, want 0 (already published)", n2)
	}

	_ = time.Second
}
