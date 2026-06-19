package kafka

import (
	"testing"
	"time"
)

func TestDefaultConfig(t *testing.T) {
	c := DefaultConfig()
	if len(c.Brokers) == 0 {
		t.Error("expected default broker")
	}
	if c.RetryMax != 3 {
		t.Errorf("RetryMax = %d, want 3", c.RetryMax)
	}
	if !c.DLQEnabled {
		t.Error("DLQ should be enabled by default")
	}
	if c.RetryBackoff != 200*time.Millisecond {
		t.Errorf("RetryBackoff = %v", c.RetryBackoff)
	}
}

func TestDLQTopic(t *testing.T) {
	c := Config{DLQSuffix: ".DLQ"}
	if got := c.dlqTopic("orders.created"); got != "orders.created.DLQ" {
		t.Errorf("dlqTopic = %q", got)
	}

	// Suffix rỗng → mặc định ".DLQ".
	c2 := Config{}
	if got := c2.dlqTopic("x"); got != "x.DLQ" {
		t.Errorf("dlqTopic empty suffix = %q", got)
	}
}

func TestHeaderConversionRoundTrip(t *testing.T) {
	in := map[string]string{"traceparent": "abc", "x-request-id": "r1"}
	out := fromKafkaHeaders(toKafkaHeaders(in))
	if len(out) != len(in) {
		t.Fatalf("len = %d, want %d", len(out), len(in))
	}
	for k, v := range in {
		if out[k] != v {
			t.Errorf("header %q = %q, want %q", k, out[k], v)
		}
	}
}

func TestHeaderConversionEmpty(t *testing.T) {
	if toKafkaHeaders(nil) != nil {
		t.Error("toKafkaHeaders(nil) should be nil")
	}
	if fromKafkaHeaders(nil) != nil {
		t.Error("fromKafkaHeaders(nil) should be nil")
	}
}
