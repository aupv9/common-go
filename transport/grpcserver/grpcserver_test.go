package grpcserver_test

import (
	"context"
	"io"
	"log/slog"
	"net"
	"testing"
	"time"

	apperrors "github.com/aupv9/common-go/errors"
	"github.com/aupv9/common-go/transport/grpcserver"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/status"
)

func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func TestNewRequiresLogger(t *testing.T) {
	if _, err := grpcserver.New(nil); err == nil {
		t.Fatal("expected error for nil logger")
	}
}

func TestHealthServiceRoundTrip(t *testing.T) {
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	srv, err := grpcserver.New(discardLogger(), grpcserver.WithReflection(false))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	// Đánh dấu SERVING (Start() làm điều này, nhưng ở đây ta tự Serve listener sẵn).
	srv.Health().SetServingStatus("", grpc_health_v1.HealthCheckResponse_SERVING)

	go func() { _ = srv.Server().Serve(lis) }()
	t.Cleanup(func() { _ = srv.Shutdown(context.Background()) })

	conn, err := grpc.NewClient(lis.Addr().String(), grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	t.Cleanup(func() { _ = conn.Close() })

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	resp, err := grpc_health_v1.NewHealthClient(conn).Check(ctx, &grpc_health_v1.HealthCheckRequest{})
	if err != nil {
		t.Fatalf("health check: %v", err)
	}
	if resp.Status != grpc_health_v1.HealthCheckResponse_SERVING {
		t.Errorf("status = %v, want SERVING", resp.Status)
	}
}

func TestErrorMappingInterceptor(t *testing.T) {
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	srv, err := grpcserver.New(discardLogger(), grpcserver.WithReflection(false))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	// Đăng ký một service trả AppError NotFound qua health.Watch? Đơn giản hơn:
	// dùng unary interceptor giả lập bằng cách gọi một method không tồn tại sẽ ra
	// Unimplemented. Thay vào đó, ta test toStatus gián tiếp qua một handler thật:
	// đăng ký health Check override không khả dụng → dùng cách khác.
	// Ở đây ta xác minh mapping qua errmap (đã test riêng), và chỉ smoke server.
	go func() { _ = srv.Server().Serve(lis) }()
	t.Cleanup(func() { _ = srv.Shutdown(context.Background()) })

	conn, _ := grpc.NewClient(lis.Addr().String(), grpc.WithTransportCredentials(insecure.NewCredentials()))
	t.Cleanup(func() { _ = conn.Close() })

	// Gọi method không đăng ký → Unimplemented (đi qua interceptor chain mà không panic).
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	err = conn.Invoke(ctx, "/no.such.Service/Method", &grpc_health_v1.HealthCheckRequest{}, &grpc_health_v1.HealthCheckResponse{})
	if status.Code(err) != codes.Unimplemented {
		t.Errorf("code = %v, want Unimplemented", status.Code(err))
	}
}

func TestToStatusViaPublicError(t *testing.T) {
	// Kiểm tra mapping AppError → gRPC code thông qua một service thật.
	srv, err := grpcserver.New(discardLogger(),
		grpcserver.WithReflection(false),
		grpcserver.WithUnaryInterceptor(func(ctx context.Context, req any, info *grpc.UnaryServerInfo, _ grpc.UnaryHandler) (any, error) {
			return nil, apperrors.NotFound("X_NOT_FOUND", "x not found")
		}),
	)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	lis, _ := net.Listen("tcp", "127.0.0.1:0")
	go func() { _ = srv.Server().Serve(lis) }()
	t.Cleanup(func() { _ = srv.Shutdown(context.Background()) })

	conn, _ := grpc.NewClient(lis.Addr().String(), grpc.WithTransportCredentials(insecure.NewCredentials()))
	t.Cleanup(func() { _ = conn.Close() })

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	_, err = grpc_health_v1.NewHealthClient(conn).Check(ctx, &grpc_health_v1.HealthCheckRequest{})
	if status.Code(err) != codes.NotFound {
		t.Fatalf("code = %v, want NotFound", status.Code(err))
	}
	if status.Convert(err).Message() != "x not found" {
		t.Errorf("message = %q", status.Convert(err).Message())
	}
}
