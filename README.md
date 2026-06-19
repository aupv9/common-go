# common-go

> Foundation library mã nguồn mở cho Go microservice.
> Chuẩn hoá config, logging, observability, error handling và transport — wiring bằng [uber/fx](https://uber-go.github.io/fx/).

Module: `github.com/aupv9/common-go` · License: [MIT](./LICENSE)

Thiết kế chi tiết: xem [DESIGN.md](./DESIGN.md).

---

## Trạng thái

| Mốc | Nội dung | Trạng thái |
|---|---|---|
| **M1** | core: `config` · `logger` · `health` · `errors` · `response` · `foundation.Core()` + example | ✅ |
| **M2** | `transport/httpserver` (gin) + `client/httpclient` | ✅ |
| **M3** | `observability` (OpenTelemetry) | ✅ |
| **M4** | `database` (pgx + migration) | ✅ |
| **M5** | `transport/grpcserver` + `transport/eventbus` (Kafka) | ✅ |
| **M6** | `auth` (JWT) · `cache` (Redis) · resilience middleware · circuit breaker · dynamic log level | ✅ |

---

## Cài đặt

Yêu cầu **Go 1.25+** (theo `go.mod`; ràng buộc bởi OpenTelemetry SDK v1.44).

```bash
go get github.com/aupv9/common-go
```

### CLI scaffold (tuỳ chọn)
Cài CLI để sinh một service chạy được trong vài giây:

```bash
go install github.com/aupv9/common-go/cmd/common-go@latest
common-go new my-service            # tạo ./my-service
cd my-service && go mod tidy && go run .
```

Xem thêm: [cmd/common-go](./cmd/common-go).

---

## Quick start

Một service tối thiểu chỉ cần ghép module qua `fx`:

```go
package main

import (
	"github.com/aupv9/common-go/foundation"
	"go.uber.org/fx"
)

func main() {
	fx.New(
		foundation.Core(), // config + logger + observability + health (luôn có)

		foundation.WithHTTP(), // gin + middleware chuẩn + health endpoint + graceful shutdown
		// foundation.WithGRPC(),       // gRPC server + interceptor + health + reflection
		// foundation.WithPostgres(),   // pgxpool + migration + health checker
		// foundation.WithHTTPClient(), // *http.Client retry/pool/otel
		// foundation.WithEventBus(),   // Kafka publisher/subscriber + DLQ + trace
		// foundation.WithAuth(),       // JWT verifier (gắn middleware ở route)
		// foundation.WithCache(),      // Redis client + health + distributed lock

		fx.Provide(NewOrderService, NewOrderHandler),
		fx.Invoke(registerRoutes),
	).Run()
}
```

Mỗi `With*()` là một `fx.Module` độc lập, bật theo nhu cầu service. Observability (OTel)
tắt mặc định — bật bằng `APP_OTEL__ENABLED=true` (export OTLP tới collector).

Xem ví dụ đầy đủ chạy được tại [`examples/minimal-service`](./examples/minimal-service).

```bash
cd examples/minimal-service
go run .
# curl localhost:8080/healthz
# curl localhost:8080/readyz
# curl localhost:8080/info
# curl localhost:8080/api/v1/ping
```

---

## Cấu trúc

```
config/                       koanf: defaults → yaml → env, validate. Load[T]() + fx.Module
logger/                       slog (JSON/text); request_id/trace_id qua context; dynamic level
observability/                OpenTelemetry: tracer + meter + propagator + runtime metrics
health/                       /healthz /readyz /info + Checker registry + build info
errors/                       AppError model (dùng chung HTTP & gRPC)
response/                     response envelope {data,error,meta}
auth/                         JWT verifier + HTTP middleware + gRPC interceptor + scopes
cache/                        Redis (go-redis) + otel + health + distributed lock
database/                     pgxpool + golang-migrate + otelpgx + health checker
client/httpclient/            *http.Client: retry + pool + timeout + otelhttp + circuit breaker
transport/httpserver/         gin + middleware (requestid/recovery/log/timeout/cors/
                              security/ratelimit/bodylimit/gzip) + otelgin
transport/grpcserver/         grpc-go + interceptor + health + reflection + otelgrpc
transport/eventbus/           Publisher/Subscriber abstraction + trace propagation
transport/eventbus/driver/kafka/  driver Kafka (segmentio/kafka-go) + at-least-once + DLQ
foundation/                   Core() và các With*() gom module
examples/                     service mẫu
```

---

## Phát triển

```bash
go mod tidy
go build ./...
go vet ./...
golangci-lint run ./...   # nếu đã cài
go test ./...
```

Đóng góp: tuân theo convention trong [DESIGN.md](./DESIGN.md) §7 và ghi changelog tại [CHANGELOG.md](./CHANGELOG.md).
