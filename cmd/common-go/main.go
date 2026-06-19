// Command common-go là CLI tiện ích của thư viện common-go.
//
// Cài đặt:
//
//	go install github.com/aupv9/common-go/cmd/common-go@latest
//
// Dùng:
//
//	common-go new my-service          # scaffold một service chạy được
//	common-go new my-service --postgres --cache
//	common-go version
//
// CLI chỉ dùng stdlib nên không thêm dependency vào module common-go.
package main

import (
	"fmt"
	"os"
)

// version được set lúc build qua -ldflags "-X main.version=vX.Y.Z"; mặc định "dev".
var version = "dev"

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}

	switch os.Args[1] {
	case "new":
		if err := runNew(os.Args[2:]); err != nil {
			fmt.Fprintln(os.Stderr, "error:", err)
			os.Exit(1)
		}
	case "version", "--version", "-v":
		fmt.Println("common-go", version)
	case "help", "--help", "-h":
		usage()
	default:
		fmt.Fprintf(os.Stderr, "unknown command %q\n\n", os.Args[1])
		usage()
		os.Exit(2)
	}
}

func usage() {
	fmt.Fprint(os.Stderr, `common-go — CLI cho foundation library common-go

Usage:
  common-go <command> [arguments]

Commands:
  new <name>    Scaffold một Go service mới dùng common-go
  version       In phiên bản CLI
  help          In trợ giúp này

Chạy "common-go new -h" để xem cờ của lệnh new.
`)
}
