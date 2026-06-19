package kafka

import (
	"context"
	"io"
	"log/slog"
	"testing"

	"github.com/aupv9/common-go/transport/eventbus"
)

type capturedPublish struct {
	topic string
	msg   eventbus.Message
}

type fakePublisher struct {
	published []capturedPublish
	err       error
}

func (f *fakePublisher) Publish(_ context.Context, topic string, msg eventbus.Message) error {
	if f.err != nil {
		return f.err
	}
	f.published = append(f.published, capturedPublish{topic: topic, msg: msg})
	return nil
}
func (f *fakePublisher) Close() error { return nil }

func newSubForTest(cfg Config, pub eventbus.Publisher) *Subscriber {
	return &Subscriber{cfg: cfg, log: slog.New(slog.NewTextHandler(io.Discard, nil)), pub: pub}
}

func TestRouteFailedAsyncRetryGoesToRetryTopic(t *testing.T) {
	pub := &fakePublisher{}
	s := newSubForTest(Config{AsyncRetry: true, RetrySuffix: ".retry", RetryMax: 3, DLQEnabled: true, DLQSuffix: ".DLQ"}, pub)

	// Lần đầu (count 0) → retry-topic với count=1.
	err := s.routeFailed(context.Background(), "orders.created", eventbus.Message{Value: []byte("x")})
	if err != nil {
		t.Fatalf("routeFailed: %v", err)
	}
	if len(pub.published) != 1 {
		t.Fatalf("published %d, want 1", len(pub.published))
	}
	got := pub.published[0]
	if got.topic != "orders.created.retry" {
		t.Errorf("topic = %q, want orders.created.retry", got.topic)
	}
	if got.msg.Headers[retryCountHeader] != "1" {
		t.Errorf("retry count = %q, want 1", got.msg.Headers[retryCountHeader])
	}
}

func TestRouteFailedAsyncRetryExhaustedGoesToDLQ(t *testing.T) {
	pub := &fakePublisher{}
	s := newSubForTest(Config{AsyncRetry: true, RetrySuffix: ".retry", RetryMax: 3, DLQEnabled: true, DLQSuffix: ".DLQ"}, pub)

	// Message đã ở count = RetryMax → vượt ngưỡng → DLQ.
	msg := eventbus.Message{Headers: map[string]string{retryCountHeader: "3"}}
	if err := s.routeFailed(context.Background(), "orders.created.retry", msg); err != nil {
		t.Fatalf("routeFailed: %v", err)
	}
	if len(pub.published) != 1 {
		t.Fatalf("published %d, want 1", len(pub.published))
	}
	if pub.published[0].topic != "orders.created.retry.DLQ" {
		t.Errorf("topic = %q, want ...retry.DLQ", pub.published[0].topic)
	}
}

func TestRouteFailedRetryTopicIdempotent(t *testing.T) {
	pub := &fakePublisher{}
	s := newSubForTest(Config{AsyncRetry: true, RetrySuffix: ".retry", RetryMax: 5}, pub)

	// Message đang ở retry-topic, count 1 → republish CÙNG retry-topic, count 2.
	msg := eventbus.Message{Headers: map[string]string{retryCountHeader: "1"}}
	_ = s.routeFailed(context.Background(), "orders.created.retry", msg)
	if pub.published[0].topic != "orders.created.retry" {
		t.Errorf("topic = %q, want orders.created.retry (idempotent)", pub.published[0].topic)
	}
	if pub.published[0].msg.Headers[retryCountHeader] != "2" {
		t.Errorf("count = %q, want 2", pub.published[0].msg.Headers[retryCountHeader])
	}
}

func TestRouteFailedDLQDisabledDrops(t *testing.T) {
	pub := &fakePublisher{}
	s := newSubForTest(Config{AsyncRetry: false, DLQEnabled: false}, pub)
	// Không DLQ, không async → drop (không publish), không lỗi.
	if err := s.routeFailed(context.Background(), "t", eventbus.Message{}); err != nil {
		t.Fatalf("routeFailed: %v", err)
	}
	if len(pub.published) != 0 {
		t.Errorf("should not publish when dropping, got %d", len(pub.published))
	}
}

func TestRouteFailedPublishErrorPropagates(t *testing.T) {
	pub := &fakePublisher{err: io.ErrClosedPipe}
	s := newSubForTest(Config{DLQEnabled: true, DLQSuffix: ".DLQ"}, pub)
	if err := s.routeFailed(context.Background(), "t", eventbus.Message{}); err == nil {
		t.Error("expected error when publish fails (so caller won't commit)")
	}
}
