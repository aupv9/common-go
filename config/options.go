package config

import "time"

// options gom cấu hình cho Load.
type options struct {
	filePath     string
	fileOptional bool
	envPrefix    string
	delim        string
	structTag    string
	defaults     any
}

func defaultOptions() options {
	return options{
		filePath:     "",
		fileOptional: true,
		envPrefix:    "APP_",
		delim:        ".",
		structTag:    "koanf",
		defaults:     DefaultAppConfig(),
	}
}

// Option tinh chỉnh hành vi của Load.
type Option func(*options)

// WithFile chỉ định đường dẫn file config (bắt buộc tồn tại).
func WithFile(path string) Option {
	return func(o *options) { o.filePath = path; o.fileOptional = false }
}

// WithOptionalFile như WithFile nhưng không lỗi nếu file không tồn tại.
func WithOptionalFile(path string) Option {
	return func(o *options) { o.filePath = path; o.fileOptional = true }
}

// WithEnvPrefix đổi prefix env (mặc định "APP_").
func WithEnvPrefix(prefix string) Option {
	return func(o *options) { o.envPrefix = prefix }
}

// WithDefaults override struct defaults (mặc định DefaultAppConfig()).
// Truyền con trỏ tới struct đã điền giá trị mặc định.
func WithDefaults(v any) Option {
	return func(o *options) { o.defaults = v }
}

// WithStructTag đổi tag dùng để map (mặc định "koanf").
func WithStructTag(tag string) Option {
	return func(o *options) { o.structTag = tag }
}

// AppConfig là cấu hình nền mà mọi service kế thừa (embed với `koanf:",squash"`).
type AppConfig struct {
	App      AppSection      `koanf:"app"`
	Log      LogSection      `koanf:"log"`
	Server   ServerSection   `koanf:"server"`
	GRPC     GRPCSection     `koanf:"grpc"`
	Otel     OtelSection     `koanf:"otel"`
	Database DatabaseSection `koanf:"database"`
	EventBus EventBusSection `koanf:"eventbus"`
	Auth     AuthSection     `koanf:"auth"`
	Cache    CacheSection    `koanf:"cache"`
}

// AppSection: định danh service.
type AppSection struct {
	Name string `koanf:"name" validate:"required"`
	Env  string `koanf:"env" validate:"required,oneof=local dev staging prod"`
}

// LogSection: cấu hình logger.
type LogSection struct {
	Level  string `koanf:"level" validate:"required,oneof=debug info warn error"`
	Format string `koanf:"format" validate:"required,oneof=json text"`
}

// ServerSection: cấu hình HTTP server (dùng từ M2; có sẵn ở M1 cho example).
type ServerSection struct {
	Host            string        `koanf:"host"`
	Port            int           `koanf:"port" validate:"required,min=1,max=65535"`
	ReadTimeout     time.Duration `koanf:"read_timeout"`
	WriteTimeout    time.Duration `koanf:"write_timeout"`
	ShutdownTimeout time.Duration `koanf:"shutdown_timeout"`
	TLS             TLSSection    `koanf:"tls"`
}

// TLSSection: cấu hình TLS/mTLS cho server (HTTP và gRPC dùng chung kiểu này).
type TLSSection struct {
	Enabled      bool   `koanf:"enabled"`
	CertFile     string `koanf:"cert_file"`
	KeyFile      string `koanf:"key_file"`
	ClientCAFile string `koanf:"client_ca_file"` // set → mTLS
}

// GRPCSection: cấu hình gRPC server (dùng từ M5).
type GRPCSection struct {
	Host            string        `koanf:"host"`
	Port            int           `koanf:"port" validate:"min=0,max=65535"`
	Reflection      bool          `koanf:"reflection"`        // bật server reflection (non-prod)
	ShutdownTimeout time.Duration `koanf:"shutdown_timeout"`  // hạn graceful stop
	MaxRecvMsgSize  int           `koanf:"max_recv_msg_size"` // byte; 0 = mặc định grpc
	MaxSendMsgSize  int           `koanf:"max_send_msg_size"` // byte; 0 = mặc định grpc
	TLS             TLSSection    `koanf:"tls"`
}

// OtelSection: cấu hình OpenTelemetry (traces + metrics). Tắt mặc định để local
// chạy không cần collector; bật bằng APP_OTEL__ENABLED=true.
type OtelSection struct {
	Enabled        bool              `koanf:"enabled"`
	Endpoint       string            `koanf:"endpoint"`                            // OTLP endpoint, vd "localhost:4317"
	Protocol       string            `koanf:"protocol" validate:"oneof=grpc http"` // grpc | http
	Insecure       bool              `koanf:"insecure"`                            // true = không TLS (local/dev)
	SampleRatio    float64           `koanf:"sample_ratio" validate:"min=0,max=1"` // tỉ lệ sample trace (0..1)
	Version        string            `koanf:"version"`                             // service.version trong resource
	Headers        map[string]string `koanf:"headers"`                             // header gửi kèm OTLP (vd Authorization cho SaaS backend)
	MetricInterval time.Duration     `koanf:"metric_interval"`                     // chu kỳ export metric (mặc định 15s)
}

