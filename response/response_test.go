package response_test

import (
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/aupv9/common-go/response"
)

// TestMetaFuncConcurrent đảm bảo SetMetaFunc đồng thời với ghi response không
// gây data race (chạy với -race).
func TestMetaFuncConcurrent(t *testing.T) {
	var wg sync.WaitGroup
	stop := make(chan struct{})

	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			select {
			case <-stop:
				return
			default:
				rec := httptest.NewRecorder()
				req := httptest.NewRequest(http.MethodGet, "/", nil)
				response.OK(rec, req, map[string]string{"k": "v"})
			}
		}
	}()

	for i := 0; i < 100; i++ {
		response.SetMetaFunc(func(*http.Request) *response.Meta {
			return &response.Meta{RequestID: "r"}
		})
	}

	close(stop)
	wg.Wait()
}

func TestMetaFuncNilSafe(t *testing.T) {
	response.SetMetaFunc(nil)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	// Không panic khi metaFrom đặt nil.
	response.OK(rec, req, "ok")
	if rec.Code != http.StatusOK {
		t.Errorf("status = %d", rec.Code)
	}
}
