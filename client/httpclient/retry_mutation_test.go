package httpclient

import (
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

// stubRT trả 503 cho (failFirst) lần đầu rồi 200.
type stubRT struct {
	calls     int
	failFirst int
	gotBodies []string
}

func (s *stubRT) RoundTrip(req *http.Request) (*http.Response, error) {
	s.calls++
	if req.Body != nil {
		b, _ := io.ReadAll(req.Body)
		s.gotBodies = append(s.gotBodies, string(b))
	}
	code := http.StatusOK
	if s.calls <= s.failFirst {
		code = http.StatusServiceUnavailable
	}
	return &http.Response{StatusCode: code, Body: io.NopCloser(strings.NewReader("")), Header: make(http.Header)}, nil
}

func TestRetryDoesNotMutateCallerRequest(t *testing.T) {
	base := &stubRT{failFirst: 1}
	rt := &retryTransport{base: base, maxRetries: 2, baseBackoff: time.Millisecond, maxBackoff: time.Millisecond, policy: DefaultRetryPolicy}

	req, _ := http.NewRequest(http.MethodPut, "http://x/", strings.NewReader("payload"))
	// http.NewRequest tự set GetBody cho strings.Reader; ép về nil để kiểm nhánh
	// tự-buffer của retryTransport (và để khẳng định caller request không bị sửa).
	req.GetBody = nil

	resp, err := rt.RoundTrip(req)
	if err != nil {
		t.Fatalf("RoundTrip: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}

	// Caller request KHÔNG bị gán GetBody (ta chỉ sửa bản clone).
	if req.GetBody != nil {
		t.Error("caller request was mutated: GetBody set")
	}
	// Mỗi lần thử nhận đúng body "payload" (replay hoạt động).
	if len(base.gotBodies) != 2 {
		t.Fatalf("attempts with body = %d, want 2", len(base.gotBodies))
	}
	for i, b := range base.gotBodies {
		if b != "payload" {
			t.Errorf("attempt %d body = %q, want payload", i, b)
		}
	}
}

func TestWithJitterBounds(t *testing.T) {
	for i := 0; i < 1000; i++ {
		d := withJitter(100 * time.Millisecond)
		if d < 50*time.Millisecond || d > 100*time.Millisecond {
			t.Fatalf("jitter out of [50ms,100ms]: %v", d)
		}
	}
	if withJitter(0) != 0 {
		t.Error("withJitter(0) should be 0")
	}
}
