package kafka

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/aupv9/common-go/transport/eventbus"
)

func TestHandleWithRetrySucceedsAfterFailures(t *testing.T) {
	s := &Subscriber{cfg: Config{RetryMax: 3, RetryBackoff: time.Millisecond}}

	var calls int
	h := func(_ context.Context, _ eventbus.Message) error {
		calls++
		if calls < 3 {
			return errors.New("transient")
		}
		return nil
	}

	if err := s.handleWithRetry(context.Background(), eventbus.Message{}, h); err != nil {
		t.Fatalf("expected success, got %v", err)
	}
	if calls != 3 {
		t.Errorf("calls = %d, want 3", calls)
	}
}

func TestHandleWithRetryExhausts(t *testing.T) {
	s := &Subscriber{cfg: Config{RetryMax: 2, RetryBackoff: time.Millisecond}}

	var calls int
	wantErr := errors.New("permanent")
	h := func(_ context.Context, _ eventbus.Message) error {
		calls++
		return wantErr
	}

	err := s.handleWithRetry(context.Background(), eventbus.Message{}, h)
	if !errors.Is(err, wantErr) {
		t.Fatalf("err = %v, want %v", err, wantErr)
	}
	// 1 lần đầu + 2 retry = 3.
	if calls != 3 {
		t.Errorf("calls = %d, want 3", calls)
	}
}

func TestHandleWithRetryStopsOnCanceledContext(t *testing.T) {
	s := &Subscriber{cfg: Config{RetryMax: 5, RetryBackoff: time.Second}}
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // huỷ ngay

	var calls int
	h := func(_ context.Context, _ eventbus.Message) error {
		calls++
		return errors.New("fail")
	}

	_ = s.handleWithRetry(ctx, eventbus.Message{}, h)
	// Lần đầu chạy, rồi sleepCtx thấy ctx huỷ → dừng. calls = 1.
	if calls != 1 {
		t.Errorf("calls = %d, want 1 (stop on canceled ctx)", calls)
	}
}
