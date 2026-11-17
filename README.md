# common-go

[![Go Reference](https://pkg.go.dev/badge/github.com/aupv9/common-go.svg)](https://pkg.go.dev/github.com/aupv9/common-go)
[![CI](https://github.com/aupv9/common-go/actions/workflows/ci.yml/badge.svg)](https://github.com/aupv9/common-go/actions/workflows/ci.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/aupv9/common-go)](https://goreportcard.com/report/github.com/aupv9/common-go)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](./LICENSE)

A batteries-included foundation library for Go microservices.

`common-go` gives every service the same config, logging, observability, error model, response envelope,
health endpoints and transports. Each capability is an independent [uber/fx](https://uber-go.github.io/fx/)
module, so a production-ready service is about 20 lines of `main.go`.

## Features

- **Config:** [koanf](https://github.com/knadh/koanf) layering (defaults → YAML → env) with validation and secret redaction.
- **Logging:** `log/slog` (JSON or text) with `request_id` / `trace_id` from context and a runtime-adjustable level.
- **Observability:** OpenTelemetry traces and metrics, OTLP or Prometheus exporters, no-op when disabled.
- **Health:** `/healthz`, `/readyz` and `/info` with a pluggable checker registry and build info.
- **Errors and responses:** one `AppError` model mapped to both HTTP status and gRPC codes, and a `{data, error, meta}` envelope.
- **HTTP server:** gin with request ID, recovery, logging, timeout, CORS, security headers, rate limit, body limit and gzip middleware.
- **gRPC server:** interceptors, health service, reflection and OTel instrumentation.
- **Event bus:** driver-agnostic publisher and subscriber, a Kafka driver with at-least-once delivery, retry topic and DLQ, plus a transactional outbox.
- **Database:** pgx pool, golang-migrate migrations and otelpgx tracing.
- **Cache:** Redis client, distributed lock and rate limiter.
- **Auth:** JWT verification (HMAC/RSA and JWKS with key rotation) for HTTP and gRPC.
- **HTTP client:** retries, connection pooling, timeouts, a circuit breaker and tracing.
- **Graceful lifecycle:** servers bind synchronously on start, so a port in use fails startup instead of being swallowed.
- **CLI:** `common-go new` scaffolds a runnable service.

## Requirements

Go 1.25 or later.

## Installation

```bash
go get github.com/aupv9/common-go
```

## Quick start

```go
package main

import (
	"github.com/aupv9/common-go/foundation"
	"go.uber.org/fx"
)

func main() {
	fx.New(
		foundation.Core(),     // config + logger + observability + health
		foundation.WithHTTP(), // gin server + standard middleware + graceful shutdown

		// Opt in to what the service needs:
		// foundation.WithGRPC(),
		// foundation.WithPostgres(),
		// foundation.WithCache(),
		// foundation.WithEventBus(),
		// foundation.WithOutbox(),
		// foundation.WithHTTPClient(),
		// foundation.WithAuth(),

		fx.Provide(NewOrderService, NewOrderHandler),
		fx.Invoke(registerRoutes),
	).Run()
}
```

Run the bundled example:

```bash
cd examples/minimal-service
go run .

curl localhost:8080/healthz
curl localhost:8080/api/v1/ping
```

### Scaffold a new service

```bash
go install github.com/aupv9/common-go/cmd/common-go@latest

common-go new order-service --module github.com/acme/order-service --postgres --cache
cd order-service && go mod tidy && go run .
```

See [`cmd/common-go`](./cmd/common-go) for all flags.

## Configuration

Configuration is read from built-in defaults, then a YAML file (`config.yaml`, or the path in `APP_CONFIG_FILE`),
then environment variables. Variables use the `APP_` prefix and `__` for nesting:

```bash
APP_SERVER__PORT=9090      # server.port
APP_LOG__LEVEL=debug       # log.level
APP_OTEL__ENABLED=true     # turn on OpenTelemetry export
```

Invalid configuration fails at startup. See [`examples/minimal-service/config.yaml`](./examples/minimal-service/config.yaml)
for a starting point and [`config/options.go`](./config/options.go) for every section.

## Packages

| Package | Purpose |
|---|---|
| [`foundation`](./foundation) | `Core()` and the `With*()` module bundles |
| [`config`](./config) | Layered config loading and validation |
| [`logger`](./logger) | `slog` setup, context fields, dynamic level |
| [`observability`](./observability) | OpenTelemetry tracer, meter and exporters |
| [`health`](./health) | Health and readiness endpoints, checker registry |
| [`errors`](./errors) | `AppError` model shared by HTTP and gRPC |
| [`response`](./response) | Response envelope |
| [`transport/httpserver`](./transport/httpserver) | gin server and middleware |
| [`transport/grpcserver`](./transport/grpcserver) | gRPC server and interceptors |
| [`transport/eventbus`](./transport/eventbus) | Event bus abstraction, Kafka driver, outbox |
| [`database`](./database) | pgx pool and migrations |
| [`cache`](./cache) | Redis client, lock, rate limiter |
| [`auth`](./auth) | JWT and JWKS verification |
| [`client/httpclient`](./client/httpclient) | Resilient `*http.Client` |
| [`tlsconfig`](./tlsconfig) | TLS and mTLS helpers |

Every package also works on its own through a plain `New(...)` constructor and functional options, without fx.
Design notes and decisions are in [DESIGN.md](./DESIGN.md).

## Development

```bash
go build ./...
go vet ./...
go test -race ./...
golangci-lint run ./...
```

Integration tests need real infrastructure and skip themselves unless it is configured:

```bash
TEST_POSTGRES_DSN="postgres://app:app@localhost:5432/app?sslmode=disable" go test -tags integration ./database/...
TEST_REDIS_ADDR="localhost:6379" go test -tags integration ./cache/...
```

## Contributing

Issues and pull requests are welcome. Please keep `gofmt` and `golangci-lint` clean, add tests for new behaviour,
and record user-facing changes in [CHANGELOG.md](./CHANGELOG.md).

## License

[MIT](./LICENSE)
