// Package grpcserver cung cấp gRPC server (grpc-go) với bộ interceptor chuẩn
// (context+request id, recovery, logging, error mapping), OTel tracing tuỳ chọn,
// health service (grpc.health.v1), reflection và graceful shutdown.
//
// Dùng qua fx:
//
//	fx.New(
//	    foundation.Core(),
//	    foundation.WithGRPC(),
//	    fx.Invoke(func(s *grpc.Server) { pb.RegisterFooServer(s, fooImpl) }),
//	).Run()
package grpcserver

import (
	"context"
	"fmt"
	"log/slog"
	"net"

	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/health"
	"google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/reflection"
)

// Server bọc *grpc.Server và quản lý vòng đời lắng nghe/dừng.
type Server struct {
	opts   options
	log    *slog.Logger
	grpc   *grpc.Server
	health *health.Server
}

// New tạo Server với interceptor chuẩn đã gắn theo thứ tự:
//
//	context+requestID → recovery → logging → errorMap → (extra) → handler
//
// Tracing (otelgrpc) gắn qua StatsHandler khi bật.
func New(log *slog.Logger, opts ...Option) (*Server, error) {
	if log == nil {
		return nil, fmt.Errorf("grpcserver: logger is required")
	}
	o := defaultOptions()
	for _, opt := range opts {
		opt(&o)
	}

	unary := []grpc.UnaryServerInterceptor{
		contextUnary(log),
		recoveryUnary(),
		loggingUnary(),
		errorMapUnary(),
	}
	unary = append(unary, o.extraUnary...)

	stream := []grpc.StreamServerInterceptor{
		contextStream(log),
		recoveryStream(),
		loggingStream(),
		errorMapStream(),
	}
	stream = append(stream, o.extraStream...)

	serverOpts := []grpc.ServerOption{
		grpc.ChainUnaryInterceptor(unary...),
		grpc.ChainStreamInterceptor(stream...),
	}
	if tlsCfg, err := o.tls.Build(); err != nil {
		return nil, err
	} else if tlsCfg != nil {
		serverOpts = append(serverOpts, grpc.Creds(credentials.NewTLS(tlsCfg)))
	}
	if o.tracing {
		serverOpts = append(serverOpts, grpc.StatsHandler(otelgrpc.NewServerHandler()))
	}
	if o.maxRecvMsgSize > 0 {
		serverOpts = append(serverOpts, grpc.MaxRecvMsgSize(o.maxRecvMsgSize))
	}
	if o.maxSendMsgSize > 0 {
		serverOpts = append(serverOpts, grpc.MaxSendMsgSize(o.maxSendMsgSize))
	}
	serverOpts = append(serverOpts, o.serverOpts...)

	gs := grpc.NewServer(serverOpts...)

	// Health service chuẩn của gRPC.
	hs := health.NewServer()
	grpc_health_v1.RegisterHealthServer(gs, hs)

	if o.reflection {
		reflection.Register(gs)
	}

	return &Server{opts: o, log: log, grpc: gs, health: hs}, nil
}

// Server trả *grpc.Server để service đăng ký implementation của mình.
func (s *Server) Server() *grpc.Server { return s.grpc }

// Health trả health server để set trạng thái SERVING/NOT_SERVING theo service.
func (s *Server) Health() *health.Server { return s.health }

// Addr trả địa chỉ lắng nghe.
func (s *Server) Addr() string { return net.JoinHostPort(s.opts.host, fmt.Sprintf("%d", s.opts.port)) }

// Listen mở listener TCP (bind cổng) ĐỒNG BỘ; tách khỏi Serve để lỗi bind được
// phát hiện ngay trong OnStart và làm fail startup thay vì bị nuốt trong goroutine.
func (s *Server) Listen() (net.Listener, error) {
	lis, err := net.Listen("tcp", s.Addr())
	if err != nil {
		return nil, fmt.Errorf("grpcserver: listen %s: %w", s.Addr(), err)
	}
	return lis, nil
}

// Serve phục vụ trên listener đã mở (blocking). Đánh dấu health SERVING khi bắt đầu.
func (s *Server) Serve(lis net.Listener) error {
	s.health.SetServingStatus("", grpc_health_v1.HealthCheckResponse_SERVING)
	s.log.Info("grpc server starting", slog.String("addr", s.Addr()))
	if err := s.grpc.Serve(lis); err != nil {
		return fmt.Errorf("grpcserver: serve: %w", err)
	}
	return nil
}

// Start tiện ích standalone: Listen rồi Serve (blocking). Trong fx, module dùng
// Listen (đồng bộ) + Serve (goroutine) để bắt lỗi bind.
func (s *Server) Start() error {
	lis, err := s.Listen()
	if err != nil {
		return err
	}
	return s.Serve(lis)
}

// Shutdown graceful stop trong hạn shutdownTimeout; quá hạn thì Stop cứng.
func (s *Server) Shutdown(ctx context.Context) error {
	s.log.Info("grpc server stopping")
	s.health.SetServingStatus("", grpc_health_v1.HealthCheckResponse_NOT_SERVING)

	done := make(chan struct{})
	go func() {
		s.grpc.GracefulStop()
		close(done)
	}()

	stopCtx, cancel := context.WithTimeout(ctx, s.opts.shutdownTimeout)
	defer cancel()
	select {
	case <-done:
		return nil
	case <-stopCtx.Done():
		s.grpc.Stop() // cưỡng bức nếu quá hạn
		return stopCtx.Err()
	}
}
