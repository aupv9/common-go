# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What this is

`common-go` is an **open-source foundation library** (not a service) for Go microservices. It standardizes config, logging, observability, errors, transport (HTTP/gRPC/event), database, cache and auth. A new service composes capabilities via [uber/fx](https://uber-go.github.io/fx/) and runs production-ready with ~20 lines of `main.go`. A scaffolding CLI lives in [`cmd/common-go`](./cmd/common-go).

Module path: `github.com/aupv9/common-go` (public, MIT). It's a normal `go get`-able module — no `GOPRIVATE` needed.

Design rationale, decisions and roadmap live in [DESIGN.md](./DESIGN.md); per-milestone change log in [CHANGELOG.md](./CHANGELOG.md). Comments and docs in this repo are written in Vietnamese — match that when editing.

## Commands

`go` may not be on `PATH` in this environment; it is installed at `/opt/homebrew/bin`. Prefix commands with `export PATH=$PATH:/opt/homebrew/bin` if `go: command not found`.

```bash
go build ./...                                  # build everything
go vet ./...
gofmt -l .                                      # must print nothing (CI fails otherwise)
go test ./...                                    # all tests
go test ./auth/ -run TestVerifyValidToken -v     # single test
golangci-lint run ./...                          # full lint (config in .golangci.yml)
go mod tidy                                       # after adding/removing imports

cd examples/minimal-service && APP_SERVER__PORT=8080 APP_APP__NAME=demo go run .
# curl localhost:8080/healthz | /readyz | /info | /api/v1/ping | /admin/loglevel
```

Default tests are broker/DB-free — pure logic + in-process HTTP/gRPC via `httptest`/bufconn; run with `-race` (the codebase is race-clean). Tests needing real infra live behind `//go:build integration` and self-skip unless env is set:

```bash
TEST_POSTGRES_DSN="postgres://app:app@localhost:5432/app?sslmode=disable" go test -tags integration ./database/...
TEST_REDIS_ADDR="localhost:6379" go test -tags integration ./cache/...
```

## Architecture

### fx module composition
Every capability is an independent `fx.Module` exposed two ways:
- A package-level `var Module fx.Option` **or** `func Module(opts ...Option) fx.Option`.
- A plain `New(...)` constructor + functional `Option`s, so the same component works standalone (outside fx) too.

`foundation` is the only package that imports everything; it groups modules:
- `foundation.Core()` = config + logger + observability + health (always-on baseline).
- `foundation.WithHTTP/WithGRPC/WithPostgres/WithCache/WithEventBus/WithHTTPClient/WithAuth/WithOutbox()` = opt-in capabilities.

Servers split `Listen()` (synchronous bind) from `Serve()`; the fx `OnStart` binds synchronously so a port-in-use error fails startup instead of being swallowed in a goroutine. Mirror this when adding a new network server.

Lifecycle (server start/stop, pool/exporter close) is registered via `fx.Lifecycle` hooks inside each module — never with `init()` or global state. Config is loaded once into `*config.AppConfig` and injected; each section (`ServerSection`, `OtelSection`, `DatabaseSection`, …) maps to a module.

### Package dependency DAG (enforced by convention)
"Low" packages must not import "high" ones. `errors` and `response` are **leaves** (stdlib only). `config` → leaf; `logger` → config; `observability` → config+logger; transports → logger/observability/response/errors; `foundation` → everything. When adding cross-package behavior, respect this direction — several deliberate placements exist precisely to keep it acyclic (see below).

### Non-obvious cross-cutting wiring (read before changing)
- **OTel → logs without a cycle**: `logger` stays OTel-free. `observability` injects `trace_id`/`span_id` into every log record by calling `logger.RegisterContextExtractor(...)` at startup (append-only hook, like `response.SetMetaFunc`). Don't make `logger` import OTel.
- **`errors` → gRPC codes mapping lives in `transport/grpcserver`** (`errmap.go`), not in `errors`, so `errors` stays a stdlib-only leaf. HTTP status mapping *is* in `errors` (no extra dep). `AppError.Kind` is the semantic source of truth for both.
- **eventbus fx wiring lives in `transport/eventbus/driver/kafka`** (`kafka.Module`), not in `transport/eventbus`. The core `eventbus` package is driver-agnostic (no fx, no kafka); putting `Module` in core would create a core↔driver import cycle. `foundation.WithEventBus()` calls `kafka.Module`.
- **request_id / trace_id propagation**: HTTP `RequestID` middleware puts request_id in context (`logger.WithRequestID`); response envelope `meta` is populated by a global `response.SetMetaFunc` hook that `httpserver` installs at startup — it reads request_id from context and trace_id directly from the OTel span (httpserver may import OTel; logger may not).
- **observability is no-op when disabled** (`Otel.Enabled=false`, the default): propagator is still installed but no exporters/collector connection — local runs need no infra. Tracing for httpserver/grpcserver/httpclient/database/cache auto-enables from `Otel.Enabled`.

### Config precedence & env mapping
`config.Load[T]` precedence: struct defaults → `config.yaml` → env (env wins, 12-factor). Env prefix `APP_`, nested via `__`: `APP_SERVER__PORT=9090` → `server.port`. Validation (`go-playground/validator` tags) failing = startup failure (fail-fast). Services embed `config.AppConfig` with `koanf:",squash"` and add their own sections.

### Conventions
- Context is the first param, never stored in structs. `New(...)` returns `(T, error)`; functional options for config.
- Wrap errors with `fmt.Errorf("...: %w", err)`; use `errors.AppError` (via `errors.NotFound/Invalid/Internal/...`) for domain-facing errors.
- Doc-comment every exported symbol; tests are table-driven; one `*_test.go` per package using the `package x_test` external form where practical.
- Semver; this lib is a dependency of every service, so breaking changes need a migration note in CHANGELOG.
