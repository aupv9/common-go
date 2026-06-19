// Package health cung cấp liveness/readiness/build-info qua HTTP.
//
//   - /healthz  : liveness — process còn sống (luôn 200 nếu phục vụ được).
//   - /readyz   : readiness — chạy mọi Checker đã đăng ký (db, eventbus...).
//   - /info     : build info (version, commit, build time).
//
// Các module khác (database, eventbus...) tự Register checker của mình vào
// Registry để góp mặt trong readiness.
package health

import (
	"context"
	"encoding/json"
	"net/http"
	"sync"
	"time"
)

// Checker kiểm tra sức khoẻ của một dependency.
type Checker interface {
	// Name định danh checker (xuất hiện trong response).
	Name() string
	// Check trả nil nếu khoẻ; lỗi nếu không.
	Check(ctx context.Context) error
}

// CheckerFunc adapter cho hàm thuần.
type CheckerFunc struct {
	CheckerName string
	CheckFn     func(ctx context.Context) error
}

func (c CheckerFunc) Name() string                    { return c.CheckerName }
func (c CheckerFunc) Check(ctx context.Context) error { return c.CheckFn(ctx) }

// Registry giữ tập checker cho readiness và metadata cho /info.
type Registry struct {
	mu       sync.RWMutex
	checkers []Checker

	service string
	env     string
	timeout time.Duration
}

// NewRegistry tạo registry. service/env hiển thị trong /info; timeout là hạn
// chạy toàn bộ readiness check.
func NewRegistry(service, env string) *Registry {
	return &Registry{
		service: service,
		env:     env,
		timeout: 5 * time.Second,
	}
}

// Register thêm một hoặc nhiều checker (thread-safe).
func (r *Registry) Register(checkers ...Checker) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.checkers = append(r.checkers, checkers...)
}

// checkResult cho từng dependency.
type checkResult struct {
	Status string `json:"status"` // "up" | "down"
	Error  string `json:"error,omitempty"`
}

type readinessResponse struct {
	Status string                 `json:"status"` // "up" | "down"
	Checks map[string]checkResult `json:"checks,omitempty"`
}

// LivenessHandler luôn trả 200 khi process phục vụ được.
func (r *Registry) LivenessHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "up"})
	}
}

// ReadinessHandler chạy mọi checker SONG SONG (mỗi checker một goroutine) trong
// hạn timeout dùng chung; 200 nếu tất cả up, 503 nếu có cái down. Chạy song song
// để tổng latency = checker chậm nhất, không phải tổng các checker.
func (r *Registry) ReadinessHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		ctx, cancel := context.WithTimeout(req.Context(), r.timeout)
		defer cancel()

		r.mu.RLock()
		checkers := make([]Checker, len(r.checkers))
		copy(checkers, r.checkers)
		r.mu.RUnlock()

		results := make([]checkResult, len(checkers))
		var wg sync.WaitGroup
		wg.Add(len(checkers))
		for i, c := range checkers {
			go func(i int, c Checker) {
				defer wg.Done()
				if err := c.Check(ctx); err != nil {
					results[i] = checkResult{Status: "down", Error: err.Error()}
					return
				}
				results[i] = checkResult{Status: "up"}
			}(i, c)
		}
		wg.Wait()

		resp := readinessResponse{Status: "up", Checks: map[string]checkResult{}}
		code := http.StatusOK
		for i, c := range checkers {
			resp.Checks[c.Name()] = results[i]
			if results[i].Status == "down" {
				resp.Status = "down"
				code = http.StatusServiceUnavailable
			}
		}
		writeJSON(w, code, resp)
	}
}

// InfoHandler trả build info.
func (r *Registry) InfoHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, BuildInfo{
			Service:   r.service,
			Version:   Version,
			Commit:    Commit,
			BuildTime: BuildTime,
			Env:       r.env,
		})
	}
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}
