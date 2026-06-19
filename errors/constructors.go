package errors

import "fmt"

// Các constructor tạo *AppError theo từng Kind. code là mã ổn định
// (vd "ORDER_NOT_FOUND"), msg là thông điệp người đọc.
//
// Mỗi Kind có hai biến thể:
//   - <Kind>(code, msg)         : lỗi mới
//   - <Kind>f(code, format, ...) : msg dạng fmt
//
// Để wrap lỗi gốc, dùng Wrap(...).

// New tạo AppError với Kind tuỳ ý.
func New(kind Kind, code, msg string) *AppError {
	return &AppError{Kind: kind, Code: code, Message: msg}
}

// Wrap bọc cause với Kind/Code/Message cho trước.
func Wrap(cause error, kind Kind, code, msg string) *AppError {
	return &AppError{Kind: kind, Code: code, Message: msg, Cause: cause}
}

// Invalid: dữ liệu vào không hợp lệ (HTTP 400).
func Invalid(code, msg string) *AppError { return New(KindInvalid, code, msg) }

// Invalidf như Invalid nhưng format message.
func Invalidf(code, format string, a ...any) *AppError {
	return New(KindInvalid, code, fmt.Sprintf(format, a...))
}

// NotFound: không tìm thấy tài nguyên (HTTP 404).
func NotFound(code, msg string) *AppError { return New(KindNotFound, code, msg) }

// NotFoundf như NotFound nhưng format message.
func NotFoundf(code, format string, a ...any) *AppError {
	return New(KindNotFound, code, fmt.Sprintf(format, a...))
}

// Unauthorized: chưa xác thực (HTTP 401).
func Unauthorized(code, msg string) *AppError { return New(KindUnauthorized, code, msg) }

// Forbidden: không đủ quyền (HTTP 403).
func Forbidden(code, msg string) *AppError { return New(KindForbidden, code, msg) }

// Conflict: xung đột trạng thái (HTTP 409).
func Conflict(code, msg string) *AppError { return New(KindConflict, code, msg) }

// Unavailable: phụ thuộc tạm thời không sẵn sàng (HTTP 503).
func Unavailable(code, msg string) *AppError { return New(KindUnavailable, code, msg) }

// Internal: lỗi nội bộ ngoài dự kiến (HTTP 500).
func Internal(code, msg string) *AppError { return New(KindInternal, code, msg) }

// Internalf như Internal nhưng format message.
func Internalf(code, format string, a ...any) *AppError {
	return New(KindInternal, code, fmt.Sprintf(format, a...))
}
