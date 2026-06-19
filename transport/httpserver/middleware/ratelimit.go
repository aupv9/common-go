package middleware

import (
	"context"
	"net/http"
	"sync"
	"time"

	apperrors "github.com/aupv9/common-go/errors"
	"github.com/aupv9/common-go/response"
	"github.com/gin-gonic/gin"
	"golang.org/x/time/rate"
)

// Limiter là backend giới hạn tần suất theo key. Cho phép cắm bản phân tán
// (vd Redis) thay cho bản in-memory mặc định. Định nghĩa structural ở đây để
// driver (cache.RedisLimiter) không phải import middleware (tránh import cycle).
type Limiter interface {
	// Allow trả true nếu request cho key được phép; error nếu backend lỗi.
	Allow(ctx context.Context, key string) (bool, error)
}

// RateLimitConfig cấu hình giới hạn tần suất theo client (mặc định theo IP).
type RateLimitConfig struct {
	// RPS là số request/giây cho phép mỗi key.
	RPS float64
	// Burst là số request bùng nổ tối đa.
	Burst int
	// KeyFunc trích key phân loại (mặc định client IP).
	KeyFunc func(*gin.Context) string
	// TTL: thời gian không hoạt động trước khi quên một key (giải phóng bộ nhớ).
	// Chỉ áp dụng cho backend in-memory.
	TTL time.Duration
	// Context (tuỳ chọn) gắn vòng đời janitor in-memory: huỷ → goroutine dừng.
	Context context.Context
	// Limiter (tuỳ chọn): backend phân tán. Set → dùng thay in-memory.
	Limiter Limiter
	// FailClosed: khi Limiter lỗi (vd Redis down), true = chặn (429), false
	// (mặc định) = cho qua (fail-open, ưu tiên availability).
	FailClosed bool
}

// RateLimit trả middleware giới hạn tần suất per-key. Vượt giới hạn → 429 qua
// response envelope. Mặc định dùng token-bucket in-memory (janitor dọn key cũ);
// nếu cfg.Limiter set thì dùng backend đó (vd Redis cho giới hạn xuyên replica).
func RateLimit(cfg RateLimitConfig) gin.HandlerFunc {
	if cfg.KeyFunc == nil {
		cfg.KeyFunc = func(c *gin.Context) string { return c.ClientIP() }
	}

	if cfg.Limiter != nil {
		return rateLimitWith(cfg, cfg.Limiter.Allow)
	}

	if cfg.TTL <= 0 {
		cfg.TTL = 10 * time.Minute
	}
	ctx := cfg.Context
	if ctx == nil {
		ctx = context.Background()
	}
	store := newLimiterStore(ctx, rate.Limit(cfg.RPS), cfg.Burst, cfg.TTL)
	return rateLimitWith(cfg, func(_ context.Context, key string) (bool, error) {
		return store.get(key).Allow(), nil
	})
}

// rateLimitWith dựng middleware từ một hàm allow; xử lý fail-open/closed + 429.
func rateLimitWith(cfg RateLimitConfig, allow func(context.Context, string) (bool, error)) gin.HandlerFunc {
	return func(c *gin.Context) {
		key := cfg.KeyFunc(c)
		ok, err := allow(c.Request.Context(), key)
		if err != nil {
			if cfg.FailClosed {
				rejectRateLimited(c)
				return
			}
			c.Next() // fail-open
			return
		}
		if !ok {
			rejectRateLimited(c)
			return
		}
		c.Next()
	}
}

func rejectRateLimited(c *gin.Context) {
	c.Abort()
	c.Header("Retry-After", "1")
	response.Error(c.Writer, c.Request,
		apperrors.New(apperrors.KindUnavailable, "RATE_LIMITED", "too many requests").
			WithHTTPStatus(http.StatusTooManyRequests))
}

// limiterStore giữ limiter theo key với dọn dẹp theo TTL.
type limiterStore struct {
	mu       sync.Mutex
	limiters map[string]*entry
	r        rate.Limit
	b        int
	ttl      time.Duration
}

type entry struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

func newLimiterStore(ctx context.Context, r rate.Limit, b int, ttl time.Duration) *limiterStore {
	s := &limiterStore{
		limiters: make(map[string]*entry),
		r:        r,
		b:        b,
		ttl:      ttl,
	}
	go s.janitor(ctx)
	return s
}

func (s *limiterStore) get(key string) *rate.Limiter {
	s.mu.Lock()
	defer s.mu.Unlock()
	e, ok := s.limiters[key]
	if !ok {
		e = &entry{limiter: rate.NewLimiter(s.r, s.b)}
		s.limiters[key] = e
	}
	e.lastSeen = time.Now()
	return e.limiter
}

// janitor định kỳ xoá các key không hoạt động lâu hơn TTL; dừng khi ctx huỷ.
func (s *limiterStore) janitor(ctx context.Context) {
	ticker := time.NewTicker(s.ttl)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			cutoff := time.Now().Add(-s.ttl)
			s.mu.Lock()
			for k, e := range s.limiters {
				if e.lastSeen.Before(cutoff) {
					delete(s.limiters, k)
				}
			}
			s.mu.Unlock()
		}
	}
}
