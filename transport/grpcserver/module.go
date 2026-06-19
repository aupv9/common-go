package grpcserver

import (
	"context"
	"log/slog"

	"github.com/aupv9/common-go/config"
	"github.com/aupv9/common-go/tlsconfig"
	"go.uber.org/fx"
	"google.golang.org/grpc"
)

// Module cung cấp *Server và *grpc.Server, chạy server theo lifecycle. Service
// đăng ký implementation qua fx.Invoke nhận *grpc.Server.
//
// opts là option bổ sung, áp dụng sau khi map từ *config.AppConfig.
func Module(opts ...Option) fx.Option {
	return fx.Module("grpcserver",
		fx.Provide(func(cfg *config.AppConfig, log *slog.Logger) (*Server, error) {
			base := optionsFromConfig(cfg)
			return New(log, append(base, opts...)...)
		}),
		fx.Provide(func(s *Server) *grpc.Server { return s.Server() }),
		fx.Invoke(run),
	)
}

func optionsFromConfig(cfg *config.AppConfig) []Option {
	opts := []Option{
		WithAddr(cfg.GRPC.Host, cfg.GRPC.Port),
		WithReflection(cfg.GRPC.Reflection),
		WithShutdownTimeout(cfg.GRPC.ShutdownTimeout),
		WithTracing(cfg.Otel.Enabled),
	}
	if cfg.GRPC.MaxRecvMsgSize > 0 {
		opts = append(opts, WithMaxRecvMsgSize(cfg.GRPC.MaxRecvMsgSize))
	}
	if cfg.GRPC.MaxSendMsgSize > 0 {
		opts = append(opts, WithMaxSendMsgSize(cfg.GRPC.MaxSendMsgSize))
	}
	if cfg.GRPC.TLS.Enabled {
		opts = append(opts, WithTLS(tlsconfig.Config{
			Enabled:      true,
			CertFile:     cfg.GRPC.TLS.CertFile,
			KeyFile:      cfg.GRPC.TLS.KeyFile,
			ClientCAFile: cfg.GRPC.TLS.ClientCAFile,
		}))
	}
	return opts
}

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
					log.Error("grpc server exited with error", slog.Any("error", err))
				}
			}()
			return nil
		},
		OnStop: func(ctx context.Context) error {
			return s.Shutdown(ctx)
		},
	})
}
