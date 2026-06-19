package eventbus_test

import (
	"context"
	"sort"
	"testing"

	"github.com/aupv9/common-go/transport/eventbus"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
)

func TestTracePropagationRoundTrip(t *testing.T) {
	// Cài propagator W3C cho test (giống observability.New làm lúc khởi động).
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{}, propagation.Baggage{},
	))

	traceID, _ := trace.TraceIDFromHex("0123456789abcdef0123456789abcdef")
	spanID, _ := trace.SpanIDFromHex("0123456789abcdef")
	sc := trace.NewSpanContext(trace.SpanContextConfig{
		TraceID:    traceID,
		SpanID:     spanID,
		TraceFlags: trace.FlagsSampled,
		Remote:     true,
	})
	producerCtx := trace.ContextWithSpanContext(context.Background(), sc)

	// Producer inject vào message.
	msg := eventbus.Message{Value: []byte("payload")}
	eventbus.InjectTrace(producerCtx, &msg)
	if _, ok := msg.Headers["traceparent"]; !ok {
		t.Fatalf("expected traceparent header injected, got %v", msg.Headers)
	}

	// Consumer extract từ message (ctx mới, không có span).
	consumerCtx := eventbus.ExtractTrace(context.Background(), msg)
	got := trace.SpanContextFromContext(consumerCtx)
	if !got.IsValid() {
		t.Fatal("extracted span context invalid")
	}
	if got.TraceID() != traceID {
		t.Errorf("trace_id = %s, want %s", got.TraceID(), traceID)
	}
	if got.SpanID() != spanID {
		t.Errorf("span_id = %s, want %s", got.SpanID(), spanID)
	}
}

func TestExtractNoHeaders(t *testing.T) {
	ctx := eventbus.ExtractTrace(context.Background(), eventbus.Message{})
	if trace.SpanContextFromContext(ctx).IsValid() {
		t.Error("expected no valid span context from empty message")
	}
}

func TestInjectCreatesHeaderMap(t *testing.T) {
	// Không có propagator field nào? vẫn không panic và không nil-map deref.
	otel.SetTextMapPropagator(propagation.TraceContext{})
	msg := eventbus.Message{}
	eventbus.InjectTrace(context.Background(), &msg)
	if msg.Headers == nil {
		t.Error("Headers should be initialized after InjectTrace")
	}
	// Sắp xếp keys để deterministic (không assert nội dung vì không có span).
	keys := make([]string, 0, len(msg.Headers))
	for k := range msg.Headers {
		keys = append(keys, k)
	}
	sort.Strings(keys)
}
