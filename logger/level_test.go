package logger_test

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/aupv9/common-go/logger"
)

func TestLevelControlSetGet(t *testing.T) {
	_, ctrl := logger.NewWithControl(logger.Config{Level: "info", Writer: io.Discard})
	if ctrl.Level() != slog.LevelInfo {
		t.Errorf("initial level = %v, want info", ctrl.Level())
	}
	ctrl.SetString("debug")
	if ctrl.Level() != slog.LevelDebug {
		t.Errorf("after set = %v, want debug", ctrl.Level())
	}
}

func TestLevelControlAffectsLogger(t *testing.T) {
	var buf strings.Builder
	log, ctrl := logger.NewWithControl(logger.Config{Level: "info", Format: "json", Writer: &buf})

	log.Debug("hidden") // dưới info → không log
	if buf.Len() != 0 {
		t.Errorf("debug logged at info level: %q", buf.String())
	}
	ctrl.SetString("debug")
	log.Debug("shown")
	if !strings.Contains(buf.String(), "shown") {
		t.Errorf("debug not logged after lowering level: %q", buf.String())
	}
}

func TestLevelHandler(t *testing.T) {
	_, ctrl := logger.NewWithControl(logger.Config{Level: "warn", Writer: io.Discard})
	h := ctrl.Handler()

	// GET → trả level hiện tại.
	rec := httptest.NewRecorder()
	h(rec, httptest.NewRequest(http.MethodGet, "/admin/loglevel", nil))
	var got map[string]string
	_ = json.Unmarshal(rec.Body.Bytes(), &got)
	if got["level"] != "warn" {
		t.Errorf("GET level = %q, want warn", got["level"])
	}

	// PUT ?level=error → đổi.
	rec = httptest.NewRecorder()
	h(rec, httptest.NewRequest(http.MethodPut, "/admin/loglevel?level=error", nil))
	if ctrl.Level() != slog.LevelError {
		t.Errorf("after PUT level = %v, want error", ctrl.Level())
	}

	// PUT thiếu level → 400.
	rec = httptest.NewRecorder()
	h(rec, httptest.NewRequest(http.MethodPut, "/admin/loglevel", strings.NewReader("{}")))
	if rec.Code != http.StatusBadRequest {
		t.Errorf("missing level status = %d, want 400", rec.Code)
	}
}
