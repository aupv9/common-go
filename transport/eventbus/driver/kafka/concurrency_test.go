package kafka

import (
	"context"
	"io"
	"log/slog"
	"testing"

	"github.com/aupv9/common-go/transport/eventbus"
)

func TestSubscribeSpawnsConcurrentReaders(t *testing.T) {
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	s := NewSubscriber(Config{
		Brokers:     []string{"127.0.0.1:0"}, // không kết nối được — chỉ kiểm số reader
		Concurrency: 3,
		DLQEnabled:  false,
	}, log)

	ctx, cancel := context.WithCancel(context.Background())
	h := func(context.Context, eventbus.Message) error { return nil }
	if err := s.Subscribe(ctx, "topic", "group", h); err != nil {
		t.Fatalf("Subscribe: %v", err)
	}

	// Reader được append đồng bộ trong Subscribe trước khi goroutine chạy.
	s.mu.Lock()
	n := len(s.readers)
	s.mu.Unlock()
	if n != 3 {
		t.Errorf("readers = %d, want 3", n)
	}

	cancel()
	_ = s.Close()
}

func TestConcurrencyDefault(t *testing.T) {
	if DefaultConfig().Concurrency != 1 {
		t.Errorf("default concurrency = %d, want 1", DefaultConfig().Concurrency)
	}
}
