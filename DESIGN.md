# common-go — Design Document

> Foundation library mã nguồn mở cho Go microservice.
> Module path: `github.com/aupv9/common-go`
> Status: **Draft v0.1** · Last updated: 2026-06-18

---

## 1. Mục tiêu & phi mục tiêu

### 1.1 Mục tiêu
- Cung cấp **foundation** ("auto-config" theo phong cách Go) để một service mới khởi tạo nhanh, đồng nhất config / logging / observability / error handling / transport.
- Giảm boilerplate: một service mới chỉ cần `fx.New(foundation.Core(), foundation.WithHTTP(), ...)` là có sẵn nền tảng production-ready.
- Đồng nhất convention giữa mọi Go service: cùng cấu trúc config, cùng format log, cùng error model, cùng response envelope, cùng health endpoint, cùng cách export trace/metric.
- Mỗi capability là một **`fx.Module`** độc lập, import lẻ được, override bằng functional options.

### 1.2 Phi mục tiêu (non-goals)
- **KHÔNG** tái tạo "magic" của Spring Boot (classpath scanning, conditional bean ẩn). Go ưu tiên *explicit over implicit*.
- **KHÔNG** ép service phải lấy nguyên cả lib. Mỗi sub-package phải dùng được riêng lẻ.
- **KHÔNG** ôm business logic. Đây là infrastructure foundation, không phải domain framework.
- **KHÔNG** wrap lib chỉ để wrap. Chỉ abstraction khi tạo ra giá trị (đồng nhất, swap được driver, gắn observability).

### 1.3 Nguyên tắc thiết kế (design principles)
1. **Stdlib-first**: `log/slog`, `context`, `net/http`, `database/sql` làm nền. Chỉ thêm dependency khi thật sự cần.
2. **Explicit wiring**: dùng `uber/fx` module + functional options; không global mutable state, không `init()` side-effect.
3. **Composable**: mỗi module độc lập; `foundation.*` chỉ là tiện ích gom nhóm các module phổ biến.
4. **Context-first**: mọi API nhận `context.Context` đầu tiên; trace_id / request_id truyền qua context.
5. **Fail fast at startup**: config sai, dependency thiếu → fail ngay khi khởi động, không fail lúc runtime.
6. **Observable by default**: log có trace_id, http/grpc có tracing/metrics middleware mặc định.
7. **Backward compatible**: lib là dependency của mọi service → semver nghiêm túc, breaking change phải có lý do và migration note.

---

## 2. Bối cảnh & tham chiếu

Các Go service thường lặp lại cùng một lớp hạ tầng: đọc config, log, trace/metric, health, error model, server và client. `common-go` gom lớp đó thành một thư viện dùng chung để mỗi service chỉ còn phần nghiệp vụ.

Tham chiếu cách enterprise/community làm:
- **fx** (Uber) — DI runtime + lifecycle, mô hình module → nền tảng wiring của lib này.
- **Kratos** (Bilibili), **go-zero**, **Kitex/CloudWeGo** (ByteDance) — full microservice framework Go; tham khảo layout, config, middleware.
- **OpenTelemetry** — chuẩn de-facto cho traces/metrics/logs.
- **koanf** (config), **slog** (logging), **pgx** (Postgres), **golang-migrate** (migration), **go-playground/validator** (validation).

---

## 3. Yêu cầu kỹ thuật (đã chốt)

| Hạng mục | Quyết định |
|---|---|
| DI / wiring | **uber/fx** (module + lifecycle) |
| Transport | **HTTP REST + gRPC + Message/Event** |
| Scope v0.1 | **Core foundation + Observability + Database + HTTP client** |
| Go version | **1.25+** (theo `go.mod`; ràng buộc bởi OpenTelemetry SDK v1.44) |
| Module path | `github.com/aupv9/common-go` |

---

## 4. Kiến trúc tổng thể

