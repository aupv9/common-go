package health

import (
	"github.com/aupv9/common-go/config"
	"go.uber.org/fx"
)

// Module cung cấp *Registry dựng từ *config.AppConfig. Đăng ký handler vào HTTP
// server là việc của tầng transport (M2); ở M1 example tự gắn handler.
var Module = fx.Module("health",
	fx.Provide(newFromConfig),
)

func newFromConfig(cfg *config.AppConfig) *Registry {
	return NewRegistry(cfg.App.Name, cfg.App.Env)
}
