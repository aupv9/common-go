// Package foundation gom các module nền tảng thành các option fx tiện dùng.
//
// Core() là nền tối thiểu mọi service cần. Các With*() (httpserver, database,
// grpc, eventbus, httpclient) sẽ được bổ sung ở các mốc M2–M5.
//
//	fx.New(
//	    foundation.Core(),
//	    // foundation.WithHTTP(),     // M2
//	    // foundation.WithPostgres(), // M4
//	    fx.Provide(...),
//	    fx.Invoke(...),
//	).Run()
package foundation

import (
	"log/slog"

	"github.com/aupv9/common-go/auth"
	"github.com/aupv9/common-go/cache"
	"github.com/aupv9/common-go/client/httpclient"
	"github.com/aupv9/common-go/config"
	"github.com/aupv9/common-go/database"
	"github.com/aupv9/common-go/health"
	"github.com/aupv9/common-go/logger"
	"github.com/aupv9/common-go/observability"
	"github.com/aupv9/common-go/transport/eventbus/driver/kafka"
	"github.com/aupv9/common-go/transport/eventbus/outbox"
	"github.com/aupv9/common-go/transport/grpcserver"
	"github.com/aupv9/common-go/transport/httpserver"
	"go.uber.org/fx"
	"go.uber.org/fx/fxevent"
)

// Core trả nền tảng tối thiểu: config + logger + observability + health, đồng
// thời route log nội bộ của fx qua slog để output đồng nhất.
//
// observability cài propagator + (tuỳ config) tracer/meter và bơm trace_id/
// span_id vào log; tắt mặc định nên local không cần collector.
//
// configOpts truyền tiếp cho config.Provide (vd nguồn file tuỳ biến).
func Core(configOpts ...config.Option) fx.Option {
	return fx.Options(
		config.Provide(configOpts...),
		logger.Module,
		observability.Module,
		health.Module,
		fx.WithLogger(func(l *slog.Logger) fxevent.Logger {
			return &fxevent.SlogLogger{Logger: l}
		}),
	)
}

// WithHTTP thêm HTTP server (gin) với middleware chuẩn, tự gắn health endpoint
// và chạy theo lifecycle. Service gắn route lên *gin.Engine qua fx.Invoke.
//
//	foundation.WithHTTP(httpserver.WithRequestTimeout(30*time.Second))
func WithHTTP(opts ...httpserver.Option) fx.Option {
	return httpserver.Module(opts...)
}

// WithHTTPClient cung cấp một *http.Client production-ready (retry + pool +
// timeout) cho graph fx.
func WithHTTPClient(opts ...httpclient.Option) fx.Option {
	return httpclient.Module(opts...)
}

// WithPostgres cung cấp *pgxpool.Pool: tạo + ping pool lúc khởi động, (tuỳ
// config) chạy migration, đăng ký health checker và đóng pool khi shutdown.
func WithPostgres(opts ...func(*database.Config)) fx.Option {
	return database.Module(opts...)
}

// WithGRPC thêm gRPC server với interceptor chuẩn, health service, reflection
// và graceful shutdown. Service đăng ký implementation qua fx.Invoke(*grpc.Server).
func WithGRPC(opts ...grpcserver.Option) fx.Option {
	return grpcserver.Module(opts...)
}

// WithEventBus cung cấp eventbus.Publisher + eventbus.Subscriber (driver Kafka)
// với trace propagation, at-least-once, retry và DLQ; đóng khi shutdown.
func WithEventBus(opts ...func(*kafka.Config)) fx.Option {
	return kafka.Module(opts...)
}

// WithAuth cung cấp auth.Verifier (JWT) cho graph. Service tự gắn
// auth.Middleware (HTTP) / auth.UnaryInterceptor (gRPC) cho route cần bảo vệ.
func WithAuth() fx.Option {
	return auth.Module
}

// WithCache cung cấp redis.UniversalClient (pool + otel + health checker), đóng
// khi shutdown.
func WithCache(opts ...func(*cache.Config)) fx.Option {
	return cache.Module(opts...)
}

// WithOutbox chạy transactional-outbox relay (cần WithPostgres + WithEventBus và
// bảng outbox từ migration). Publish message đã ghi trong transaction nghiệp vụ.
func WithOutbox(opts ...outbox.Option) fx.Option {
	return outbox.Module(opts...)
}
