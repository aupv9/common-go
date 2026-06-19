package outbox

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/aupv9/common-go/transport/eventbus"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Relay đọc các message chưa publish trong bảng outbox và publish ra eventbus,
// đánh dấu đã gửi. Dùng SELECT ... FOR UPDATE SKIP LOCKED để nhiều instance chạy
// song song không tranh nhau cùng row.
type Relay struct {
	pool     *pgxpool.Pool
	pub      eventbus.Publisher
	log      *slog.Logger
	batch    int
	interval time.Duration
}

// Option tinh chỉnh Relay.
type Option func(*Relay)

// WithBatchSize đặt số message xử lý mỗi vòng (mặc định 100).
func WithBatchSize(n int) Option { return func(r *Relay) { r.batch = n } }

// WithInterval đặt chu kỳ poll (mặc định 1s).
func WithInterval(d time.Duration) Option { return func(r *Relay) { r.interval = d } }

// NewRelay tạo Relay.
func NewRelay(pool *pgxpool.Pool, pub eventbus.Publisher, log *slog.Logger, opts ...Option) *Relay {
	r := &Relay{pool: pool, pub: pub, log: log, batch: 100, interval: time.Second}
	for _, o := range opts {
		o(r)
	}
	return r
}

// Run lặp poll tới khi ctx huỷ.
func (r *Relay) Run(ctx context.Context) {
	ticker := time.NewTicker(r.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if n, err := r.RelayOnce(ctx); err != nil {
				r.log.ErrorContext(ctx, "outbox relay error", slog.Any("error", err))
			} else if n > 0 {
				r.log.DebugContext(ctx, "outbox relayed", slog.Int("count", n))
			}
		}
	}
}

// RelayOnce xử lý tối đa batch message chưa publish; trả số đã publish thành công.
// Publish rồi mới đánh dấu published_at trong cùng transaction: publish lỗi →
// rollback (message được thử lại lượt sau) → at-least-once.
func (r *Relay) RelayOnce(ctx context.Context) (int, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return 0, fmt.Errorf("outbox: begin: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck // rollback sau commit là no-op

	rows, err := tx.Query(ctx,
		`SELECT id, topic, msg_key, payload, headers
		   FROM outbox_messages
		  WHERE published_at IS NULL
		  ORDER BY id
		  LIMIT $1
		  FOR UPDATE SKIP LOCKED`, r.batch)
	if err != nil {
		return 0, fmt.Errorf("outbox: query: %w", err)
	}

	type item struct {
		id  int64
		msg eventbus.Message
		top string
	}
	var items []item
	for rows.Next() {
		var (
			id      int64
			topic   string
			key     []byte
			payload []byte
			hRaw    []byte
		)
		if err := rows.Scan(&id, &topic, &key, &payload, &hRaw); err != nil {
			rows.Close()
			return 0, fmt.Errorf("outbox: scan: %w", err)
		}
		headers := map[string]string{}
		_ = json.Unmarshal(hRaw, &headers)
		items = append(items, item{id: id, top: topic, msg: eventbus.Message{Key: key, Value: payload, Headers: headers}})
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return 0, fmt.Errorf("outbox: rows: %w", err)
	}

	published := 0
	for _, it := range items {
		if err := r.pub.Publish(ctx, it.top, it.msg); err != nil {
			// Publish lỗi → dừng, rollback (không đánh dấu) để thử lại lượt sau.
			return published, fmt.Errorf("outbox: publish id=%d: %w", it.id, err)
		}
		if _, err := tx.Exec(ctx, `UPDATE outbox_messages SET published_at = now() WHERE id = $1`, it.id); err != nil {
			return published, fmt.Errorf("outbox: mark id=%d: %w", it.id, err)
		}
		published++
	}

	if err := tx.Commit(ctx); err != nil {
		return 0, fmt.Errorf("outbox: commit: %w", err)
	}
	return published, nil
}