```
common-go/
├── go.mod                          # github.com/aupv9/common-go
├── DESIGN.md                       # tài liệu này
├── README.md
├── CHANGELOG.md
│
├── config/                         # fx.Module — koanf: defaults → yaml → env; Load[T] + validate
├── logger/                         # fx.Module — slog JSON; trace_id/request_id từ context
├── observability/                  # fx.Module — OTel tracer + meter + propagator; shutdown qua lifecycle
├── errors/                         # error model + mã lỗi chuẩn (sentinel + AppError)
├── response/                       # response envelope chuẩn (REST)
├── health/                         # fx.Module — /healthz /readyz + build info
│
├── transport/
│   ├── httpserver/                 # fx.Module — chi + middleware chuẩn
│   │   └── middleware/
│   ├── grpcserver/                 # fx.Module — grpc-go + interceptors
│   └── eventbus/                   # fx.Module — Publisher/Subscriber abstraction + driver
│       └── driver/                 # nats / kafka / mqtt
│
├── client/
│   └── httpclient/                 # fx.Module — factory: retry + timeout + otel propagation
│
├── database/                       # fx.Module — pgxpool + migration + health hook
│
├── foundation/                     # tiện ích gom module: Core(), WithHTTP(), WithGRPC()...
│
└── examples/
    └── minimal-service/            # service mẫu chạy được
```

### 4.1 Sơ đồ phụ thuộc giữa package (DAG, không cycle)

```
errors  ──────────────┐
response ──> errors    │
config ───────────────┤
logger ──> config      │
observability ──> config, logger
health ──> observability
httpserver ──> logger, observability, response, errors
grpcserver ──> logger, observability, errors
eventbus  ──> logger, observability
httpclient ──> logger, observability
database  ──> config, logger, observability, health
foundation ──> (tất cả module trên)
```

Quy tắc: **package "thấp" không import package "cao"**. `errors`/`response` là lá, không phụ thuộc gì trong lib (trừ stdlib). `foundation` là đỉnh, chỉ nó được import tất cả.

---

## 5. Mô hình wiring với fx

### 5.1 Quy ước module
- Mỗi capability export một biến `Module` kiểu `fx.Option`, đặt tên `Module` ở cấp package:
  ```go
  // package logger
  var Module = fx.Module("logger",
      fx.Provide(New),
      fx.Invoke(func(*slog.Logger) {}), // ép khởi tạo eager nếu cần
  )
  ```
- Constructor đặt tên `New`, nhận dependency qua tham số (fx tự inject), trả về `(T, error)`.
- Cleanup (đóng pool, flush tracer...) đăng ký qua `fx.Lifecycle.Append(OnStop)`.

### 5.2 foundation — gom nhóm
```go
package foundation

// Core: nền tảng tối thiểu mọi service cần.
func Core() fx.Option {
    return fx.Options(
        config.Module,
        logger.Module,
        observability.Module,
        health.Module,
        fx.WithLogger(...), // route log của fx qua slog
    )
}

func WithHTTP(opts ...httpserver.Option) fx.Option { ... }
func WithGRPC(opts ...grpcserver.Option) fx.Option { ... }
func WithPostgres(opts ...database.Option) fx.Option { ... }
func WithHTTPClient(opts ...httpclient.Option) fx.Option { ... }
func WithEventBus(opts ...eventbus.Option) fx.Option { ... }
```

### 5.3 Trải nghiệm service dev (mục tiêu)
```go
func main() {
    fx.New(
        foundation.Core(),
        foundation.WithHTTP(),
        foundation.WithPostgres(),

        fx.Provide(
            NewOrderRepository,
            NewOrderService,
            NewOrderHandler,
        ),
        fx.Invoke(routes.Register),
    ).Run()
}
```

### 5.4 Functional options pattern (chuẩn cho mọi component dùng ngoài fx)
```go
type Option func(*config)

func WithTimeout(d time.Duration) Option { return func(c *config) { c.timeout = d } }

func New(opts ...Option) (*Server, error) {
    c := defaultConfig()
    for _, o := range opts { o(&c) }
    ...
}
```
→ Mọi component dùng được **cả trong fx lẫn standalone** (gọi `New(...)` trực tiếp).

---

## 6. Đặc tả từng module

