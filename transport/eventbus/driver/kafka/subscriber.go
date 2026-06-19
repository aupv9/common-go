package kafka

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"strconv"
	"sync"
	"time"

	"github.com/aupv9/common-go/logger"
	"github.com/aupv9/common-go/transport/eventbus"
	kafkago "github.com/segmentio/kafka-go"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/trace"
)

// Subscriber implement eventbus.Subscriber trên kafka.Reader (consumer group),
// commit thủ công để đạt at-least-once: chỉ commit khi handler thành công hoặc
// message đã được chuyển sang DLQ.
type Subscriber struct {
	cfg    Config
	log    *slog.Logger
	tracer trace.Tracer
	// pub dùng để đẩy DLQ và/hoặc retry-topic; nil nếu không bật cái nào.
	pub eventbus.Publisher

	mu      sync.Mutex
	readers []*kafkago.Reader
	wg      sync.WaitGroup
}

// retryCountHeader là header đếm số lần đã retry (cho AsyncRetry).
const retryCountHeader = "x-retry-count"

// NewSubscriber tạo Subscriber. Tạo sẵn một Publisher khi cần DLQ hoặc AsyncRetry.
func NewSubscriber(cfg Config, log *slog.Logger) *Subscriber {
	var pub eventbus.Publisher
	if cfg.DLQEnabled || cfg.AsyncRetry {
		pub = NewPublisher(cfg)
	}
	return &Subscriber{
		cfg:    cfg,
		log:    log,
		tracer: otel.Tracer(tracerName),
		pub:    pub,
	}
}

// Subscribe mở Concurrency reader (mặc định 1) cho topic+group, mỗi reader chạy
// vòng nhận trong goroutine riêng. Nhiều reader trong cùng group → Kafka chia
// partition cho từng reader, xử lý song song nhưng giữ thứ tự trong mỗi partition.
func (s *Subscriber) Subscribe(ctx context.Context, topic, group string, h eventbus.Handler) error {
	n := s.cfg.Concurrency
	if n < 1 {
		n = 1
	}
	for i := 0; i < n; i++ {
		r := kafkago.NewReader(kafkago.ReaderConfig{
			Brokers: s.cfg.Brokers,
			GroupID: group,
			Topic:   topic,
		})

		s.mu.Lock()
		s.readers = append(s.readers, r)
		s.mu.Unlock()

		s.wg.Add(1)
		go s.consume(ctx, r, topic, h)
	}
	return nil
}

// consume vòng lặp nhận message tới khi ctx huỷ hoặc reader đóng.
func (s *Subscriber) consume(ctx context.Context, r *kafkago.Reader, topic string, h eventbus.Handler) {
	defer s.wg.Done()
	for {
		m, err := r.FetchMessage(ctx)
		if err != nil {
			// ctx huỷ hoặc reader đã đóng → kết thúc êm.
			if ctx.Err() != nil || errors.Is(err, io.EOF) || errors.Is(err, io.ErrClosedPipe) {
				return
			}
			s.log.ErrorContext(ctx, "kafka fetch error", slog.String("topic", topic), slog.Any("error", err))
			return
		}

		s.processMessage(ctx, r, topic, m, h)
	}
}

// processMessage xử lý một message: extract trace, retry handler, DLQ nếu fail,
// rồi commit. Tách hàm để defer span chạy đúng phạm vi từng message.
func (s *Subscriber) processMessage(ctx context.Context, r *kafkago.Reader, topic string, m kafkago.Message, h eventbus.Handler) {
	ebMsg := eventbus.Message{
		Key:     m.Key,
		Value:   m.Value,
		Headers: fromKafkaHeaders(m.Headers),
	}

	msgCtx := eventbus.ExtractTrace(ctx, ebMsg)
	msgCtx = logger.IntoContext(msgCtx, s.log)
	if s.cfg.Tracing {
		var span trace.Span
		msgCtx, span = s.tracer.Start(msgCtx, topic+" process", trace.WithSpanKind(trace.SpanKindConsumer))
		defer span.End()
	}

	// AsyncRetry: chạy handler MỘT lần (không sleep chặn partition); thất bại thì
	// đẩy retry-topic/DLQ. Mặc định: retry inline có backoff.
	var hErr error
	if s.cfg.AsyncRetry {
		hErr = h(msgCtx, ebMsg)
	} else {
		hErr = s.handleWithRetry(msgCtx, ebMsg, h)
	}

	if hErr != nil {
		// routeFailed trả lỗi nếu KHÔNG route được (retry/DLQ publish fail) → không
		// commit để message được giao lại sau.
		if rErr := s.routeFailed(msgCtx, topic, ebMsg); rErr != nil {
			s.log.ErrorContext(msgCtx, "kafka route-failed publish error; will redeliver",
				slog.String("topic", topic), slog.Any("error", rErr))
			return
		}
	}

	if err := r.CommitMessages(ctx, m); err != nil {
		s.log.ErrorContext(ctx, "kafka commit failed", slog.String("topic", topic), slog.Any("error", err))
	}
}

