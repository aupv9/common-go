// minimal-service là service mẫu chạy được, minh hoạ cách dùng common-go.
//
// Chạy:
//
//	go run .
//	curl localhost:8080/healthz
//	curl localhost:8080/readyz
//	curl localhost:8080/info
//	curl localhost:8080/api/v1/ping
//
// Toàn bộ wiring nằm trong main(): foundation.Core() cấp config/logger/health,
// foundation.WithHTTP() cấp gin engine + middleware chuẩn + health endpoint +
// graceful shutdown. Service chỉ còn việc khai báo handler và gắn route.
package main

import (
	"github.com/aupv9/common-go/examples/minimal-service/internal"
	"github.com/aupv9/common-go/examples/minimal-service/internal/handler"
	"github.com/aupv9/common-go/foundation"
	"go.uber.org/fx"
)

func main() {
	fx.New(
		foundation.Core(),     // config + logger + health
		foundation.WithHTTP(), // gin engine + middleware chuẩn + health endpoint + server

		fx.Provide(handler.New),
		fx.Invoke(internal.Register), // gắn route nghiệp vụ
	).Run()
}
