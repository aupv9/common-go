// Package errors định nghĩa error model dùng chung cho mọi service: AppError.
//
// Một AppError mang đủ thông tin để map sang cả HTTP (response envelope) lẫn
// gRPC (status) mà không phụ thuộc transport. Đây là package "lá": chỉ dùng
// stdlib, không import package khác trong common-go.
//
// Dùng errors.Is / errors.As / Unwrap như bình thường:
//
//	if errors.Is(err, someSentinel) { ... }
//	var ae *errors.AppError
//	if errors.As(err, &ae) { httpStatus := ae.HTTPStatus() }
package errors

import (
	stderrors "errors"
	"fmt"
	"net/http"
)

// Kind phân loại lỗi ở mức ngữ nghĩa, độc lập transport. Mỗi Kind có mặc định
// HTTP status tương ứng (xem httpStatusByKind).
type Kind string

const (
	KindInvalid      Kind = "invalid"      // dữ liệu vào không hợp lệ
	KindNotFound     Kind = "not_found"    // không tìm thấy tài nguyên
	KindUnauthorized Kind = "unauthorized" // chưa xác thực
	KindForbidden    Kind = "forbidden"    // không đủ quyền
	KindConflict     Kind = "conflict"     // xung đột trạng thái
	KindUnavailable  Kind = "unavailable"  // phụ thuộc tạm thời không sẵn sàng
	KindInternal     Kind = "internal"     // lỗi nội bộ ngoài dự kiến
)

// AppError là lỗi domain-facing chuẩn của hệ thống.
//
// Code là mã ổn định, máy đọc được (vd "ORDER_NOT_FOUND"), dùng cho client
// và logging. Message là thông điệp người đọc. Cause giữ lỗi gốc để wrap.
type AppError struct {
	Kind     Kind           // phân loại ngữ nghĩa
	Code     string         // mã ổn định, vd "ORDER_NOT_FOUND"
	Message  string         // human-readable
	Cause    error          // lỗi gốc (nếu có); hỗ trợ Unwrap
	Metadata map[string]any // thông tin phụ (an toàn để trả ra ngoài)

	// httpStatus cho phép override mặc định theo Kind (0 = dùng mặc định).
	httpStatus int
}

// Error implement error.
func (e *AppError) Error() string {
	if e == nil {
		return "<nil>"
	}
	if e.Cause != nil {
		return fmt.Sprintf("%s: %s: %v", e.Code, e.Message, e.Cause)
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

// Unwrap để errors.Is/As đi xuống Cause.
func (e *AppError) Unwrap() error { return e.Cause }

// Is so khớp theo Kind và Code, cho phép errors.Is(err, &AppError{Kind: ...}).
// Trường rỗng ở target được coi là wildcard.
func (e *AppError) Is(target error) bool {
	var t *AppError
	if !stderrors.As(target, &t) {
		return false
	}
	if t.Kind != "" && t.Kind != e.Kind {
		return false
	}
	if t.Code != "" && t.Code != e.Code {
		return false
	}
	return true
}

// HTTPStatus trả HTTP status: ưu tiên override, nếu không dùng mặc định theo Kind.
func (e *AppError) HTTPStatus() int {
	if e.httpStatus != 0 {
		return e.httpStatus
	}
	if s, ok := httpStatusByKind[e.Kind]; ok {
		return s
	}
	return http.StatusInternalServerError
}

// WithMetadata gắn thêm key/value, trả về chính nó để tiện chain.
func (e *AppError) WithMetadata(key string, value any) *AppError {
	if e.Metadata == nil {
		e.Metadata = make(map[string]any)
	}
	e.Metadata[key] = value
	return e
}

// WithHTTPStatus override HTTP status mặc định của Kind.
func (e *AppError) WithHTTPStatus(status int) *AppError {
	e.httpStatus = status
	return e
}

var httpStatusByKind = map[Kind]int{
	KindInvalid:      http.StatusBadRequest,
	KindNotFound:     http.StatusNotFound,
	KindUnauthorized: http.StatusUnauthorized,
	KindForbidden:    http.StatusForbidden,
	KindConflict:     http.StatusConflict,
	KindUnavailable:  http.StatusServiceUnavailable,
	KindInternal:     http.StatusInternalServerError,
}

// As là tiện ích lấy *AppError ra khỏi chuỗi lỗi; trả (nil, false) nếu không có.
func As(err error) (*AppError, bool) {
	var ae *AppError
	if stderrors.As(err, &ae) {
		return ae, true
	}
	return nil, false
}

// From bọc một error bất kỳ thành *AppError. Nếu err đã là AppError thì giữ
// nguyên; ngược lại coi là lỗi nội bộ. Dùng ở ranh giới transport để luôn có
// một AppError để render.
func From(err error) *AppError {
	if err == nil {
		return nil
	}
	if ae, ok := As(err); ok {
		return ae
	}
	return &AppError{
		Kind:    KindInternal,
		Code:    "INTERNAL",
		Message: "internal server error",
		Cause:   err,
	}
}
