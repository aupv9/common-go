# Changelog

Tất cả thay đổi đáng chú ý của `common-go` được ghi tại đây.
Định dạng theo [Keep a Changelog](https://keepachangelog.com/), versioning theo [SemVer](https://semver.org/).

## [Unreleased]

### Changed (open source — đợt 6)
- **BREAKING — đổi module path:** module path cũ (private) → `github.com/aupv9/common-go`. Service consumer cập nhật import + `go.mod`; không còn cần `GOPRIVATE`.
- **License:** thêm `LICENSE` (MIT) — thư viện trở thành mã nguồn mở.
- **CLI:** thêm `cmd/common-go` — `common-go new <name>` scaffold service chạy được (flags `--postgres --cache --grpc --eventbus --module --dir`). Chỉ dùng stdlib, không thêm dependency.
- **CI:** chuyển từ `.gitlab-ci.yml` sang `.github/workflows/ci.yml` (GitHub Actions: lint + test race/cover + integration pg/redis).

### Fixed / Added (production hardening — đợt 5)
- **Fixed:** lỗi bind cổng lúc khởi động không còn bị nuốt — `httpserver`/`grpcserver` tách `Listen()` (đồng bộ, bind) khỏi `Serve()`; `fx` OnStart bind đồng bộ nên cổng bận → fail startup ngay.
- `database`/`cache`/`observability`: khởi tạo lúc start dùng context có timeout tường minh (thay `context.Background()` trần).
- `transport/httpserver`: `WithAdminGuard` — bảo vệ nhóm `/admin` (vd `/admin/loglevel`) bằng middleware (auth/IP allowlist).
- **Eventbus async retry-topic:** `AsyncRetry` + `RetrySuffix` — đẩy message lỗi sang `<topic>.retry` (header `x-retry-count`) thay vì sleep chặn partition; vượt `RetryMax` → DLQ.
- **Outbox pattern:** package `transport/eventbus/outbox` — `Save` (ghi trong transaction nghiệp vụ) + `Relay` (FOR UPDATE SKIP LOCKED, publish-then-mark, at-least-once); `foundation.WithOutbox`.
- **CI:** `.github/workflows/ci.yml` (lint + `go test -race -cover` + job integration pg/redis).
- **Tests:** nâng coverage tầng transport ≥70% (httpserver 74%, middleware 88%, grpcserver 71%); test xác minh bind-fail.
- **Docs:** đồng bộ yêu cầu Go **1.25+** (ràng buộc bởi OTel SDK v1.44).

### Added (reliability — đợt 4)
- `observability`: cấu hình OTLP đầy đủ — `otel.headers` (vd Authorization cho backend SaaS) và `otel.metric_interval` (chu kỳ export metric), wire vào cả trace lẫn metric exporter (grpc/http).
- `transport/eventbus/driver/kafka`: `Concurrency` — N reader song song mỗi `Subscribe` (Kafka chia partition → tăng throughput, giữ thứ tự trong từng partition). Map từ `eventbus.concurrency`.
- `auth`: `JWKSVerifier` — xác thực JWT RSA bằng key lấy từ JWKS endpoint (OIDC), chọn key theo `kid`, tự refresh khi gặp kid lạ (có throttle `RefreshInterval`) → hỗ trợ xoay key.
- Integration tests gated `//go:build integration` cho `database` (Postgres) và `cache` (Redis/lock/rate-limit) — dùng `TEST_POSTGRES_DSN`/`TEST_REDIS_ADDR`, tự skip nếu chưa set (không thêm dependency).

### Added (scale & ergonomics — đợt 3)
- `observability`: helper app-metrics — `Meter`, `Int64Counter`/`MustInt64Counter`, `Float64Histogram`/`MustFloat64Histogram`, `Int64UpDownCounter` (dùng global MeterProvider, no-op khi tắt).
- `transport/httpserver`: `BindJSON[T]` + `Validate` — bind JSON và validate (`go-playground/validator`), lỗi trả `AppError{Invalid}` kèm map field→tag.
- `cache`: distributed lock nâng cấp — `Lock.Token()`/`Key()` (fencing token), `Lock.Refresh` (gia hạn TTL atomic theo token), `Lock.Watchdog` (tự gia hạn định kỳ); thêm `ErrLockLost`.
- `cache`: `RedisLimiter` — rate limit token-bucket phân tán (Lua + Redis `TIME`), dùng chung quota xuyên replica.
- `transport/httpserver/middleware`: `RateLimit` nhận `Limiter` interface (cắm backend phân tán) + `FailClosed` (mặc định fail-open khi backend lỗi).

### Added (security hardening — đợt 2)
- `tlsconfig`: package lá dựng `*tls.Config` từ file, hỗ trợ TLS và mTLS (`RequireAndVerifyClientCert`), MinVersion TLS 1.2.
- `transport/httpserver`: `WithTLS` + `Server.Start` dùng `ListenAndServeTLS` khi bật; map từ `server.tls.*`.
- `transport/grpcserver`: `WithTLS` (qua `grpc.Creds`); map từ `grpc.tls.*`.
- `config`: `ResolveSecret`/`ResolveSecretOr` — quy ước `file:/path` nạp secret từ file (Docker/K8s secret), áp dụng cho DB/cache/auth.
- `config`: `DatabaseSection`/`AuthSection`/`CacheSection` implement `slog.LogValuer` — che password/secret/DSN khi log.
- `transport/httpserver/middleware`: `AccessLog` redact giá trị query param nhạy cảm (token/password/api_key/...).
- `client/httpclient`: `WithPerAttemptTimeout` — timeout mỗi lần thử (chuyển quyền huỷ sang lúc đóng body của response cuối).
- `config`: thêm `TLSSection` (trong `server.tls` và `grpc.tls`).

### Fixed (correctness hardening — đợt 1)
- `client/httpclient`: `retryTransport` không còn sửa request của caller (tuân thủ hợp đồng `http.RoundTripper`) — clone request rồi mới buffer/replay body.
- `client/httpclient`: thêm "equal jitter" cho exponential backoff (chống thundering herd); `Retry-After` của server vẫn được tôn trọng nguyên giá trị.
- `logger`: `RegisterContextExtractor`/đọc extractor chuyển sang `atomic.Pointer` (copy-on-write) — an toàn goroutine, hết data race tiềm ẩn ở hot path log.
- `response`: hook `SetMetaFunc` lưu qua `atomic.Pointer` — đọc/ghi an toàn goroutine.
- `transport/httpserver/middleware`: janitor của `RateLimit` dừng theo `RateLimitConfig.Context` (gắn lifecycle, hết rò goroutine khi tạo lặp lại).
- `health`: readiness chạy các checker SONG SONG (tổng latency = checker chậm nhất thay vì tổng).

### Added (M6 — Enterprise hardening)
- `auth`: xác thực JWT (HMAC/RSA) độc lập transport — `Verifier`, `Principal` (subject/issuer/scopes/claims), HTTP middleware `Middleware`/`RequireScopes`, gRPC `UnaryInterceptor`/`StreamInterceptor`/`CheckScopes`. fx `Module` cấp `Verifier`.
- `cache`: client Redis (`go-redis`, standalone/cluster) — pool tuning, `redisotel` tracing/metrics, health checker, distributed lock (`AcquireLock`/`Release`, SET NX + Lua release). fx `Module`.
- `transport/httpserver/middleware`: `SecurityHeaders`, `BodyLimit`, `RateLimit` (token-bucket per-key, 429), tích hợp `Gzip` (`gin-contrib/gzip`) — bật qua option `WithSecurityHeaders`/`WithBodyLimit`/`WithRateLimit`/`WithGzip`.
- `client/httpclient`: circuit breaker (`sony/gobreaker`) — `WithCircuitBreaker`; mở mạch sau N lỗi transport liên tiếp, bao ngoài retry.
- `logger`: dynamic log level lúc runtime — `LevelControl` (slog.LevelVar) + handler `GET/PUT /admin/loglevel` (httpserver tự mount).
- `config`: thêm `AuthSection`, `CacheSection`.
- `foundation`: thêm `WithAuth()`, `WithCache()`.

### Added (M5 — gRPC + Event)
- `transport/grpcserver`: gRPC server (grpc-go) với chuỗi interceptor chuẩn (context+request id → recovery → logging → error mapping) cho cả unary lẫn stream, OTel tracing (`otelgrpc`) tuỳ chọn, health service `grpc.health.v1`, reflection (non-prod) và graceful shutdown. Functional options + fx `Module`. Map `errors.Kind` → gRPC `codes` đặt tại transport (giữ `errors` là package lá).
- `transport/eventbus`: abstraction độc lập driver — `Message`, `Handler`, `Publisher`, `Subscriber` + propagate trace context qua message header (`InjectTrace`/`ExtractTrace`).
- `transport/eventbus/driver/kafka`: driver Kafka (`segmentio/kafka-go`) — Publisher (balancer hash theo key, RequireAll, producer span), Subscriber (consumer group, **at-least-once** commit thủ công, retry + backoff, **DLQ** khi hết retry, consumer span). fx `Module`.
- `config`: thêm `GRPCSection` và `EventBusSection`.
- `foundation`: thêm `WithGRPC()` và `WithEventBus()`.

### Added (M4 — Database)
- `database`: pool Postgres `pgxpool` production-ready — tuning (max/min conns, lifetimes, connect timeout), ping fail-fast lúc khởi động, tracing query qua `otelpgx` (bật khi OTel on), DSN builder từ field rời hoặc DSN trực tiếp. fx `Module` đóng pool qua lifecycle.
- `database`: `Migrate` (golang-migrate, driver pgx5 + source file), tuỳ chọn chạy "up" lúc khởi động (`migrate_on_start`), idempotent với `ErrNoChange`.
- `database`: `Checker` (health.Checker) tự đăng ký vào readiness.
- `config`: thêm `DatabaseSection`.
- `foundation`: thêm `WithPostgres()`.

### Added (M3 — Observability)
- `observability`: OpenTelemetry — TracerProvider (batch + OTLP gRPC/HTTP, sampler ParentBased ratio), MeterProvider (periodic OTLP) + runtime metrics, propagator W3C TraceContext+Baggage. No-op khi tắt (không cần collector cho local). Shutdown/flush qua `fx.Lifecycle`. `SpanLogAttrs` bơm trace_id/span_id.
- `logger`: `RegisterContextExtractor` — bơm field từ context (vd OTel) vào log mà không tạo phụ thuộc ngược.
- `config`: thêm `OtelSection` (enabled/endpoint/protocol/insecure/sample_ratio/version).
- `transport/httpserver`: tự gắn `otelgin` khi bật OTel; `response` meta lấy trace_id từ span context.
- `client/httpclient`: bọc `otelhttp` mặc định (propagation + client span); `WithoutOTel` để tắt.
- `foundation`: `Core()` thêm `observability.Module`.

### Added (M2 — HTTP)
- `transport/httpserver`: HTTP server trên `gin` với bộ middleware chuẩn (ContextLogger → RequestID → Recovery → AccessLog → Timeout → CORS), graceful shutdown qua `fx.Lifecycle`, tự gắn health endpoint và cấu hình `response` meta (request_id/trace_id). Functional options + fx `Module`.
- `transport/httpserver/middleware`: `RequestID` (sinh/propagate `X-Request-ID`), `Recovery` (panic → 500, không lộ stack), `AccessLog` (structured, skip health), `Timeout`, `CORS`.
- `client/httpclient`: factory `*http.Client` production-ready — pool tuning, timeout, retry (exponential backoff + Retry-After, chỉ method idempotent mặc định), điểm cắm OTel (`WithBaseTransport`). Functional options + fx `Module`.
- `foundation`: thêm `WithHTTP()` và `WithHTTPClient()`.
- `logger`: thêm `RequestIDFromContext` / `TraceIDFromContext`.
- `examples/minimal-service`: rút gọn về `foundation.Core()` + `foundation.WithHTTP()` (main ~20 dòng).

### Added (M1 — Skeleton)
- `errors`: error model `AppError` (Code/Message/HTTP/GRPC/Cause/Metadata) + constructor (NotFound, Invalid, Internal, Unauthorized, Forbidden, Conflict, Unavailable). Tương thích `errors.Is/As/Unwrap`.
- `response`: response envelope chuẩn (`{data,error,meta}`) + helper `OK`/`Created`/`Error`/`List`; tự map `AppError` → HTTP status.
- `config`: `Load[T]` (defaults → YAML → env) với `koanf` + validate bằng `go-playground/validator`; struct nền `AppConfig`; fx `Module`.
- `logger`: `log/slog` (JSON/text) + truyền `service`/`request_id`/`trace_id` qua context; fx `Module`.
- `health`: `/healthz` `/readyz` `/info`, `Checker` registry, build info qua `-ldflags`; fx `Module`.
- `foundation`: `Core()` gom config + logger + health, route log của fx qua slog.
- `examples/minimal-service`: service mẫu (gin) chạy được.

[Unreleased]: https://github.com/aupv9/common-go/commits/main