### 6.1 `config`
- **Lib**: `knadh/koanf` (gọn, ít magic) + `go-playground/validator`.
- **Precedence** (thấp → cao): defaults trong struct tag → file `config.yaml` → biến môi trường (12-factor, ưu tiên cao nhất cho container/k8s).
- **API**:
  ```go
  // Load đọc theo precedence, validate, trả struct đã điền.
  func Load[T any](opts ...Option) (*T, error)
  ```
- Env mapping: prefix theo service (vd `APP_`), nested bằng `__` (vd `APP_DATABASE__HOST`).
- Module export `config.Module` provide `*AppConfig` chung (cho logger/db dùng), service tự `fx.Provide` config riêng nếu cần.
- **Validate fail → startup fail** (fail fast).

### 6.2 `logger`
- **Lib**: `log/slog` (stdlib). Handler JSON cho production, text cho local (chọn qua config).
- Tự inject `trace_id`, `span_id`, `request_id` từ `context.Context` vào mỗi log record (custom handler bọc slog).
- API:
  ```go
  func New(cfg Config) *slog.Logger
  func FromContext(ctx context.Context) *slog.Logger   // logger đã gắn context fields
  func IntoContext(ctx, *slog.Logger) context.Context
  ```
- Route log nội bộ của fx/grpc/http qua slog để format đồng nhất.
- Level điều chỉnh runtime (qua config reload hoặc endpoint, để sau).

### 6.3 `observability`
- **Lib**: OpenTelemetry Go SDK.
- Cung cấp `TracerProvider`, `MeterProvider`, propagator (W3C TraceContext + Baggage).
- Exporter: OTLP (gRPC/HTTP) — endpoint từ config; fallback no-op khi tắt.
- Đăng ký shutdown (flush) qua `fx.Lifecycle.OnStop`.
- Liên kết với logger: span context → log fields.
- Runtime metrics (Go runtime, process) bật mặc định.

### 6.4 `errors`
- Error model chuẩn toàn hệ thống:
  ```go
  type AppError struct {
      Code     string   // mã ổn định, vd "ORDER_NOT_FOUND"
      Message  string   // human-readable
      HTTP     int      // map sang HTTP status
      GRPC     codes.Code
      Cause    error    // wrap, hỗ trợ errors.Is/As
      Metadata map[string]any
  }
  ```
- Sentinel errors + constructor: `errors.NotFound(code, msg)`, `errors.Invalid(...)`, `errors.Internal(...)`.
- Tương thích `errors.Is` / `errors.As` / `Unwrap`.
- Dùng chung cho cả HTTP (qua response) và gRPC (qua interceptor) → một error model, hai transport.

### 6.5 `response`
- Envelope REST chuẩn:
  ```json
  { "data": {...}, "error": null, "meta": { "request_id": "...", "trace_id": "..." } }
  ```
- Helper: `response.OK(w, data)`, `response.Error(w, err)` (tự map `AppError` → HTTP status + body).
- Pagination meta chuẩn cho list endpoint.

### 6.6 `health`
- Endpoint `/healthz` (liveness) và `/readyz` (readiness — kiểm tra dependency: db, eventbus...).
- `/info` hoặc `/buildinfo`: version, git commit, build time (inject qua `-ldflags`).
- Đăng ký health-check qua interface; module nào (db, eventbus) tự đăng ký checker của mình.

### 6.7 `transport/httpserver`
- **Lib**: `go-chi/chi` (router nhẹ, idiomatic, `net/http` thuần).
- Middleware chuẩn bật mặc định (thứ tự rõ ràng): RequestID → Recovery → OTel tracing → Logging → Timeout → CORS.
- Graceful shutdown qua `fx.Lifecycle`.
- Server config (port, timeouts) từ config module.
- Service đăng ký route qua `fx.Invoke`.

### 6.8 `transport/grpcserver`
- **Lib**: `grpc-go`.
- Interceptor chuẩn (unary + stream): Recovery → OTel → Logging → error mapping (`AppError` → gRPC status).
- Reflection bật ở môi trường non-prod.
- Health service (grpc.health.v1) tích hợp với module `health`.
- Graceful shutdown qua lifecycle.

