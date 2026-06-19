package database

import (
	"fmt"
	"net/url"
	"time"
)

// Config cấu hình kết nối Postgres (pgxpool). Nếu DSN set thì dùng nguyên DSN,
// bỏ qua các trường rời.
type Config struct {
	DSN      string
	Host     string
	Port     int
	User     string
	Password string
	Name     string
	SSLMode  string

	MaxConns        int32
	MinConns        int32
	MaxConnLifetime time.Duration
	MaxConnIdleTime time.Duration
	ConnectTimeout  time.Duration

	// AppName gắn vào application_name của connection (tiện trace ở pg_stat_activity).
	AppName string
	// Tracing bật otelpgx (span cho mỗi query).
	Tracing bool
}

// DefaultConfig trả cấu hình mặc định hợp lý cho pool.
func DefaultConfig() Config {
	return Config{
		Host:            "localhost",
		Port:            5432,
		SSLMode:         "disable",
		MaxConns:        10,
		MinConns:        2,
		MaxConnLifetime: time.Hour,
		MaxConnIdleTime: 30 * time.Minute,
		ConnectTimeout:  5 * time.Second,
	}
}

// DSNString trả DSN dạng URL Postgres. Ưu tiên Config.DSN nếu đã set.
func (c Config) DSNString() string {
	if c.DSN != "" {
		return c.DSN
	}
	q := url.Values{}
	if c.SSLMode != "" {
		q.Set("sslmode", c.SSLMode)
	}
	if c.ConnectTimeout > 0 {
		q.Set("connect_timeout", fmt.Sprintf("%d", int(c.ConnectTimeout.Seconds())))
	}
	if c.AppName != "" {
		q.Set("application_name", c.AppName)
	}
	u := url.URL{
		Scheme:   "postgres",
		User:     url.UserPassword(c.User, c.Password),
		Host:     fmt.Sprintf("%s:%d", c.Host, c.Port),
		Path:     "/" + c.Name,
		RawQuery: q.Encode(),
	}
	return u.String()
}
