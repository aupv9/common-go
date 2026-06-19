// Package kafka là driver Kafka (segmentio/kafka-go) cho eventbus: Publisher
// ghi message, Subscriber nhận theo consumer group với at-least-once, retry và
// dead-letter queue. Trace context tự propagate qua message header.
package kafka

import (
	"context"

	"github.com/aupv9/common-go/transport/eventbus"
	kafkago "github.com/segmentio/kafka-go"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/trace"
)

// tracerName là instrumentation scope cho span của driver.
const tracerName = "github.com/aupv9/common-go/transport/eventbus/driver/kafka"

// Publisher implement eventbus.Publisher trên *kafka.Writer.
type Publisher struct {
	cfg    Config
	writer *kafkago.Writer
	tracer trace.Tracer
}

// NewPublisher tạo Publisher. Balancer Hash để cùng Key → cùng partition (giữ
// thứ tự theo key). Topic set per-message nên một writer phục vụ mọi topic.
func NewPublisher(cfg Config) *Publisher {
	return &Publisher{
		cfg: cfg,
		writer: &kafkago.Writer{
			Addr:                   kafkago.TCP(cfg.Brokers...),
			Balancer:               &kafkago.Hash{},
			WriteTimeout:           cfg.WriteTimeout,
			RequiredAcks:           kafkago.RequireAll,
			AllowAutoTopicCreation: true,
		},
		tracer: otel.Tracer(tracerName),
	}
}

// Publish inject trace context vào headers, (tuỳ chọn) mở producer span và ghi
// message tới topic.
func (p *Publisher) Publish(ctx context.Context, topic string, msg eventbus.Message) error {
	if p.cfg.Tracing {
		var span trace.Span
		ctx, span = p.tracer.Start(ctx, topic+" publish", trace.WithSpanKind(trace.SpanKindProducer))
		defer span.End()
	}

	eventbus.InjectTrace(ctx, &msg)

	return p.writer.WriteMessages(ctx, kafkago.Message{
		Topic:   topic,
		Key:     msg.Key,
		Value:   msg.Value,
		Headers: toKafkaHeaders(msg.Headers),
	})
}

// Close flush buffer và đóng writer.
func (p *Publisher) Close() error { return p.writer.Close() }

// toKafkaHeaders chuyển map header sang []kafka.Header.
func toKafkaHeaders(h map[string]string) []kafkago.Header {
	if len(h) == 0 {
		return nil
	}
	out := make([]kafkago.Header, 0, len(h))
	for k, v := range h {
		out = append(out, kafkago.Header{Key: k, Value: []byte(v)})
	}
	return out
}

// fromKafkaHeaders chuyển []kafka.Header sang map header.
func fromKafkaHeaders(h []kafkago.Header) map[string]string {
	if len(h) == 0 {
		return nil
	}
	out := make(map[string]string, len(h))
	for _, kv := range h {
		out[kv.Key] = string(kv.Value)
	}
	return out
}