### 6.9 `transport/eventbus`
- Abstraction:
  ```go
  type Publisher interface { Publish(ctx, topic string, msg Message) error }
  type Subscriber interface { Subscribe(ctx, topic string, h Handler) error }
  ```
- Driver swap được: `driver/nats`, `driver/kafka`, `driver/mqtt` (chọn 1 cho v đầu — xem mục 9).
- Tự propagate trace context qua message header.
- At-least-once semantics, retry + DLQ policy (config).

### 6.10 `client/httpclient`
- Factory tạo `*http.Client` production-ready:
  - Timeout, connection pool tuning.
  - Retry (exponential backoff, chỉ retry idempotent / theo policy).
  - OTel propagation (inject trace header) + metrics.
  - Tùy chọn circuit breaker (để sau).
- API: `httpclient.New(opts...) *http.Client` + `httpclient.Module`.

### 6.11 `database`
- **Lib**: `jackc/pgx/v5` (pgxpool) cho Postgres.
- Migration: `golang-migrate` (chạy lúc startup tùy config, hoặc CLI riêng).
- Pool config từ config module; đăng ký health checker vào `health`.
- Tracing query qua OTel (otelpgx).
- Shutdown pool qua lifecycle.
- (Mở rộng sau: Redis, read-replica routing.)

---

## 7. Convention & quy ước code

- **Package naming**: ngắn, số ít, không stutter (`config`, không `configpkg`).
- **Errors**: luôn wrap với context (`fmt.Errorf("...: %w", err)`); dùng `errors.AppError` cho lỗi domain-facing.
- **Context**: tham số đầu tiên, không lưu trong struct.
- **No global state**: không package-level mutable var; mọi thứ qua DI.
- **Constructor**: `New(...)` trả `(T, error)`; option pattern cho config.
- **Testing**: table-driven; `testify` cho assert; integration test dùng `testcontainers-go` (db/eventbus). Mỗi public package có example test.
- **Linting**: `golangci-lint` (cấu hình chuẩn ở repo root); CI bắt buộc pass.
- **Comment**: doc comment cho mọi exported symbol (godoc).

---

## 8. Versioning & phân phối

- **Semver**: `v0.x` giai đoạn đầu (API có thể đổi); lên `v1.0.0` khi ổn định.
- **CHANGELOG.md**: Keep a Changelog format; mọi breaking change ghi rõ + migration note.
- **Tag**: git tag `vX.Y.Z`; service pin version trong `go.mod`.
- **Module công khai (OSS, MIT)**: `go get`-able bình thường, không cần `GOPRIVATE`; phân phối qua proxy Go mặc định.
- **Một module duy nhất** (single go.mod) ở v0.x cho đơn giản; cân nhắc tách multi-module nếu dependency của một capability quá nặng (vd kafka) — đánh giá sau.

---

## 9. Quyết định (đã/ chưa chốt)

| # | Vấn đề | Quyết định | Trạng thái |
|---|---|---|---|
| 1 | HTTP router | **`gin`** (team quen hơn) | ✅ Chốt |
| 2 | Event driver đầu tiên | **Kafka** (`segmentio/kafka-go`, pure-Go, no cgo) | ✅ Chốt + triển khai M5 |
| 3 | Config file format | YAML (`koanf`) | ✅ Chốt (M1) |
| 4 | OTLP backend | OTLP gRPC/HTTP → OTel Collector (endpoint qua config) | ✅ Chốt (M3); no-op khi tắt |
| 5 | Migration chạy ở đâu | Startup (cờ `database.migrate_on_start`) | ✅ Chốt (M4) |

> Tất cả quyết định #1–#5 đã chốt và triển khai. gRPC error map (`errors.Kind` → gRPC `codes`) đặt tại `transport/grpcserver` để giữ `errors` là package lá chỉ phụ thuộc stdlib.

---

## 10. Roadmap

