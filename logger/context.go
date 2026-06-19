package logger

import (
	"context"
	"log/slog"
	"sync/atomic"
)

type ctxKey int

const (
	loggerKey ctxKey = iota
	requestIDKey
	traceIDKey
)

// IntoContext gắn logger vào context để các tầng dưới lấy ra dùng.
func IntoContext(ctx context.Context, l *slog.Logger) context.Context {
	return context.WithValue(ctx, loggerKey, l)
}

// FromContext lấy logger từ context; fallback slog.Default() nếu chưa set.
func FromContext(ctx context.Context) *slog.Logger {
	if l, ok := ctx.Value(loggerKey).(*slog.Logger); ok && l != nil {
		return l
	}
	return slog.Default()
}

// WithRequestID gắn request_id vào context (sẽ xuất hiện trong mọi log record
// phát ra với context đó).
func WithRequestID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, requestIDKey, id)
}

// WithTraceID gắn trace_id vào context. Ở M1 thường để trống; M3 (OTel) sẽ
// điền tự động từ span context.
func WithTraceID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, traceIDKey, id)
}

// RequestIDFromContext lấy request_id đã gắn (rỗng nếu chưa có). Dùng ở tầng
// transport để điền meta response hoặc propagate sang downstream.
func RequestIDFromContext(ctx context.Context) string {
	if v, ok := ctx.Value(requestIDKey).(string); ok {
		return v
	}
	return ""
}

// TraceIDFromContext lấy trace_id đã gắn (rỗng nếu chưa có).
func TraceIDFromContext(ctx context.Context) string {
	if v, ok := ctx.Value(traceIDKey).(string); ok {
		return v
	}
	return ""
}

// ContextExtractor rút các field cần log ra từ context. Dùng để gắn nguồn field
// ngoài (vd observability bơm trace_id/span_id từ OTel span) mà không tạo phụ
// thuộc ngược: logger không cần biết tới OTel.
type ContextExtractor func(ctx context.Context) []slog.Attr

// extractors giữ snapshot bất biến các extractor qua atomic.Pointer: đọc ở Handle
// (hot path) không khóa, ghi (Register) hoán đổi copy-on-write. An toàn ngay cả
// khi đăng ký xảy ra đồng thời với logging.
var extractors atomic.Pointer[[]ContextExtractor]

// RegisterContextExtractor thêm một extractor chạy cho mọi log record. Thường gọi
// lúc khởi động (vd trong observability.Module) nhưng an toàn cho goroutine.
func RegisterContextExtractor(e ContextExtractor) {
	if e == nil {
		return
	}
	for {
		old := extractors.Load()
		next := make([]ContextExtractor, 0, lenPtr(old)+1)
		if old != nil {
			next = append(next, *old...)
		}
		next = append(next, e)
		if extractors.CompareAndSwap(old, &next) {
			return
		}
	}
}

func lenPtr(p *[]ContextExtractor) int {
	if p == nil {
		return 0
	}
	return len(*p)
}

// contextHandler là slog.Handler bọc handler gốc và bơm các field từ context
// (request_id, trace_id, và các extractor đã đăng ký) vào mỗi record.
type contextHandler struct {
	slog.Handler
}

// Handle thêm field context rồi uỷ quyền cho handler gốc.
func (h *contextHandler) Handle(ctx context.Context, r slog.Record) error {
	if v, ok := ctx.Value(requestIDKey).(string); ok && v != "" {
		r.AddAttrs(slog.String("request_id", v))
	}
	if v, ok := ctx.Value(traceIDKey).(string); ok && v != "" {
		r.AddAttrs(slog.String("trace_id", v))
	}
	if exs := extractors.Load(); exs != nil {
		for _, e := range *exs {
			r.AddAttrs(e(ctx)...)
		}
	}
	return h.Handler.Handle(ctx, r)
}

// WithAttrs giữ nguyên bọc contextHandler khi thêm attrs.
func (h *contextHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &contextHandler{Handler: h.Handler.WithAttrs(attrs)}
}

// WithGroup giữ nguyên bọc contextHandler khi mở group.
func (h *contextHandler) WithGroup(name string) slog.Handler {
	return &contextHandler{Handler: h.Handler.WithGroup(name)}
}
