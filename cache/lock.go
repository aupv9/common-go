package cache

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"time"

	"github.com/redis/go-redis/v9"
)

// ErrLockNotAcquired trả về khi không giành được lock (đã có process khác giữ).
var ErrLockNotAcquired = errors.New("cache: lock not acquired")

// ErrLockLost trả về khi refresh nhưng lock không còn do mình giữ (hết hạn hoặc
// bị process khác chiếm).
var ErrLockLost = errors.New("cache: lock lost")

// Lock là một distributed lock đã giành được; giữ token để chỉ chủ sở hữu mới
// release/refresh được (tránh tác động nhầm lock của process khác). Token cũng
// dùng làm fencing token: caller có thể truyền xuống downstream để chống thao
// tác cũ khi lock đã bị mất.
type Lock struct {
	rdb   redis.UniversalClient
	key   string
	token string
	ttl   time.Duration
}

// Token trả fencing token của lock (định danh duy nhất lần giữ này).
func (l *Lock) Token() string { return l.token }

// Key trả khoá lock.
func (l *Lock) Key() string { return l.key }

// releaseScript đảm bảo chỉ xoá key nếu value khớp token (atomic check-and-del).
var releaseScript = redis.NewScript(`
if redis.call("get", KEYS[1]) == ARGV[1] then
	return redis.call("del", KEYS[1])
else
	return 0
end`)

// refreshScript gia hạn TTL chỉ khi vẫn do mình giữ (atomic check-and-pexpire).
var refreshScript = redis.NewScript(`
if redis.call("get", KEYS[1]) == ARGV[1] then
	return redis.call("pexpire", KEYS[1], ARGV[2])
else
	return 0
end`)

// AcquireLock thử giành lock key trong ttl bằng SET NX PX. Trả ErrLockNotAcquired
// nếu key đã bị giữ.
func AcquireLock(ctx context.Context, rdb redis.UniversalClient, key string, ttl time.Duration) (*Lock, error) {
	token, err := newToken()
	if err != nil {
		return nil, err
	}
	ok, err := rdb.SetNX(ctx, key, token, ttl).Result()
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, ErrLockNotAcquired
	}
	return &Lock{rdb: rdb, key: key, token: token, ttl: ttl}, nil
}

// Release giải phóng lock (chỉ khi vẫn do mình giữ).
func (l *Lock) Release(ctx context.Context) error {
	return releaseScript.Run(ctx, l.rdb, []string{l.key}, l.token).Err()
}

// Refresh gia hạn TTL của lock về l.ttl (chỉ khi vẫn do mình giữ). Trả
// ErrLockLost nếu lock đã mất (token không khớp / đã hết hạn).
func (l *Lock) Refresh(ctx context.Context) error {
	res, err := refreshScript.Run(ctx, l.rdb, []string{l.key}, l.token, l.ttl.Milliseconds()).Int64()
	if err != nil {
		return err
	}
	if res == 0 {
		return ErrLockLost
	}
	return nil
}

// Watchdog tự gia hạn lock định kỳ (mỗi ttl/3) tới khi ctx huỷ hoặc lock mất.
// Trả về hàm stop để dừng watchdog (gọi khi xong việc, thường defer). Dùng khi
// thời gian giữ lock không đoán trước được.
//
//	lock, _ := cache.AcquireLock(ctx, rdb, key, 30*time.Second)
//	stop := lock.Watchdog(ctx, nil)
//	defer stop()
//	defer lock.Release(context.Background())
func (l *Lock) Watchdog(ctx context.Context, onLost func(error)) (stop func()) {
	interval := l.ttl / 3
	if interval <= 0 {
		interval = time.Second
	}
	wctx, cancel := context.WithCancel(ctx)
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-wctx.Done():
				return
			case <-ticker.C:
				if err := l.Refresh(wctx); err != nil {
					if onLost != nil {
						onLost(err)
					}
					return
				}
			}
		}
	}()
	return cancel
}

func newToken() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(b[:]), nil
}
