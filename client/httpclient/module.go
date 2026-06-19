package httpclient

import (
	"net/http"

	"go.uber.org/fx"
)

// Module cung cấp một *http.Client mặc định cho graph fx. Service cần nhiều
// client với cấu hình khác nhau nên tự fx.Provide (dùng New trực tiếp) hoặc
// dùng named provider thay vì dựa vào client chung này.
//
// opts áp dụng lên client mặc định.
func Module(opts ...Option) fx.Option {
	return fx.Module("httpclient",
		fx.Provide(func() *http.Client {
			return New(opts...)
		}),
	)
}
