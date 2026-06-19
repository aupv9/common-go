package eventbus

import (
	"context"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
)

// headerCarrier adapter map[string]string sang propagation.TextMapCarrier để
// OTel inject/extract trace context qua message header.
type headerCarrier map[string]string

func (c headerCarrier) Get(key string) string { return c[key] }
func (c headerCarrier) Set(key, val string)   { c[key] = val }
func (c headerCarrier) Keys() []string {
	keys := make([]string, 0, len(c))
	for k := range c {
		keys = append(keys, k)
	}
	return keys
}

// InjectTrace bơm trace context hiện tại trong ctx vào msg.Headers (tạo map nếu
// chưa có). Gọi ở Publisher trước khi gửi.
func InjectTrace(ctx context.Context, msg *Message) {
	if msg.Headers == nil {
		msg.Headers = make(map[string]string)
	}
	otel.GetTextMapPropagator().Inject(ctx, headerCarrier(msg.Headers))
}

// ExtractTrace rút trace context từ msg.Headers ra ctx. Gọi ở Subscriber trước
// khi xử lý để consumer span nối được với producer span.
func ExtractTrace(ctx context.Context, msg Message) context.Context {
	if msg.Headers == nil {
		return ctx
	}
	return otel.GetTextMapPropagator().Extract(ctx, headerCarrier(msg.Headers))
}

// ensure interface tuân thủ tại compile-time.
var _ propagation.TextMapCarrier = headerCarrier(nil)