| Mốc | Nội dung | Output | Trạng thái |
|---|---|---|---|
| **M1 — Skeleton** | `go.mod`, `config`, `logger`, `health`, `errors`, `response`, `foundation.Core()`, example chạy được | Khung xương + 1 service mẫu | ✅ Done |
| **M2 — HTTP** | `httpserver` + middleware, `httpclient` | REST foundation | ✅ Done |
| **M3 — Observability** | OTel wiring đầy đủ vào logger/http | Tracing/metrics end-to-end | ✅ Done |
| **M4 — Database** | `database` pgx + migration + health | DB foundation | ✅ Done |
| **M5 — gRPC + Event** | `grpcserver`, `eventbus` (+ Kafka driver) | Đủ 3 transport | ✅ Done |
| **M6 — Enterprise hardening** | `auth` (JWT), `cache` (Redis), resilience middleware (rate limit / body limit / security headers / gzip), circuit breaker, dynamic log level | Bảo mật + resilience + vận hành | ✅ Done |

### 10.1 Hardening đã làm (đợt 1–3)
- **Correctness (đợt 1):** httpclient retry không mutate request gốc (clone) + jitter; logger/response hook chuyển atomic (hết data race); rate-limit janitor gắn lifecycle; readiness chạy song song.
- **Security (đợt 2):** TLS/mTLS cho HTTP + gRPC (`tlsconfig`); secret `file:` ref + redaction khi log; redact query param nhạy cảm; per-attempt timeout cho httpclient.
- **Scale & ergonomics (đợt 3):** app-metrics helper (`observability`); validation helper `BindJSON[T]`; distributed rate limit (`cache.RedisLimiter` + `middleware.Limiter`); distributed lock `Refresh`/`Watchdog`/fencing token.
- **Reliability (đợt 4):** OTLP headers + metric interval cấu hình được; eventbus `Concurrency` (N reader song song); auth `JWKSVerifier` (xoay key theo `kid`); integration tests gated `integration` cho db/cache.

### 10.2 Roadmap mở rộng còn lại
| Hạng mục | Trạng thái / Ghi chú |
|---|---|
| Eventbus retry phi-chặn (retry-topic) | ✅ Làm: `AsyncRetry` + `RetrySuffix` (đẩy `<topic>.retry`, escalate DLQ theo `x-retry-count`) |
| Outbox / idempotency | ✅ Làm: package `transport/eventbus/outbox` (`Save` trong tx + `Relay` FOR UPDATE SKIP LOCKED) |
| Integration tests trong CI | ✅ Khung gated `integration` + `.github/workflows/ci.yml` (job pg/redis); GitHub Actions |
| Service discovery (Consul/etcd/Nacos) | Hoãn — thường để k8s + DNS lo; thêm khi chạy ngoài k8s |
| Config hot-reload + secrets (Vault) | Hoãn — cần chốt secret manager; hiện đã có secret `file:` ref |

### 10.3 Đánh giá tách multi-module (D3) — **kết luận: hoãn**
Vấn đề: go.mod có ~91 require (otel+kafka+migrate+grpc+redis); service không dùng eventbus vẫn thấy `kafka-go` trong go.sum.
Đánh giá: Go **không biên dịch package không được import** vào binary consumer → chi phí thực chỉ là kích thước `go.sum` và thời gian `go mod download` ở CI, không ảnh hưởng binary. Tách multi-module là **breaking change** (đổi import path, cần version bump + phối hợp mọi service). Kết luận: **chưa đáng** đánh đổi; chỉ tách khi go.sum/download trở thành điểm đau thực sự. Giữ single-module ở v0.x như §8.

Mỗi mốc: code + test + cập nhật README + ví dụ. Review API sau M1 trước khi nhân rộng.

---

## 11. Cấu trúc service mẫu (target cho examples/minimal-service)

```
minimal-service/
├── main.go              # fx.New(foundation.Core(), foundation.WithHTTP(), ...)
├── config.yaml
└── internal/
    ├── handler/
    ├── service/
    └── routes.go
```

`main.go` mục tiêu ~30 dòng, đọc là hiểu toàn bộ wiring.
