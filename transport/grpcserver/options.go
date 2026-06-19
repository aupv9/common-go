package grpcserver

import (
	"time"

	"github.com/aupv9/common-go/tlsconfig"
	"google.golang.org/grpc"
)

type options struct {
	host            string
	port            int
	reflection      bool
	tracing         bool
	shutdownTimeout time.Duration
	maxRecvMsgSize  int
	maxSendMsgSize  int

	// extra interceptor do service thêm, gắn SAU bộ chuẩn.
	extraUnary  []grpc.UnaryServerInterceptor
	extraStream []grpc.StreamServerInterceptor
	// serverOpts cho phép cấp thêm grpc.ServerOption thô (vd creds/TLS).
	serverOpts []grpc.ServerOption

	tls tlsconfig.Config
}

func defaultOptions() options {
	return options{
		host:            "0.0.0.0",
		port:            9090,
		reflection:      true,
		tracing:         false,
		shutdownTimeout: 10 * time.Second,
	}
}

// Option tinh chỉnh gRPC server.
type Option func(*options)

// WithAddr đặt host và port.
func WithAddr(host string, port int) Option {
	return func(o *options) { o.host = host; o.port = port }
}

// WithPort đặt port.
func WithPort(port int) Option { return func(o *options) { o.port = port } }

// WithReflection bật/tắt server reflection (nên tắt ở prod).
func WithReflection(enabled bool) Option { return func(o *options) { o.reflection = enabled } }

// WithTracing bật OTel interceptor (otelgrpc stats handler).
func WithTracing(enabled bool) Option { return func(o *options) { o.tracing = enabled } }

// WithShutdownTimeout đặt hạn cho GracefulStop trước khi Stop cứng.
func WithShutdownTimeout(d time.Duration) Option { return func(o *options) { o.shutdownTimeout = d } }

// WithMaxRecvMsgSize giới hạn kích thước message nhận (byte).
func WithMaxRecvMsgSize(n int) Option { return func(o *options) { o.maxRecvMsgSize = n } }

// WithMaxSendMsgSize giới hạn kích thước message gửi (byte).
func WithMaxSendMsgSize(n int) Option { return func(o *options) { o.maxSendMsgSize = n } }

// WithUnaryInterceptor thêm unary interceptor của service (sau bộ chuẩn).
func WithUnaryInterceptor(i ...grpc.UnaryServerInterceptor) Option {
	return func(o *options) { o.extraUnary = append(o.extraUnary, i...) }
}

// WithStreamInterceptor thêm stream interceptor của service (sau bộ chuẩn).
func WithStreamInterceptor(i ...grpc.StreamServerInterceptor) Option {
	return func(o *options) { o.extraStream = append(o.extraStream, i...) }
}

// WithServerOption cấp thêm grpc.ServerOption thô (vd transport credentials).
func WithServerOption(opt ...grpc.ServerOption) Option {
	return func(o *options) { o.serverOpts = append(o.serverOpts, opt...) }
}

// WithTLS bật TLS/mTLS cho gRPC server (xem tlsconfig.Config).
func WithTLS(cfg tlsconfig.Config) Option {
	return func(o *options) { o.tls = cfg }
}
