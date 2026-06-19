package observability_test

import (
	"context"
	"testing"

	"github.com/aupv9/common-go/observability"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
)

func TestNewDisabledIsNoop(t *testing.T) {
	p, err := observability.New(context.Background(), observability.Config{Enabled: false})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if p.Enabled() {
		t.Error("Enabled() = true, want false")
	}
	if err := p.Shutdown(context.Background()); err != nil {
		t.Errorf("Shutdown: %v", err)
	}
}

func TestNewSetsPropagator(t *testing.T) {
	_, err := observability.New(context.Background(), observability.Config{Enabled: false})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	prop := otel.GetTextMapPropagator()
	fields := prop.Fields()
	// Composite TraceContext+Baggage expose "traceparent" và "baggage".
	want := map[string]bool{"traceparent": false, "baggage": false}
	for _, f := range fields {
		if _, ok := want[f]; ok {
			want[f] = true
		}
	}
	for k, seen := range want {
		if !seen {
			t.Errorf("propagator missing field %q (fields=%v)", k, fields)
		}
	}
	_ = propagation.TraceContext{}
}

func TestSpanLogAttrs(t *testing.T) {
	// Không có span → nil.
	if got := observability.SpanLogAttrs(context.Background()); got != nil {
		t.Errorf("expected nil for context without span, got %v", got)
	}

	// Có span context hợp lệ → có trace_id/span_id.
	traceID, _ := trace.TraceIDFromHex("0123456789abcdef0123456789abcdef")
	spanID, _ := trace.SpanIDFromHex("0123456789abcdef")
	sc := trace.NewSpanContext(trace.SpanContextConfig{
		TraceID:    traceID,
		SpanID:     spanID,
		TraceFlags: trace.FlagsSampled,
	})
	ctx := trace.ContextWithSpanContext(context.Background(), sc)

	attrs := observability.SpanLogAttrs(ctx)
	if len(attrs) != 2 {
		t.Fatalf("attrs = %v, want 2", attrs)
	}
	found := map[string]string{}
	for _, a := range attrs {
		found[a.Key] = a.Value.String()
	}
	if found["trace_id"] != traceID.String() {
		t.Errorf("trace_id = %q, want %q", found["trace_id"], traceID.String())
	}
	if found["span_id"] != spanID.String() {
		t.Errorf("span_id = %q, want %q", found["span_id"], spanID.String())
	}
}
