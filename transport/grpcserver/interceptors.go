package grpcserver

import (
	"context"
	"log/slog"
	"runtime/debug"
	"time"

	"github.com/aupv9/common-go/logger"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

// metadataRequestID là khoá metadata mang request id (đồng bộ với HTTP header).
const metadataRequestID = "x-request-id"

// chainContext gắn logger nền + request id vào ctx của request. request id lấy
// từ metadata nếu có, ngược lại sinh mới và gửi lại qua response header.
func contextUnary(base *slog.Logger) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, _ *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		ctx = enrichContext(ctx, base)
		return handler(ctx, req)
	}
}

func contextStream(base *slog.Logger) grpc.StreamServerInterceptor {
	return func(srv any, ss grpc.ServerStream, _ *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		ctx := enrichContext(ss.Context(), base)
		return handler(srv, &wrappedStream{ServerStream: ss, ctx: ctx})
	}
}

// enrichContext gắn logger + request id (kèm gửi header response).
func enrichContext(ctx context.Context, base *slog.Logger) context.Context {
	ctx = logger.IntoContext(ctx, base)
	id := requestIDFromMD(ctx)
	if id == "" {
		id = newID()
	}
	ctx = logger.WithRequestID(ctx, id)
	_ = grpc.SetHeader(ctx, metadata.Pairs(metadataRequestID, id))
	return ctx
}

func requestIDFromMD(ctx context.Context) string {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return ""
	}
	if vals := md.Get(metadataRequestID); len(vals) > 0 {
		return vals[0]
	}
	return ""
}

// recoveryUnary bắt panic → codes.Internal, log stack server-side.
func recoveryUnary() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, _ *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp any, err error) {
		defer func() {
			if rec := recover(); rec != nil {
				logger.FromContext(ctx).ErrorContext(ctx, "grpc handler panic",
					slog.Any("panic", rec), slog.String("stack", string(debug.Stack())))
				err = status.Error(codes.Internal, "internal server error")
			}
		}()
		return handler(ctx, req)
	}
}

func recoveryStream() grpc.StreamServerInterceptor {
	return func(srv any, ss grpc.ServerStream, _ *grpc.StreamServerInfo, handler grpc.StreamHandler) (err error) {
		defer func() {
			if rec := recover(); rec != nil {
				ctx := ss.Context()
				logger.FromContext(ctx).ErrorContext(ctx, "grpc stream handler panic",
					slog.Any("panic", rec), slog.String("stack", string(debug.Stack())))
				err = status.Error(codes.Internal, "internal server error")
			}
		}()
		return handler(srv, ss)
	}
}

// errorMapUnary chuyển error trả về của handler thành *status.Status chuẩn.
func errorMapUnary() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, _ *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		resp, err := handler(ctx, req)
		if err == nil {
			return resp, nil
		}
		return resp, toStatus(err).Err()
	}
}

func errorMapStream() grpc.StreamServerInterceptor {
	return func(srv any, ss grpc.ServerStream, _ *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		if err := handler(srv, ss); err != nil {
			return toStatus(err).Err()
		}
		return nil
	}
}

// loggingUnary log mỗi RPC sau khi xử lý: method, code, latency.
func loggingUnary() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		start := time.Now()
		resp, err := handler(ctx, req)
		logRPC(ctx, info.FullMethod, err, time.Since(start))
		return resp, err
	}
}

func loggingStream() grpc.StreamServerInterceptor {
	return func(srv any, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		start := time.Now()
		err := handler(srv, ss)
		logRPC(ss.Context(), info.FullMethod, err, time.Since(start))
		return err
	}
}

func logRPC(ctx context.Context, method string, err error, latency time.Duration) {
	code := codeFor(err)
	log := logger.FromContext(ctx)
	attrs := []slog.Attr{
		slog.String("grpc_method", method),
		slog.String("grpc_code", code.String()),
		slog.Duration("latency", latency),
	}
	switch {
	case code == codes.OK:
		log.LogAttrs(ctx, slog.LevelInfo, "grpc request", attrs...)
	case code == codes.Internal || code == codes.Unknown || code == codes.DataLoss:
		attrs = append(attrs, slog.String("error", errString(err)))
		log.LogAttrs(ctx, slog.LevelError, "grpc request", attrs...)
	default:
		attrs = append(attrs, slog.String("error", errString(err)))
		log.LogAttrs(ctx, slog.LevelWarn, "grpc request", attrs...)
	}
}

func errString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

// wrappedStream cho phép thay context của ServerStream.
type wrappedStream struct {
	grpc.ServerStream
	ctx context.Context
}

func (w *wrappedStream) Context() context.Context { return w.ctx }
