package grpcserver

import (
	"testing"

	"github.com/aupv9/common-go/config"
)

func TestOptionsFromConfig(t *testing.T) {
	cfg := config.DefaultAppConfig()
	cfg.GRPC.Host = "1.2.3.4"
	cfg.GRPC.Port = 7777
	cfg.GRPC.Reflection = false
	cfg.GRPC.MaxRecvMsgSize = 1024
	cfg.GRPC.MaxSendMsgSize = 2048
	cfg.Otel.Enabled = true
	cfg.GRPC.TLS.Enabled = true
	cfg.GRPC.TLS.CertFile = "c"
	cfg.GRPC.TLS.KeyFile = "k"

	o := defaultOptions()
	for _, opt := range optionsFromConfig(cfg) {
		opt(&o)
	}

	if o.host != "1.2.3.4" || o.port != 7777 {
		t.Errorf("addr = %s:%d", o.host, o.port)
	}
	if o.reflection {
		t.Error("reflection should be off")
	}
	if !o.tracing {
		t.Error("tracing should be on (otel enabled)")
	}
	if o.maxRecvMsgSize != 1024 || o.maxSendMsgSize != 2048 {
		t.Errorf("msg sizes = %d/%d", o.maxRecvMsgSize, o.maxSendMsgSize)
	}
	if !o.tls.Enabled || o.tls.CertFile != "c" {
		t.Errorf("tls = %+v", o.tls)
	}
}
