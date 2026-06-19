package httpclient_test

import (
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/aupv9/common-go/client/httpclient"
)

func TestRetryThenSuccess(t *testing.T) {
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		n := atomic.AddInt32(&calls, 1)
		if n < 3 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	cli := httpclient.New(
		httpclient.WithRetry(3, time.Millisecond, 5*time.Millisecond),
	)
	resp, err := cli.Get(srv.URL)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	if got := atomic.LoadInt32(&calls); got != 3 {
		t.Errorf("calls = %d, want 3 (2 retries)", got)
	}
}

func TestRetryExhausted(t *testing.T) {
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		atomic.AddInt32(&calls, 1)
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer srv.Close()

	cli := httpclient.New(httpclient.WithRetry(2, time.Millisecond, 5*time.Millisecond))
	resp, err := cli.Get(srv.URL)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503", resp.StatusCode)
	}
	// 1 lần đầu + 2 retry = 3.
	if got := atomic.LoadInt32(&calls); got != 3 {
		t.Errorf("calls = %d, want 3", got)
	}
}

func TestNoRetryOnPOSTByDefault(t *testing.T) {
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		atomic.AddInt32(&calls, 1)
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer srv.Close()

	cli := httpclient.New(httpclient.WithRetry(3, time.Millisecond, 5*time.Millisecond))
	resp, err := cli.Post(srv.URL, "text/plain", nil)
	if err != nil {
		t.Fatalf("Post: %v", err)
	}
	defer resp.Body.Close()

	if got := atomic.LoadInt32(&calls); got != 1 {
		t.Errorf("calls = %d, want 1 (POST not retried by default)", got)
	}
}

func TestNoRetryWhenDisabled(t *testing.T) {
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		atomic.AddInt32(&calls, 1)
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer srv.Close()

	cli := httpclient.New(httpclient.WithNoRetry())
	resp, err := cli.Get(srv.URL)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	defer resp.Body.Close()

	if got := atomic.LoadInt32(&calls); got != 1 {
		t.Errorf("calls = %d, want 1 (retry disabled)", got)
	}
}

func TestDefaultRetryPolicy(t *testing.T) {
	get := httptest.NewRequest(http.MethodGet, "http://x/", nil)
	post := httptest.NewRequest(http.MethodPost, "http://x/", nil)

	cases := []struct {
		name string
		req  *http.Request
		resp *http.Response
		err  error
		want bool
	}{
		{"GET transport error", get, nil, http.ErrHandlerTimeout, true},
		{"GET 503", get, &http.Response{StatusCode: 503}, nil, true},
		{"GET 200", get, &http.Response{StatusCode: 200}, nil, false},
		{"GET 400", get, &http.Response{StatusCode: 400}, nil, false},
		{"POST 503", post, &http.Response{StatusCode: 503}, nil, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := httpclient.DefaultRetryPolicy(tc.req, tc.resp, tc.err); got != tc.want {
				t.Errorf("got %v, want %v", got, tc.want)
			}
		})
	}
}