// DatabaseSection: cấu hình Postgres (pgxpool). DSN nếu set sẽ override các
// trường rời. Để trống toàn bộ nếu service không dùng DB (module không bật).
type DatabaseSection struct {
	DSN             string        `koanf:"dsn"` // nếu set thì ưu tiên dùng nguyên DSN
	Host            string        `koanf:"host"`
	Port            int           `koanf:"port"`
	User            string        `koanf:"user"`
	Password        string        `koanf:"password"`
	Name            string        `koanf:"name"`
	SSLMode         string        `koanf:"ssl_mode"` // disable | require | verify-full...
	MaxConns        int32         `koanf:"max_conns"`
	MinConns        int32         `koanf:"min_conns"`
	MaxConnLifetime time.Duration `koanf:"max_conn_lifetime"`
	MaxConnIdleTime time.Duration `koanf:"max_conn_idle_time"`
	ConnectTimeout  time.Duration `koanf:"connect_timeout"`
	MigrateOnStart  bool          `koanf:"migrate_on_start"` // chạy migration lúc khởi động
	MigrationsPath  string        `koanf:"migrations_path"`  // thư mục chứa file migration
}

// EventBusSection: cấu hình message/event bus (driver Kafka từ M5). Brokers
// rỗng nghĩa là service không dùng eventbus.
type EventBusSection struct {
	Brokers      []string      `koanf:"brokers"`
	GroupID      string        `koanf:"group_id"`      // consumer group mặc định
	RetryMax     int           `koanf:"retry_max"`     // số lần thử lại handler
	RetryBackoff time.Duration `koanf:"retry_backoff"` // backoff giữa các lần thử
	DLQEnabled   bool          `koanf:"dlq_enabled"`   // đẩy DLQ khi hết retry
	DLQSuffix    string        `koanf:"dlq_suffix"`    // hậu tố DLQ topic
	Concurrency  int           `koanf:"concurrency"`   // số reader song song mỗi Subscribe
	AsyncRetry   bool          `koanf:"async_retry"`   // retry phi-chặn qua retry-topic
	RetrySuffix  string        `koanf:"retry_suffix"`  // hậu tố retry-topic
}

// AuthSection: xác thực JWT (dùng từ module auth).
type AuthSection struct {
	Algorithm    string `koanf:"algorithm"`      // HS256/RS256...
	Secret       string `koanf:"secret"`         // HMAC secret
	PublicKeyPEM string `koanf:"public_key_pem"` // RSA public key (PEM)
	Issuer       string `koanf:"issuer"`
	Audience     string `koanf:"audience"`
	ScopeClaim   string `koanf:"scope_claim"` // mặc định "scope"
}

// CacheSection: cấu hình Redis (dùng từ module cache).
type CacheSection struct {
	Addrs        []string      `koanf:"addrs"` // 1 addr = standalone; nhiều = cluster
	Password     string        `koanf:"password"`
	DB           int           `koanf:"db"` // chỉ standalone
	PoolSize     int           `koanf:"pool_size"`
	MinIdleConns int           `koanf:"min_idle_conns"`
	DialTimeout  time.Duration `koanf:"dial_timeout"`
	ReadTimeout  time.Duration `koanf:"read_timeout"`
	WriteTimeout time.Duration `koanf:"write_timeout"`
}

// DefaultAppConfig trả giá trị mặc định hợp lý cho AppConfig.
func DefaultAppConfig() *AppConfig {
	return &AppConfig{
		App: AppSection{
			Name: "service",
			Env:  "local",
		},
		Log: LogSection{
			Level:  "info",
			Format: "json",
		},
		Server: ServerSection{
			Host:            "0.0.0.0",
			Port:            8080,
			ReadTimeout:     15 * time.Second,
			WriteTimeout:    15 * time.Second,
			ShutdownTimeout: 10 * time.Second,
		},
		GRPC: GRPCSection{
			Host:            "0.0.0.0",
			Port:            9090,
			Reflection:      true,
			ShutdownTimeout: 10 * time.Second,
		},
		Otel: OtelSection{
			Enabled:     false,
			Endpoint:    "localhost:4317",
			Protocol:    "grpc",
			Insecure:    true,
			SampleRatio: 1.0,
		},
		Database: DatabaseSection{
			Host:            "localhost",
			Port:            5432,
			SSLMode:         "disable",
			MaxConns:        10,
			MinConns:        2,
			MaxConnLifetime: time.Hour,
			MaxConnIdleTime: 30 * time.Minute,
			ConnectTimeout:  5 * time.Second,
			MigrationsPath:  "migrations",
		},
		EventBus: EventBusSection{
			Brokers:      []string{"localhost:9092"},
			RetryMax:     3,
			RetryBackoff: 200 * time.Millisecond,
			DLQEnabled:   true,
			DLQSuffix:    ".DLQ",
		},
		Auth: AuthSection{
			Algorithm:  "HS256",
			ScopeClaim: "scope",
		},
		Cache: CacheSection{
			Addrs:        []string{"localhost:6379"},
			PoolSize:     10,
			MinIdleConns: 2,
			DialTimeout:  5 * time.Second,
			ReadTimeout:  3 * time.Second,
			WriteTimeout: 3 * time.Second,
		},
	}
}
