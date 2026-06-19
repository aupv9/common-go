package cache

import (
	"context"

	"github.com/redis/go-redis/v9"
)

// RedisLimiter là rate limiter token-bucket phân tán trên Redis: nhiều replica
// dùng chung quota theo key. Thoả mãn interface middleware.Limiter (structural)
// nên truyền thẳng vào httpserver middleware.RateLimit{Limiter: ...}.
type RedisLimiter struct {
	rdb    redis.UniversalClient
	rate   float64 // token/giây
	burst  int     // dung lượng bucket
	prefix string
}

// NewRedisLimiter tạo limiter với rate (token/giây) và burst (capacity). prefix
// gắn trước key trong Redis (mặc định "ratelimit:").
func NewRedisLimiter(rdb redis.UniversalClient, rate float64, burst int, prefix string) *RedisLimiter {
	if prefix == "" {
		prefix = "ratelimit:"
	}
	return &RedisLimiter{rdb: rdb, rate: rate, burst: burst, prefix: prefix}
}

// tokenBucketScript: token bucket dùng thời gian của Redis (TIME) để nhất quán
// xuyên replica. KEYS[1]=bucket. ARGV: rate, capacity, requested. Trả 1 nếu cho
// phép, 0 nếu vượt.
var tokenBucketScript = redis.NewScript(`
local rate = tonumber(ARGV[1])
local cap = tonumber(ARGV[2])
local requested = tonumber(ARGV[3])
local t = redis.call("TIME")
local now = tonumber(t[1]) + tonumber(t[2]) / 1000000

local data = redis.call("HMGET", KEYS[1], "tokens", "ts")
local tokens = tonumber(data[1])
local ts = tonumber(data[2])
if tokens == nil then
	tokens = cap
	ts = now
end

local delta = now - ts
if delta < 0 then delta = 0 end
tokens = math.min(cap, tokens + delta * rate)

local allowed = 0
if tokens >= requested then
	tokens = tokens - requested
	allowed = 1
end

redis.call("HMSET", KEYS[1], "tokens", tokens, "ts", now)
local ttl = math.ceil(cap / rate) + 1
redis.call("EXPIRE", KEYS[1], ttl)
return allowed
`)

// Allow tiêu thụ 1 token cho key; trả true nếu còn quota.
func (l *RedisLimiter) Allow(ctx context.Context, key string) (bool, error) {
	res, err := tokenBucketScript.Run(ctx, l.rdb, []string{l.prefix + key}, l.rate, l.burst, 1).Int64()
	if err != nil {
		return false, err
	}
	return res == 1, nil
}
