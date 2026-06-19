package httpserver

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/aupv9/common-go/config"
	"github.com/aupv9/common-go/health"
	"github.com/aupv9/common-go/logger"
	"github.com/aupv9/common-go/response"
	"github.com/aupv9/common-go/tlsconfig"
	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/fx"
)

// Module trả fx.Option cung cấp *Server và *gin.Engine, tự gắn health endpoint,
// cấu hình response meta (request_id/trace_id) và chạy server theo lifecycle.
//
// opts là option bổ sung, áp dụng sau khi đã map từ *config.AppConfig.
func Module(opts ...Option) fx.Option {
	return fx.Module("httpserver",
		fx.Provide(func(cfg *config.AppConfig, log *slog.Logger) (*Server, error) {
			base := optionsFromConfig(cfg)
			return New(log, append(base, opts...)...)
		}),
		fx.Provide(func(s *Server) *gin.Engine { return s.Engine() }),
		fx.Invoke(setupResponseMeta),
		fx.Invoke(mountRoutes),
		fx.Invoke(run),
	)
}

// optionsFromConfig dịch cấu hình server từ AppConfig sang option httpserver.
func optionsFromConfig(cfg *config.AppConfig) []Option {
	mode := gin.ReleaseMode
	if cfg.App.Env == "local" {
		mode = gin.DebugMode
	}
	opts := []Option{
		WithAddr(cfg.Server.Host, cfg.Server.Port),
		WithReadTimeout(cfg.Server.ReadTimeout),
		WithWriteTimeout(cfg.Server.WriteTimeout),
		WithShutdownTimeout(cfg.Server.ShutdownTimeout),
		WithMode(mode),
	}
	if cfg.Otel.Enabled {
		opts = append(opts, WithTracing(cfg.App.Name))
	}
	if cfg.Server.TLS.Enabled {
		opts = append(opts, WithTLS(tlsconfig.Config{
			Enabled:      true,
			CertFile:     cfg.Server.TLS.CertFile,
			KeyFile:      cfg.Server.TLS.KeyFile,
			ClientCAFile: cfg.Server.TLS.ClientCAFile,
		}))
	}
	return opts
}

// setupResponseMeta cấu hình response.Error/OK tự gắn request_id + trace_id vào
// envelope meta, đọc từ request context (do middleware RequestID/OTel điền).
func setupResponseMeta() {
	response.SetMetaFunc(func(r *http.Request) *response.Meta {
		if r == nil {
			return nil
		}
		ctx := r.Context()
		rid := logger.RequestIDFromContext(ctx)
		tid := traceIDFromContext(ctx)
		if rid == "" && tid == "" {
			return nil
		}
		return &response.Meta{RequestID: rid, TraceID: tid}
	})
}

// traceIDFromContext lấy trace_id từ OTel span context (rỗng nếu không có span
// hợp lệ). Đặt ở đây vì httpserver phụ thuộc observability/OTel, còn logger thì
// không — giữ logger độc lập với OTel.
func traceIDFromContext(ctx context.Context) string {
	sc := trace.SpanContextFromContext(ctx)
	if !sc.IsValid() {
		return ""
	}
	return sc.TraceID().String()
}

// mountRoutes gắn health endpoint + nhóm /admin (loglevel) lên engine. Nhóm
// /admin được bảo vệ bằng adminGuards (WithAdminGuard) nếu có.
func mountRoutes(s *Server, reg *health.Registry, lvl *logger.LevelControl) {
	e := s.Engine()
	e.GET("/healthz", gin.WrapF(reg.LivenessHandler()))
	e.GET("/readyz", gin.WrapF(reg.ReadinessHandler()))
	e.GET("/info", gin.WrapF(reg.InfoHandler()))

	// Quản trị runtime; bảo vệ bằng adminGuards (auth/IP allowlist) nếu được cấu hình.
	admin := e.Group("/admin", s.opts.adminGuards...)
	admin.Any("/loglevel", gin.WrapF(lvl.Handler()))
}

// run gắn vòng đời server vào fx: OnStart serve trong goroutine, OnStop graceful.
func run(lc fx.Lifecycle, s *Server, log *slog.Logger) {
	lc.Append(fx.Hook{
		OnStart: func(_ context.Context) error {
			// Bind đồng bộ: lỗi cổng → fail startup ngay (không bị nuốt).
			lis, err := s.Listen()
			if err != nil {
				return err
			}
			go func() {
				if err := s.Serve(lis); err != nil {
					log.Error("http server exited with error", slog.Any("error", err))
				}
			}()
			return nil
		},
		OnStop: func(ctx context.Context) error {
			return s.Shutdown(ctx)
		},
	})
}
