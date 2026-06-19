package database

import (
	"context"
	"net/url"
	"strings"
	"testing"
)

func TestDSNStringPrefersExplicitDSN(t *testing.T) {
	c := Config{DSN: "postgres://u:p@h:5432/db?sslmode=disable"}
	if got := c.DSNString(); got != c.DSN {
		t.Errorf("DSNString() = %q, want %q", got, c.DSN)
	}
}

func TestDSNStringBuildsFromFields(t *testing.T) {
	c := Config{
		Host: "db.internal", Port: 6432, User: "app", Password: "s3cret",
		Name: "orders", SSLMode: "require", AppName: "order-svc",
	}
	raw := c.DSNString()
	u, err := url.Parse(raw)
	if err != nil {
		t.Fatalf("parse DSN %q: %v", raw, err)
	}
	if u.Scheme != "postgres" {
		t.Errorf("scheme = %q", u.Scheme)
	}
	if u.Host != "db.internal:6432" {
		t.Errorf("host = %q", u.Host)
	}
	if u.Path != "/orders" {
		t.Errorf("path = %q", u.Path)
	}
	if pw, _ := u.User.Password(); pw != "s3cret" {
		t.Errorf("password = %q", pw)
	}
	q := u.Query()
	if q.Get("sslmode") != "require" {
		t.Errorf("sslmode = %q", q.Get("sslmode"))
	}
	if q.Get("application_name") != "order-svc" {
		t.Errorf("application_name = %q", q.Get("application_name"))
	}
}

func TestToMigrateURL(t *testing.T) {
	cases := map[string]string{
		"postgres://u:p@h:5432/db":   "pgx5://u:p@h:5432/db",
		"postgresql://u:p@h:5432/db": "pgx5://u:p@h:5432/db",
		"pgx5://already":             "pgx5://already",
	}
	for in, want := range cases {
		if got := toMigrateURL(in); got != want {
			t.Errorf("toMigrateURL(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestMigrateEmptyPath(t *testing.T) {
	err := Migrate("postgres://u:p@h/db", "")
	if err == nil || !strings.Contains(err.Error(), "migrations path is empty") {
		t.Errorf("expected empty-path error, got %v", err)
	}
}

func TestCheckerNilPool(t *testing.T) {
	c := NewChecker(nil, "")
	if c.Name() != "database" {
		t.Errorf("Name() = %q, want default 'database'", c.Name())
	}
	if err := c.Check(context.Background()); err == nil {
		t.Error("expected error for nil pool")
	}
}
