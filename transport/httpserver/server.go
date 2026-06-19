// Package httpserver cung cấp HTTP server dựng trên gin với bộ middleware chuẩn
// (request id, recovery, access log, timeout, CORS) và graceful shutdown.
//
// Dùng qua fx:
//
//	fx.New(
//	    foundation.Core(),
//	    foundation.WithHTTP(),         // cung cấp *gin.Engine + chạy server
//	    fx.Invoke(routes.Register),    // service gắn route lên *gin.Engine
//	).Run()
//
// Hoặc standalone:
//
//	srv, _ := httpserver.New(log, httpserver.WithPort(9090))
//	srv.Engine().GET("/ping", handler)
//	_ = srv.Start()
//	defer srv.Shutdown(context.Background())
package httpserver

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/http"

	"github.com/aupv9/common-go/transport/httpserver/middleware"
	"github.com/gin-contrib/gzip"
	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/contrib/instrumentation/github.com/gin-gonic/gin/otelgin"
)

// Server bọc gin engine và http.Server, quản lý vòng đời.
type Server struct {
	opts   options
	log    *slog.Logger
	engine *gin.Engine
	http   *http.Server
}

// New tạo Server với bộ middleware chuẩn đã gắn sẵn theo thứ tự:
//
//	ContextLogger → RequestID → Recovery → AccessLog → Timeout → CORS → extra
//
// (Tracing OTel được chèn ở M3.) log là logger nền inject vào mỗi request.
func New(log *slog.Logger, opts ...Option) (*Server, error) {
	if log == nil {
		return nil, fmt.Errorf("httpserver: logger is required")
	}
	o := defaultOptions()
	for _, opt := range opts {
		opt(&o)
	}

	gin.SetMode(o.mode)
	engine := gin.New()

	// Bộ middleware chuẩn — thứ tự quan trọng.
	engine.Use(
		middleware.ContextLogger(log),
		middleware.RequestID(),
	)
	// OTel tracing đặt ngay sau RequestID để span bao trùm recovery + handler,
	// và request_id có mặt trong các log phát ra bên trong span.
	if o.tracingEnabled {
		engine.Use(otelgin.Middleware(o.serviceName))
	}
	engine.Use(
		middleware.Recovery(),
		middleware.AccessLog(o.logSkipPaths...),
	)
	if o.securityHeaders {
		engine.Use(middleware.SecurityHeaders(o.hsts))
	}
	if o.gzipEnabled {
		engine.Use(gzip.Gzip(o.gzipLevel))
	}
	if o.bodyLimit > 0 {
		engine.Use(middleware.BodyLimit(o.bodyLimit))
	}
	if o.rateLimitOn {
		engine.Use(middleware.RateLimit(o.rateLimit))
	}
	if o.requestTimeout > 0 {
		engine.Use(middleware.Timeout(o.requestTimeout))
	}
	if o.corsEnabled {
		engine.Use(middleware.CORS(o.cors))
	}
	if len(o.extra) > 0 {
		engine.Use(o.extra...)
	}

	tlsCfg, err := o.tls.Build()
	if err != nil {
		return nil, err
	}

	addr := net.JoinHostPort(o.host, fmt.Sprintf("%d", o.port))
	srv := &Server{
		opts:   o,
		log:    log,
		engine: engine,
		http: &http.Server{
			Addr:         addr,
			Handler:      engine,
			ReadTimeout:  o.readTimeout,
			WriteTimeout: o.writeTimeout,
			IdleTimeout:  o.idleTimeout,
			TLSConfig:    tlsCfg,
		},
	}
	return srv, nil
}

// Engine trả gin engine để service gắn route.
func (s *Server) Engine() *gin.Engine { return s.engine }

// Addr trả địa chỉ server lắng nghe.
func (s *Server) Addr() string { return s.http.Addr }

// Listen mở listener TCP (bind cổng) một cách ĐỒNG BỘ. Tách khỏi Serve để lỗi
// bind (vd cổng đang dùng) được phát hiện ngay trong OnStart và làm fail startup,
// thay vì bị nuốt trong goroutine.
func (s *Server) Listen() (net.Listener, error) {
	lis, err := net.Listen("tcp", s.http.Addr)
	if err != nil {
		return nil, fmt.Errorf("httpserver: listen %s: %w", s.http.Addr, err)
	}
	return lis, nil
}

// Serve phục vụ trên listener đã mở (blocking). Trả nil khi server bị Shutdown.
// Dùng TLS khi đã cấu hình (chứng chỉ nằm trong http.Server.TLSConfig).
func (s *Server) Serve(lis net.Listener) error {
	tls := s.http.TLSConfig != nil
	s.log.Info("http server starting", slog.String("addr", s.http.Addr), slog.Bool("tls", tls))

	var err error
	if tls {
		// Chứng chỉ đã nạp trong TLSConfig → tham số file để trống.
		err = s.http.ServeTLS(lis, "", "")
	} else {
		err = s.http.Serve(lis)
	}
	if err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("httpserver: serve: %w", err)
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

// Shutdown dừng server, chờ request đang chạy hoàn tất trong hạn shutdownTimeout.
func (s *Server) Shutdown(ctx context.Context) error {
	s.log.Info("http server stopping")
	shutdownCtx, cancel := context.WithTimeout(ctx, s.opts.shutdownTimeout)
	defer cancel()
	if err := s.http.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("httpserver: shutdown: %w", err)
	}
	return nil
}
