// Package outbox triển khai transactional outbox cho eventbus: ghi message vào
// bảng outbox TRONG CÙNG transaction nghiệp vụ (atomic với thay đổi DB), rồi một
// Relay nền publish ra eventbus và đánh dấu đã gửi. Đảm bảo at-least-once mà
// không mất message khi publish lỗi/crash (consumer phải idempotent).
//
//	// trong transaction nghiệp vụ:
//	tx, _ := pool.Begin(ctx)
//	// ... ghi dữ liệu nghiệp vụ ...
//	outbox.Save(ctx, tx, outbox.Record{Topic: "orders.created", Value: payload})
//	tx.Commit(ctx)
//
//	// nền (qua outbox.Module): Relay đọc row chưa publish → Publisher.Publish.
package outbox

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jackc/pgx/v5/pgconn"
)

// Schema là DDL tạo bảng outbox (idempotent). Đưa vào migration của service.
const Schema = `
CREATE TABLE IF NOT EXISTS outbox_messages (
    id           BIGSERIAL PRIMARY KEY,
    topic        TEXT        NOT NULL,
    msg_key      BYTEA,
    payload      BYTEA       NOT NULL,
    headers      JSONB       NOT NULL DEFAULT '{}',
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    published_at TIMESTAMPTZ
);
CREATE INDEX IF NOT EXISTS idx_outbox_unpublished
    ON outbox_messages (id) WHERE published_at IS NULL;`

// Record là một message chờ publish.
type Record struct {
	Topic   string
	Key     []byte
	Value   []byte
	Headers map[string]string
}

// Execer là phần giao của pgx.Tx và pgxpool.Pool mà Save cần — cho phép gọi Save
// bằng transaction nghiệp vụ (khuyến nghị) hoặc pool trực tiếp.
type Execer interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
}

// Save chèn một record vào bảng outbox dùng db (thường là pgx.Tx của transaction
// nghiệp vụ để đảm bảo atomic).
func Save(ctx context.Context, db Execer, r Record) error {
	headers := r.Headers
	if headers == nil {
		headers = map[string]string{}
	}
	hJSON, err := json.Marshal(headers)
	if err != nil {
		return fmt.Errorf("outbox: marshal headers: %w", err)
	}
	_, err = db.Exec(ctx,
		`INSERT INTO outbox_messages (topic, msg_key, payload, headers) VALUES ($1, $2, $3, $4)`,
		r.Topic, r.Key, r.Value, hJSON)
	if err != nil {
		return fmt.Errorf("outbox: insert: %w", err)
	}
	return nil
}
