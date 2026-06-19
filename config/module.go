package config

import (
	"os"

	"go.uber.org/fx"
)

// Module nạp AppConfig và cung cấp *AppConfig cho graph fx (logger, health...
// phụ thuộc vào nó). Dùng option mặc định:
//   - đọc env prefix "APP_"
//   - file YAML lấy từ env APP_CONFIG_FILE (nếu set), nếu không thử "config.yaml"
//     ở thư mục làm việc (optional — không có cũng không sao).
//
// Service cần cấu hình riêng có thể tự fx.Provide ServiceConfig của mình và
// dùng Provide(...) để truyền option tuỳ biến.
var Module = Provide()

// Provide trả fx.Option cung cấp *AppConfig với các option cho trước.
// opts được áp dụng sau các default; file mặc định lấy từ env APP_CONFIG_FILE.
func Provide(opts ...Option) fx.Option {
	return fx.Module("config",
		fx.Provide(func() (*AppConfig, error) {
			base := []Option{defaultFileOption()}
			return Load[AppConfig](append(base, opts...)...)
		}),
	)
}

// defaultFileOption chọn file config từ env APP_CONFIG_FILE, fallback "config.yaml".
func defaultFileOption() Option {
	path := os.Getenv("APP_CONFIG_FILE")
	if path == "" {
		path = "config.yaml"
	}
	return WithOptionalFile(path)
}
