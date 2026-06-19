package httpserver_test

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"testing"
	"time"

	"github.com/aupv9/common-go/config"
	"github.com/aupv9/common-go/health"
	"github.com/aupv9/common-go/logger"
	"github.com/aupv9/common-go/transport/httpserver"
	"go.uber.org/fx"
)

// TestModuleBoots dựng httpserver.Module qua fx với cổng động, kiểm health +
// admin endpoint hoạt động và shutdown sạch — bao phủ run/mountRoutes/
// setupResponseMeta/optionsFromConfig.
func TestModuleBoots(t *testing.T) {
	port := freePort(t)
	cfg := config.DefaultAppConfig()
	cfg.App.Name = "test"
	cfg.App.Env = "local"
	cfg.Server.Host = "127.0.0.1"
	cfg.Server.Port = port

	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	_, lvl := logger.NewWithControl(logger.Config{Level: "info", Writer: io.Discard})

	app := fx.New(
		fx.Supply(cfg, log, lvl),
		fx.Provide(func() *health.Registry { return health.NewRegistry("test", "local") }),
		httpserver.Module(),
		fx.NopLogger,
	)

	startCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := app.Start(startCtx); err != nil {
		t.Fatalf("fx start: %v", err)
	}
	t.Cleanup(func() {
		stopCtx, c := context.WithTimeout(context.Background(), 5*time.Second)
		defer c()
		_ = app.Stop(stopCtx)
	})

	base := "http://127.0.0.1:" + itoa(port)
	waitHTTP(t, "127.0.0.1:"+itoa(port))

	for _, path := range []string{"/healthz", "/readyz", "/info", "/admin/loglevel"} {
		resp, err := http.Get(base + path)
		if err != nil {
			t.Fatalf("GET %s: %v", path, err)
		}
		resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Errorf("GET %s status = %d, want 200", path, resp.StatusCode)
		}
	}
}

func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	var b [20]byte
	pos := len(b)
	for i > 0 {
		pos--
		b[pos] = byte('0' + i%10)
		i /= 10
	}
	return string(b[pos:])
}
