package logger

import (
	"log/slog"

	"github.com/aupv9/common-go/config"
	"go.uber.org/fx"
)

// Module cung cấp *slog.Logger và *LevelControl dựng từ *config.AppConfig, đặt
// logger làm slog.Default() để code chưa nhận logger qua DI vẫn log đúng format.
var Module = fx.Module("logger",
	fx.Provide(newFromConfig),
	fx.Invoke(setDefault),
)

func newFromConfig(cfg *config.AppConfig) (*slog.Logger, *LevelControl) {
	return NewWithControl(Config{
		Level:   cfg.Log.Level,
		Format:  Format(cfg.Log.Format),
		Service: cfg.App.Name,
	})
}

func setDefault(l *slog.Logger) {
	slog.SetDefault(l)
}
