package config

import "log/slog"

// Các section chứa secret implement slog.LogValuer để khi log (slog.Any) thì
// trường nhạy cảm (password/secret/DSN) tự bị che, tránh rò ra log.

// LogValue che Password và DSN (DSN nhúng credential).
func (s DatabaseSection) LogValue() slog.Value {
	dsn := ""
	if s.DSN != "" {
		dsn = redacted
	}
	return slog.GroupValue(
		slog.String("host", s.Host),
		slog.Int("port", s.Port),
		slog.String("user", s.User),
		slog.String("name", s.Name),
		slog.String("ssl_mode", s.SSLMode),
		slog.String("password", maskIf(s.Password)),
		slog.String("dsn", dsn),
	)
}

// LogValue che Secret (HMAC) — PublicKeyPEM không phải secret nhưng dài, rút gọn.
func (s AuthSection) LogValue() slog.Value {
	return slog.GroupValue(
		slog.String("algorithm", s.Algorithm),
		slog.String("issuer", s.Issuer),
		slog.String("audience", s.Audience),
		slog.String("secret", maskIf(s.Secret)),
		slog.Bool("public_key_set", s.PublicKeyPEM != ""),
	)
}

// LogValue che Password.
func (s CacheSection) LogValue() slog.Value {
	return slog.GroupValue(
		slog.Any("addrs", s.Addrs),
		slog.Int("db", s.DB),
		slog.Int("pool_size", s.PoolSize),
		slog.String("password", maskIf(s.Password)),
	)
}

// maskIf trả "***" nếu có giá trị, rỗng nếu không (để phân biệt set/unset).
func maskIf(v string) string {
	if v == "" {
		return ""
	}
	return redacted
}
