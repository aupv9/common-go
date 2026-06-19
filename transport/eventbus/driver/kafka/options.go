package kafka

import (
	"strings"
	"time"
)

// Config cấu hình driver Kafka cho cả Publisher lẫn Subscriber.
type Config struct {
	Brokers []string

	// Retry handler ở phía consumer.
	RetryMax     int           // số lần thử lại handler trước khi bỏ/đẩy DLQ
	RetryBackoff time.Duration // backoff giữa các lần thử

	// Dead-letter queue: khi handler fail hết retry, đẩy message sang topic
	// "<topic><DLQSuffix>" rồi commit để không kẹt partition.
	DLQEnabled bool
	DLQSuffix  string

	// AsyncRetry: thay vì retry inline (sleep chặn partition), đẩy message thất
	// bại sang retry-topic "<topic><RetrySuffix>" kèm header đếm lần, commit ngay.
	// Service chạy consumer trên retry-topic (thường có delay) với cùng handler;
	// vượt RetryMax → DLQ. RetryMax khi đó là số lần retry async tối đa.
	AsyncRetry  bool
	RetrySuffix string

	// Tracing bật producer/consumer span (otel).
	Tracing bool

	// Concurrency là số reader song song cho mỗi Subscribe (mỗi reader nhận một
	// tập partition khác nhau trong group → tăng throughput, giữ thứ tự trong
	// từng partition). Mặc định 1.
	Concurrency int

	DialTimeout  time.Duration
	WriteTimeout time.Duration
	ReadTimeout  time.Duration
}

// DefaultConfig trả cấu hình mặc định hợp lý.
func DefaultConfig() Config {
	return Config{
		Brokers:      []string{"localhost:9092"},
		RetryMax:     3,
		RetryBackoff: 200 * time.Millisecond,
		DLQEnabled:   true,
		DLQSuffix:    ".DLQ",
		AsyncRetry:   false,
		RetrySuffix:  ".retry",
		Tracing:      false,
		Concurrency:  1,
		DialTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
		ReadTimeout:  10 * time.Second,
	}
}

// dlqTopic trả tên DLQ topic cho một topic gốc.
func (c Config) dlqTopic(topic string) string {
	suffix := c.DLQSuffix
	if suffix == "" {
		suffix = ".DLQ"
	}
	return topic + suffix
}

// retryTopic trả tên retry topic; idempotent — nếu topic đã là retry-topic thì
// giữ nguyên (message tái xử lý cùng một retry-topic tới khi vượt RetryMax → DLQ).
func (c Config) retryTopic(topic string) string {
	suffix := c.RetrySuffix
	if suffix == "" {
		suffix = ".retry"
	}
	if strings.HasSuffix(topic, suffix) {
		return topic
	}
	return topic + suffix
}
