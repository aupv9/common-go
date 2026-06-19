package httpclient_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/aupv9/common-go/client/httpclient"
)

func TestPerAttemptTimeout(t *testing.T) {
	// Server chậm hơn per-attempt timeout → request lỗi deadline.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-time.After(500 * time.Millisecond):
			w.WriteHeader(http.StatusOK)
		case <-r.Context().Done():
		}
	}))
	defer srv.Close()

	cli := httpclient.New(
		httpclient.WithNoRetry(),
		httpclient.WithoutOTel(),
		httpclient.WithPerAttemptTimeout(50*time.Millisecond),
		httpclient.WithTimeout(0), // tắt timeout tổng để chỉ kiểm per-attempt
	)

	_, err := cli.Get(srv.URL)
	if err == nil {
		t.Fatal("expected per-attempt timeout error")
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("err = %v, want context.DeadlineExceeded", err)
	}
}

func TestPerAttemptTimeoutSuccessReadsBody(t *testing.T) {
	// Server trả nhanh → body đọc được trọn vẹn dù có per-attempt timeout
	// (timeout chỉ huỷ khi đóng body).
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("hello-body"))
	}))
	defer srv.Close()

	cli := httpclient.New(
		httpclient.WithNoRetry(),
		httpclient.WithoutOTel(),
		httpclient.WithPerAttemptTimeout(2*time.Second),
	)

	resp, err := cli.Get(srv.URL)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	defer resp.Body.Close()
	buf := make([]byte, 32)
	n, _ := resp.Body.Read(buf)
	if string(buf[:n]) != "hello-body" {
		t.Errorf("body = %q, want hello-body", string(buf[:n]))
	}
}
