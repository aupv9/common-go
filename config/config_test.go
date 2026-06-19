package config_test

import (
	"testing"

	"github.com/aupv9/common-go/config"
)

func TestLoadDefaults(t *testing.T) {
	t.Setenv("APP_CONFIG_FILE", "") // tránh đọc file ngoài ý muốn
	cfg, err := config.Load[config.AppConfig]()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.App.Name != "service" || cfg.Server.Port != 8080 || cfg.Log.Level != "info" {
		t.Fatalf("defaults sai: %+v", cfg)
	}
}

func TestEnvOverride(t *testing.T) {
	t.Setenv("APP_APP__NAME", "orders")
	t.Setenv("APP_SERVER__PORT", "9090")
	t.Setenv("APP_LOG__LEVEL", "debug")

	cfg, err := config.Load[config.AppConfig]()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.App.Name != "orders" {
		t.Errorf("App.Name = %q, want orders", cfg.App.Name)
	}
	if cfg.Server.Port != 9090 {
		t.Errorf("Server.Port = %d, want 9090", cfg.Server.Port)
	}
	if cfg.Log.Level != "debug" {
		t.Errorf("Log.Level = %q, want debug", cfg.Log.Level)
	}
}

func TestValidationFails(t *testing.T) {
	t.Setenv("APP_LOG__LEVEL", "verbose") // không thuộc oneof
	if _, err := config.Load[config.AppConfig](); err == nil {
		t.Fatal("mong đợi lỗi validation cho log.level không hợp lệ")
	}
}
