# common-go CLI

CLI tiện ích của [common-go](../../). Hiện hỗ trợ scaffold một service Go chạy được.

## Cài đặt

```bash
go install github.com/aupv9/common-go/cmd/common-go@latest
```

(hoặc chạy thẳng từ source: `go run ./cmd/common-go ...`)

## Lệnh

### `new` — scaffold service mới

```bash
common-go new <name> [flags]
```

| Flag | Mặc định | Ý nghĩa |
|---|---|---|
| `--module` | `<name>` | Go module path của service mới (vd `github.com/acme/order-service`) |
| `--dir` | `./<name>` | thư mục output |
| `--postgres` | `false` | thêm `foundation.WithPostgres()` + section `database` |
| `--cache` | `false` | thêm `foundation.WithCache()` + section `cache` |
| `--grpc` | `false` | thêm `foundation.WithGRPC()` + section `grpc` |
| `--eventbus` | `false` | thêm `foundation.WithEventBus()` + section `eventbus` |
| `--force` | `false` | ghi đè nếu thư mục đã tồn tại |

Tên service và flag có thể viết theo thứ tự nào cũng được:

```bash
common-go new order-service --postgres --cache
common-go new order-service --module github.com/acme/order-service --postgres
```

Service sinh ra gồm `main.go` (wiring fx), `config.yaml`, `internal/routes.go`,
`internal/handler/`, `go.mod`, `README.md`, `.gitignore`. File `.go` luôn được
format bằng `go/format` (gofmt-sạch).

Sau khi tạo:

```bash
cd <name>
go get github.com/aupv9/common-go@latest
go mod tidy
go run .
```

### `version`

```bash
common-go version
```

## Ghi chú

- CLI **chỉ dùng stdlib** (`flag`, `embed`, `text/template`, `go/format`) → không thêm dependency vào module common-go.
- Template nằm trong [`templates/`](./templates), nhúng bằng `//go:embed`.
