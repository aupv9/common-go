package database

import (
	"errors"
	"fmt"
	"strings"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/pgx/v5" // driver "pgx5"
	_ "github.com/golang-migrate/migrate/v4/source/file"     // source "file"
)

// Migrate chạy toàn bộ migration "up" từ migrationsPath lên database tại dsn.
// Idempotent: không có migration mới (ErrNoChange) không coi là lỗi.
//
// migrationsPath là thư mục chứa file migration golang-migrate
// (vd 0001_init.up.sql / 0001_init.down.sql).
func Migrate(dsn, migrationsPath string) error {
	if migrationsPath == "" {
		return fmt.Errorf("database: migrate: migrations path is empty")
	}

	source := "file://" + migrationsPath
	m, err := migrate.New(source, toMigrateURL(dsn))
	if err != nil {
		return fmt.Errorf("database: migrate init: %w", err)
	}
	defer func() {
		srcErr, dbErr := m.Close()
		_ = srcErr
		_ = dbErr
	}()

	if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return fmt.Errorf("database: migrate up: %w", err)
	}
	return nil
}

// toMigrateURL đổi scheme DSN sang "pgx5" mà driver golang-migrate đăng ký.
func toMigrateURL(dsn string) string {
	if strings.HasPrefix(dsn, "postgres://") {
		return "pgx5://" + strings.TrimPrefix(dsn, "postgres://")
	}
	if strings.HasPrefix(dsn, "postgresql://") {
		return "pgx5://" + strings.TrimPrefix(dsn, "postgresql://")
	}
	return dsn
}
