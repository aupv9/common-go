// Package config nạp cấu hình theo thứ tự ưu tiên: defaults → file YAML → env.
//
// Env có độ ưu tiên cao nhất (12-factor, hợp container/k8s). Mapping env: bỏ
// prefix, hạ thường, "__" → "." (nested). Ví dụ với prefix "APP_":
//
//	APP_SERVER__PORT=9090   ->  server.port = 9090
//	APP_LOG__LEVEL=debug    ->  log.level   = debug
//
// Dùng generic Load để nạp vào struct bất kỳ của service:
//
//	type ServiceConfig struct {
//	    config.AppConfig `koanf:",squash"`
//	    Stripe struct{ Key string `koanf:"key"` } `koanf:"stripe"`
//	}
//	cfg, err := config.Load[ServiceConfig](config.WithFile("config.yaml"))
package config

import (
	"fmt"
	"strings"

	"github.com/go-playground/validator/v10"
	"github.com/knadh/koanf/parsers/yaml"
	"github.com/knadh/koanf/providers/env"
	"github.com/knadh/koanf/providers/file"
	"github.com/knadh/koanf/providers/structs"
	"github.com/knadh/koanf/v2"
)

// validate là instance dùng chung (thread-safe, cache reflection).
var validate = validator.New()

// Load nạp config kiểu T theo precedence defaults → file → env rồi validate.
// Trả lỗi nếu file lỗi cú pháp, unmarshal thất bại, hoặc validation fail.
func Load[T any](opts ...Option) (*T, error) {
	c := defaultOptions()
	for _, o := range opts {
		o(&c)
	}

	k := koanf.New(c.delim)

	// 1. Defaults từ struct (đọc tag `koanf` của T qua zero value + override).
	if c.defaults != nil {
		if err := k.Load(structs.Provider(c.defaults, c.structTag), nil); err != nil {
			return nil, fmt.Errorf("config: load defaults: %w", err)
		}
	}

	// 2. File YAML (tuỳ chọn; không tồn tại thì bỏ qua nếu optional).
	if c.filePath != "" {
		if err := k.Load(file.Provider(c.filePath), yaml.Parser()); err != nil {
			if !c.fileOptional {
				return nil, fmt.Errorf("config: load file %q: %w", c.filePath, err)
			}
		}
	}

	// 3. Env (ưu tiên cao nhất).
	prefix := c.envPrefix
	delim := c.delim
	envCb := func(key string) string {
		key = strings.TrimPrefix(key, prefix)
		key = strings.ToLower(key)
		return strings.ReplaceAll(key, "__", delim)
	}
	if err := k.Load(env.Provider(prefix, delim, envCb), nil); err != nil {
		return nil, fmt.Errorf("config: load env: %w", err)
	}

	var out T
	if err := k.UnmarshalWithConf("", &out, koanf.UnmarshalConf{Tag: c.structTag}); err != nil {
		return nil, fmt.Errorf("config: unmarshal: %w", err)
	}

	if err := validate.Struct(&out); err != nil {
		return nil, fmt.Errorf("config: validation failed: %w", err)
	}

	return &out, nil
}
