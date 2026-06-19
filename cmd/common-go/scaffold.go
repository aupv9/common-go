package main

import (
	"bytes"
	"embed"
	"flag"
	"fmt"
	"go/format"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"text/template"
)

// commonModule là module path của chính thư viện này; service được scaffold sẽ import nó.
const commonModule = "github.com/aupv9/common-go"

//go:embed all:templates
var templatesFS embed.FS

// serviceData là dữ liệu truyền vào template khi render.
type serviceData struct {
	Name         string // tên service (vd "order-service")
	Module       string // go module path của service mới
	CommonModule string // module path của common-go
	HTTP         bool
	Postgres     bool
	Cache        bool
	GRPC         bool
	EventBus     bool
}

// runNew xử lý lệnh `common-go new <name> [flags]`.
func runNew(args []string) error {
	fs := flag.NewFlagSet("new", flag.ExitOnError)
	var (
		module   = fs.String("module", "", "go module path của service (mặc định = tên service)")
		dir      = fs.String("dir", "", "thư mục output (mặc định = ./<name>)")
		postgres = fs.Bool("postgres", false, "thêm Postgres (foundation.WithPostgres)")
		cache    = fs.Bool("cache", false, "thêm Redis cache (foundation.WithCache)")
		grpc     = fs.Bool("grpc", false, "thêm gRPC server (foundation.WithGRPC)")
		eventbus = fs.Bool("eventbus", false, "thêm Kafka event bus (foundation.WithEventBus)")
		force    = fs.Bool("force", false, "ghi đè nếu thư mục đã tồn tại")
	)
	fs.Usage = func() {
		fmt.Fprint(os.Stderr, `Usage: common-go new <name> [flags]

Tạo một service Go chạy được dùng common-go (foundation.Core + WithHTTP).

Flags:
`)
		fs.PrintDefaults()
		fmt.Fprint(os.Stderr, `
Ví dụ:
  common-go new order-service
  common-go new order-service --module github.com/acme/order-service --postgres --cache
`)
	}
	// Cho phép viết tên trước flag (vd `new order-service --postgres`): tách
	// positional dẫn đầu ra trước rồi mới Parse phần còn lại (flag.Parse dừng ở
	// non-flag đầu tiên).
	var name string
	if len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		name = args[0]
		args = args[1:]
	}
	if err := fs.Parse(args); err != nil {
		return err
	}
	if name == "" {
		name = fs.Arg(0)
	}

	if name == "" {
		fs.Usage()
		return fmt.Errorf("thiếu tên service")
	}
	if !validName(name) {
		return fmt.Errorf("tên service không hợp lệ %q (chỉ dùng chữ thường, số, dấu '-')", name)
	}

	data := serviceData{
		Name:         name,
		Module:       *module,
		CommonModule: commonModule,
		HTTP:         true,
		Postgres:     *postgres,
		Cache:        *cache,
		GRPC:         *grpc,
		EventBus:     *eventbus,
	}
	if data.Module == "" {
		data.Module = name
	}

	out := *dir
	if out == "" {
		out = name
	}
	if _, err := os.Stat(out); err == nil && !*force {
		return fmt.Errorf("thư mục %q đã tồn tại (dùng --force để ghi đè)", out)
	}

	if err := renderTemplates(out, data); err != nil {
		return err
	}

	printNextSteps(out, data)
	return nil
}

// renderTemplates duyệt cây templates/, render từng file và ghi vào out/ với
// cấu trúc thư mục tương ứng (bỏ đuôi ".tmpl").
func renderTemplates(out string, data serviceData) error {
	return fs.WalkDir(templatesFS, "templates", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}

		rel := strings.TrimPrefix(path, "templates/")
		rel = strings.TrimSuffix(rel, ".tmpl")
		dest := filepath.Join(out, rel)

		if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
			return err
		}

		raw, err := templatesFS.ReadFile(path)
		if err != nil {
			return err
		}
		tmpl, err := template.New(rel).Parse(string(raw))
		if err != nil {
			return fmt.Errorf("parse template %s: %w", path, err)
		}

		var buf bytes.Buffer
		if err := tmpl.Execute(&buf, data); err != nil {
			return fmt.Errorf("render %s: %w", dest, err)
		}

		out := buf.Bytes()
		// Đảm bảo file .go luôn gofmt-sạch bất kể whitespace trong template.
		if strings.HasSuffix(dest, ".go") {
			formatted, ferr := format.Source(out)
			if ferr != nil {
				return fmt.Errorf("gofmt %s: %w", dest, ferr)
			}
			out = formatted
		}

		if err := os.WriteFile(dest, out, 0o644); err != nil {
			return err
		}
		fmt.Println("  create", dest)
		return nil
	})
}

func printNextSteps(out string, data serviceData) {
	fmt.Printf("\n✓ Đã tạo service %q tại ./%s\n\nBước tiếp theo:\n", data.Name, out)
	fmt.Printf("  cd %s\n", out)
	fmt.Printf("  go get %s@latest\n", data.CommonModule)
	fmt.Println("  go mod tidy")
	fmt.Println("  go run .")
	fmt.Println("\nThử:")
	fmt.Println("  curl localhost:8080/healthz")
	fmt.Println("  curl localhost:8080/api/v1/ping")
}

// validName cho phép chữ thường, số và dấu '-' (không bắt đầu/kết thúc bằng '-').
func validName(s string) bool {
	if s == "" || s[0] == '-' || s[len(s)-1] == '-' {
		return false
	}
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z':
		case r >= '0' && r <= '9':
		case r == '-':
		default:
			return false
		}
	}
	return true
}
