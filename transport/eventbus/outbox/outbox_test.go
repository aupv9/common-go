package outbox

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
)

type fakeExecer struct {
	sql  string
	args []any
}

func (f *fakeExecer) Exec(_ context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	f.sql = sql
	f.args = args
	return pgconn.CommandTag{}, nil
}

func TestSaveInsertsRecord(t *testing.T) {
	f := &fakeExecer{}
	err := Save(context.Background(), f, Record{
		Topic:   "orders.created",
		Key:     []byte("k1"),
		Value:   []byte("payload"),
		Headers: map[string]string{"trace": "abc"},
	})
	if err != nil {
		t.Fatalf("Save: %v", err)
	}
	if len(f.args) != 4 {
		t.Fatalf("args = %d, want 4", len(f.args))
	}
	if f.args[0] != "orders.created" {
		t.Errorf("topic arg = %v", f.args[0])
	}
	// headers (arg[3]) phải là JSON hợp lệ chứa key trace.
	var h map[string]string
	if err := json.Unmarshal(f.args[3].([]byte), &h); err != nil {
		t.Fatalf("headers not JSON: %v", err)
	}
	if h["trace"] != "abc" {
		t.Errorf("headers = %v", h)
	}
}

func TestSaveNilHeaders(t *testing.T) {
	f := &fakeExecer{}
	if err := Save(context.Background(), f, Record{Topic: "t", Value: []byte("v")}); err != nil {
		t.Fatalf("Save: %v", err)
	}
	// headers nil → JSON "{}".
	if string(f.args[3].([]byte)) != "{}" {
		t.Errorf("nil headers should marshal to {}, got %s", f.args[3].([]byte))
	}
}
