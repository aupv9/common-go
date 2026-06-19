package config_test

import (
	"bytes"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/aupv9/common-go/config"
)

func TestResolveSecretPlain(t *testing.T) {
	got, err := config.ResolveSecret("plain-value")
	if err != nil || got != "plain-value" {
		t.Fatalf("got (%q,%v)", got, err)
	}
}

func TestResolveSecretFromFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "pw")
	if err := os.WriteFile(path, []byte("s3cret\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := config.ResolveSecret("file:" + path)
	if err != nil {
		t.Fatalf("ResolveSecret: %v", err)
	}
	if got != "s3cret" {
		t.Errorf("got %q, want s3cret (trailing newline trimmed)", got)
	}
}

func TestResolveSecretMissingFile(t *testing.T) {
	if _, err := config.ResolveSecret("file:/no/such/file"); err == nil {
		t.Error("expected error for missing secret file")
	}
	// ResolveSecretOr nuốt lỗi, trả nguyên giá trị.
	if got := config.ResolveSecretOr("file:/no/such/file"); got != "file:/no/such/file" {
		t.Errorf("ResolveSecretOr fallback = %q", got)
	}
}

func TestDatabaseSectionRedaction(t *testing.T) {
	var buf bytes.Buffer
	log := slog.New(slog.NewJSONHandler(&buf, nil))
	log.Info("cfg", slog.Any("db", config.DatabaseSection{
		Host: "h", User: "u", Password: "supersecret", DSN: "postgres://u:p@h/db",
	}))
	out := buf.String()
	if strings.Contains(out, "supersecret") {
		t.Errorf("password leaked in log: %s", out)
	}
	if strings.Contains(out, "u:p@h") {
		t.Errorf("DSN credentials leaked in log: %s", out)
	}
	if !strings.Contains(out, "***") {
		t.Errorf("expected masked marker in log: %s", out)
	}
}

func TestAuthSectionRedaction(t *testing.T) {
	var buf bytes.Buffer
	log := slog.New(slog.NewJSONHandler(&buf, nil))
	log.Info("cfg", slog.Any("auth", config.AuthSection{Secret: "hmac-key-123", Algorithm: "HS256"}))
	if strings.Contains(buf.String(), "hmac-key-123") {
		t.Errorf("auth secret leaked: %s", buf.String())
	}
}
