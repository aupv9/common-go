package config

import (
	"fmt"
	"os"
	"strings"
)

// ResolveSecret giải tham chiếu secret theo quy ước Docker/K8s: nếu giá trị có
// dạng "file:/đường/dẫn" thì đọc nội dung file (cắt khoảng trắng cuối) — tiện
// nạp secret từ file mount thay vì để plaintext trong env/config. Giá trị thường
// được trả nguyên.
//
//	APP_DATABASE__PASSWORD=file:/run/secrets/db_password
func ResolveSecret(v string) (string, error) {
	const prefix = "file:"
	if !strings.HasPrefix(v, prefix) {
		return v, nil
	}
	path := strings.TrimPrefix(v, prefix)
	b, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("config: read secret file %q: %w", path, err)
	}
	return strings.TrimRight(string(b), "\r\n"), nil
}

// ResolveSecretOr như ResolveSecret nhưng trả nguyên giá trị nếu đọc file lỗi
// (tiện cho mapping module; dùng ResolveSecret khi cần bắt lỗi tường minh).
func ResolveSecretOr(v string) string {
	out, err := ResolveSecret(v)
	if err != nil {
		return v
	}
	return out
}

const redacted = "***"
