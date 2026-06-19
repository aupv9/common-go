package grpcserver_test

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/aupv9/common-go/transport/grpcserver"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/status"
)

func startServer(t *testing.T, opts ...grpcserver.Option) (*grpc.ClientConn, *grpcserver.Server) {
	t.Helper()
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	all := append([]grpcserver.Option{grpcserver.WithReflection(false)}, opts...)
	srv, err := grpcserver.New(discardLogger(), all...)
	if err != nil {
		t.Fatal(err)
	}
	go func() { _ = srv.Server().Serve(lis) }()
	t.Cleanup(func() { _ = srv.Shutdown(context.Background()) })

	conn, err := grpc.NewClient(lis.Addr().String(), grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	return conn, srv
}

func TestRecoveryInterceptor(t *testing.T) {
	conn, _ := startServer(t, grpcserver.WithUnaryInterceptor(
		func(context.Context, any, *grpc.UnaryServerInfo, grpc.UnaryHandler) (any, error) {
			panic("boom in handler")
		},
	))
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	_, err := grpc_health_v1.NewHealthClient(conn).Check(ctx, &grpc_health_v1.HealthCheckRequest{})
	if status.Code(err) != codes.Internal {
		t.Fatalf("code = %v, want Internal (recovered panic)", status.Code(err))
	}
}

func TestStreamInterceptorsRoundTrip(t *testing.T) {
	conn, srv := startServer(t)
	srv.Health().SetServingStatus("", grpc_health_v1.HealthCheckResponse_SERVING)

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	// Watch là server-streaming → đi qua stream interceptor chain + wrappedStream.
	stream, err := grpc_health_v1.NewHealthClient(conn).Watch(ctx, &grpc_health_v1.HealthCheckRequest{})
	if err != nil {
		t.Fatalf("Watch: %v", err)
	}
	resp, err := stream.Recv()
	if err != nil {
		t.Fatalf("Recv: %v", err)
	}
	if resp.Status != grpc_health_v1.HealthCheckResponse_SERVING {
		t.Errorf("status = %v, want SERVING", resp.Status)
	}
}