// routeFailed quyết định nơi đẩy message thất bại:
//   - AsyncRetry & chưa vượt RetryMax → retry-topic (tăng x-retry-count);
//   - ngược lại → DLQ (nếu bật), hoặc drop (log) nếu DLQ tắt.
//
// Trả lỗi nếu việc publish thất bại (caller sẽ không commit). Trả nil khi đã
// route xong hoặc chủ động drop.
func (s *Subscriber) routeFailed(ctx context.Context, topic string, msg eventbus.Message) error {
	if s.cfg.AsyncRetry {
		count := retryCount(msg)
		if count < s.cfg.RetryMax && s.pub != nil {
			next := msg
			next.Headers = cloneHeaders(msg.Headers)
			setRetryCount(&next, count+1)
			rt := s.cfg.retryTopic(topic)
			if err := s.pub.Publish(ctx, rt, next); err != nil {
				return err
			}
			s.log.WarnContext(ctx, "message sent to retry-topic",
				slog.String("topic", topic), slog.String("retry_topic", rt), slog.Int("attempt", count+1))
			return nil
		}
		// vượt RetryMax → rơi xuống DLQ bên dưới.
	}

	if s.cfg.DLQEnabled && s.pub != nil {
		dlq := s.cfg.dlqTopic(topic)
		if err := s.pub.Publish(ctx, dlq, msg); err != nil {
			return err
		}
		s.log.WarnContext(ctx, "message routed to DLQ", slog.String("topic", topic), slog.String("dlq", dlq))
		return nil
	}

	s.log.ErrorContext(ctx, "message dropped after retries (DLQ disabled)", slog.String("topic", topic))
	return nil
}

// retryCount đọc header x-retry-count (0 nếu không có/không hợp lệ).
func retryCount(msg eventbus.Message) int {
	if msg.Headers == nil {
		return 0
	}
	n, _ := strconv.Atoi(msg.Headers[retryCountHeader])
	return n
}

// setRetryCount ghi header x-retry-count.
func setRetryCount(msg *eventbus.Message, n int) {
	if msg.Headers == nil {
		msg.Headers = map[string]string{}
	}
	msg.Headers[retryCountHeader] = strconv.Itoa(n)
}

// cloneHeaders copy map header để không sửa message gốc.
func cloneHeaders(h map[string]string) map[string]string {
	out := make(map[string]string, len(h)+1)
	for k, v := range h {
		out[k] = v
	}
	return out
}

// handleWithRetry gọi handler, retry tối đa RetryMax lần với backoff. Tôn trọng
// ctx huỷ. Trả lỗi cuối cùng nếu vẫn fail.
func (s *Subscriber) handleWithRetry(ctx context.Context, msg eventbus.Message, h eventbus.Handler) error {
	var err error
	for attempt := 0; attempt <= s.cfg.RetryMax; attempt++ {
		if err = h(ctx, msg); err == nil {
			return nil
		}
		if attempt == s.cfg.RetryMax {
			break
		}
		if !sleepCtx(ctx, s.cfg.RetryBackoff) {
			return err
		}
	}
	return err
}

// Close dừng mọi reader, chờ goroutine kết thúc và đóng DLQ publisher.
func (s *Subscriber) Close() error {
	s.mu.Lock()
	readers := s.readers
	s.readers = nil
	s.mu.Unlock()

	var firstErr error
	for _, r := range readers {
		if err := r.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	s.wg.Wait()
	if s.pub != nil {
		if err := s.pub.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

// sleepCtx chờ d hoặc tới khi ctx huỷ; trả false nếu ctx huỷ trước.
func sleepCtx(ctx context.Context, d time.Duration) bool {
	if d <= 0 {
		return ctx.Err() == nil
	}
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-t.C:
		return true
	}
}
