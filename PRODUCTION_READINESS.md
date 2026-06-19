# Production Readiness Checklist — common-go

> Trạng thái sẵn sàng production của foundation library `common-go`.
> Cập nhật: 2026-06-19 · Verdict: **✅ Sẵn sàng cho service nội bộ**
> Quy ước: `[x]` đạt · `[~]` đạt một phần / tuỳ service · `[ ]` chưa làm (hoãn có lý do)

---

## Tổng quan số liệu

| Chỉ số | Giá trị |
|---|---|
| Số package | 18 (core + transport + client + infra; chưa tính example) |
| File Go (không tính test) | 72 |
| Test functions | 131 |
| `go test -race ./...` | ✅ pass toàn bộ (race-clean) |
| `go vet` / `gofmt -l` | ✅ sạch |
| Go version | 1.25+ (ràng buộc OTel SDK v1.44) |
| Direct deps (go.mod) | ~91 dòng require (otel + grpc + kafka + pgx + redis + jwt) |

### Coverage theo nhóm
| Nhóm | Coverage | Ghi chú |
|---|---|---|
| Tầng chặn request (transport) | httpserver 74% · middleware 88% · grpcserver 71% | ✅ ≥70% |
| Logic thuần | errors 70% · httpclient 78% · tlsconfig 79% · health 88% · logger 69% | ✅ tốt |
| Infra-coupled | database 26% · cache 17% · observability 24% · kafka 59% · outbox 12% | 🟡 phần wiring cần DB/Redis/Kafka → phủ bằng test gated `integration` |

---

## 1. Correctness & Concurrency
- [x] `go test -race` xanh toàn bộ; không data race
- [x] httpclient retry không mutate request của caller (clone) + jitter
- [x] Hook toàn cục (`logger` extractors, `response` metaFrom) dùng `atomic.Pointer` (an toàn goroutine)
- [x] Goroutine có vòng đời rõ ràng (rate-limit janitor, eventbus consumer, outbox relay dừng theo ctx)
- [x] `context.Context` là tham số đầu, không lưu trong struct
- [x] Không global mutable state (trừ 2 hook atomic set-once, có chủ đích)

## 2. Khởi động & Tắt (lifecycle)
- [x] **Lỗi bind cổng làm fail startup** (http+grpc tách `Listen` đồng bộ / `Serve` goroutine) — đã verify end-to-end
- [x] Fail-fast: config sai / không kết nối được DB/Redis → fail lúc start
- [x] Graceful shutdown qua `fx.Lifecycle` (server stop → đóng pool/exporter, thứ tự ngược)
- [x] Startup connect có timeout tường minh (db/cache/observability)
- [x] Build info (version/commit/buildtime) qua `-ldflags` → `/info`

## 3. Bảo mật (Security)
- [x] TLS/mTLS cho HTTP và gRPC (`tlsconfig`, MinVersion 1.2)
- [x] Xác thực JWT (HMAC/RSA) + JWKS rotation (OIDC) — HTTP middleware + gRPC interceptor + scopes
- [x] Secret nạp từ file (`file:` ref, hợp Docker/K8s secret)
- [x] Redaction secret khi log (`slog.LogValuer` cho DB/Auth/Cache section)
- [x] Redact query param nhạy cảm trong access log
- [x] Recovery không lộ stack/panic detail ra client
- [x] Endpoint `/admin/*` bảo vệ được (`WithAdminGuard`)
- [~] Body size limit, security headers, CORS — có sẵn, **service tự bật** theo nhu cầu

## 4. Observability
- [x] Structured logging (slog JSON) + `request_id`/`trace_id` tự động
- [x] OTel traces + metrics + runtime metrics; propagator W3C
- [x] No-op khi tắt (local không cần collector)
- [x] OTLP cấu hình đầy đủ: endpoint/protocol/headers (auth SaaS)/sample ratio/metric interval
- [x] Auto-instrument HTTP (otelgin/otelhttp), gRPC (otelgrpc), DB (otelpgx), Redis (redisotel)
- [x] Helper app-metrics (`observability.Meter/Counter/Histogram`)
- [x] Dynamic log level runtime (`/admin/loglevel`)

## 5. Resilience
- [x] httpclient: retry (idempotent + backoff + jitter + Retry-After), circuit breaker, per-attempt timeout
- [x] Rate limit: in-memory (per-IP) + **phân tán qua Redis** (`cache.RedisLimiter`), fail-open mặc định
- [x] Health: `/healthz` (liveness) + `/readyz` (readiness, checker chạy **song song**)
- [x] Eventbus: at-least-once, retry inline HOẶC async retry-topic (không chặn partition), DLQ
- [x] Eventbus concurrency (N reader/partition song song)
- [x] Distributed lock có fencing token + `Refresh`/`Watchdog`
- [x] Transactional outbox (publish atomic với DB tx)

## 6. Error handling & API contract
- [x] Error model thống nhất (`AppError`: Kind/Code/Message) map sang cả HTTP & gRPC
- [x] Response envelope chuẩn `{data, error, meta}` + pagination
- [x] Validation request (`BindJSON[T]` → field errors)
- [x] `errors.Is/As/Unwrap` tương thích

## 7. Config
- [x] Precedence defaults → YAML → env (12-factor); validate fail = startup fail
- [x] Env prefix `APP_`, nested `__`; mỗi capability một section
- [x] Service embed `AppConfig` (`koanf:",squash"`) + section riêng

## 8. Testing & CI
- [x] Unit test race-clean; example chạy được
- [x] Tầng chặn request coverage ≥70%
- [x] Integration test gated `//go:build integration` (db/cache/outbox) — tự skip khi thiếu env
- [x] `.github/workflows/ci.yml`: lint + `go test -race -cover` + job integration (pg/redis)
- [~] Cần **runner Docker** trong CI để chạy job integration

## 9. Docs & Versioning
- [x] DESIGN.md (kiến trúc, decisions, roadmap), CHANGELOG (Keep a Changelog), README, CLAUDE.md
- [x] Godoc cho mọi exported symbol (tiếng Việt theo convention)
- [~] Semver: đang v0.x — lên v1.0.0 khi API ổn định; breaking change cần migration note

---

## Hoãn có chủ đích (không chặn prod)
- [ ] **Service discovery** (Consul/etcd/Nacos) — thường để k8s + DNS lo
- [ ] **Config hot-reload + Vault** — cần chốt secret manager (đã có secret `file:` ref)
- [ ] **Tách multi-module** (kafka/migrate) — đánh giá: chưa đáng đổi breaking change ở v0.x (Go không compile package không import vào binary consumer). Xem DESIGN §10.3

---

## Việc service PHẢI làm khi tích hợp (per-service)
- [ ] Bật TLS prod (`server.tls.*` / `grpc.tls.*`) hoặc terminate ở ingress/mesh
- [ ] Bảo vệ `/admin/*` (`WithAdminGuard`) hoặc chặn ở mạng nội bộ
- [ ] Đặt `APP_OTEL__ENABLED=true` + endpoint collector ở prod
- [ ] Cấu hình secret qua `file:` ref / secret manager (không hardcode)
- [ ] Bật rate limit / body limit / security headers / CORS theo nhu cầu endpoint
- [ ] Nếu dùng outbox: thêm `outbox.Schema` vào migration; consumer phải **idempotent**
- [ ] Set readiness/liveness probe k8s trỏ `/readyz` `/healthz`
