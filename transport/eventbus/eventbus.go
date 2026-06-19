// Package eventbus định nghĩa abstraction message/event độc lập driver:
// Publisher để phát, Subscriber để nhận, Message là đơn vị truyền.
//
// Driver cụ thể (Kafka...) nằm dưới eventbus/driver/*. Trace context tự được
// propagate qua message header (xem propagation.go) nên một consumer span nối
// được với producer span xuyên service.
//
//	pub.Publish(ctx, "orders.created", eventbus.Message{
//	    Key:   []byte(orderID),
//	    Value: payload,
//	})
//
//	sub.Subscribe(ctx, "orders.created", "billing", func(ctx context.Context, m eventbus.Message) error {
//	    return handle(ctx, m.Value)
//	})
package eventbus

import "context"

// Message là một đơn vị message/event. Key dùng để partition (cùng key → cùng
// partition, giữ thứ tự). Headers mang metadata (gồm trace context tự bơm).
type Message struct {
	Key     []byte
	Value   []byte
	Headers map[string]string
}

// Handler xử lý một message. Trả nil = thành công (sẽ được commit/ack); trả lỗi
// = thất bại (driver retry hoặc đẩy DLQ tuỳ policy).
type Handler func(ctx context.Context, msg Message) error

// Publisher phát message tới một topic.
type Publisher interface {
	// Publish gửi msg tới topic. Tự inject trace context vào headers.
	Publish(ctx context.Context, topic string, msg Message) error
	// Close giải phóng tài nguyên (flush buffer, đóng connection).
	Close() error
}

// Subscriber đăng ký nhận message từ topic theo consumer group.
type Subscriber interface {
	// Subscribe bắt đầu nhận message từ topic trong group, gọi h cho mỗi message.
	// Không blocking: driver chạy vòng nhận trong goroutine riêng. Dừng khi ctx
	// huỷ hoặc Close được gọi.
	Subscribe(ctx context.Context, topic, group string, h Handler) error
	// Close dừng mọi subscription và đóng connection.
	Close() error
}
