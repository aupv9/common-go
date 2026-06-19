// Package response định nghĩa response envelope chuẩn cho REST API và các
// helper để ghi response (JSON) đồng nhất trên mọi service.
//
// Mọi response có dạng:
//
//	{
//	  "data":  <payload> | null,
//	  "error": <ErrorBody> | null,
//	  "meta":  { "request_id": "...", "trace_id": "...", "pagination": {...} }
//	}
//
// Helper Error tự map errors.AppError → HTTP status + ErrorBody.
package response

import (
	"encoding/json"
	"net/http"
	"sync/atomic"

	apperrors "github.com/aupv9/common-go/errors"
)

// Envelope là khung response thống nhất.
type Envelope struct {
	Data  any        `json:"data"`
	Error *ErrorBody `json:"error"`
	Meta  *Meta      `json:"meta,omitempty"`
}

// ErrorBody là phần lỗi trả ra client (an toàn, không lộ chi tiết nội bộ).
type ErrorBody struct {
	Code     string         `json:"code"`
	Message  string         `json:"message"`
	Metadata map[string]any `json:"metadata,omitempty"`
}

// Meta chứa thông tin phụ trợ của response.
type Meta struct {
	RequestID  string      `json:"request_id,omitempty"`
	TraceID    string      `json:"trace_id,omitempty"`
	Pagination *Pagination `json:"pagination,omitempty"`
}

// Pagination cho list endpoint.
type Pagination struct {
	Page       int   `json:"page"`
	PageSize   int   `json:"page_size"`
	TotalItems int64 `json:"total_items"`
	TotalPages int   `json:"total_pages"`
}

// MetaFunc lấy meta (request_id, trace_id) từ request. Mặc định nil → không có
// meta; transport layer (M2) sẽ cấp một implementation đọc từ context.
type MetaFunc func(r *http.Request) *Meta

// metaFrom là hook toàn cục để các helper tự gắn meta. Lưu qua atomic.Pointer
// nên đọc/ghi an toàn cho goroutine (transport set lúc khởi động, handler đọc
// đồng thời khi phục vụ request).
var metaFrom atomic.Pointer[MetaFunc]

// SetMetaFunc đăng ký hàm trích meta từ request. Thường gọi lúc khởi động.
func SetMetaFunc(f MetaFunc) { metaFrom.Store(&f) }

// OK ghi 200 với payload.
func OK(w http.ResponseWriter, r *http.Request, data any) {
	write(w, r, http.StatusOK, &Envelope{Data: data})
}

// Created ghi 201 với payload.
func Created(w http.ResponseWriter, r *http.Request, data any) {
	write(w, r, http.StatusCreated, &Envelope{Data: data})
}

// NoContent ghi 204 (không body).
func NoContent(w http.ResponseWriter) {
	w.WriteHeader(http.StatusNoContent)
}

// List ghi 200 với payload kèm pagination.
func List(w http.ResponseWriter, r *http.Request, data any, p *Pagination) {
	env := &Envelope{Data: data}
	if m := metaForRequest(r); m != nil {
		m.Pagination = p
		env.Meta = m
	} else if p != nil {
		env.Meta = &Meta{Pagination: p}
	}
	writeEnvelope(w, http.StatusOK, env)
}

// Error map err → AppError → HTTP status + ErrorBody và ghi ra.
// Lỗi không phải AppError được coi là internal (500) — chi tiết gốc KHÔNG lộ
// ra client (caller nên log err đầy đủ trước khi gọi).
func Error(w http.ResponseWriter, r *http.Request, err error) {
	ae := apperrors.From(err)
	body := &ErrorBody{Code: ae.Code, Message: ae.Message, Metadata: ae.Metadata}
	write(w, r, ae.HTTPStatus(), &Envelope{Error: body})
}

func write(w http.ResponseWriter, r *http.Request, status int, env *Envelope) {
	if env.Meta == nil {
		env.Meta = metaForRequest(r)
	}
	writeEnvelope(w, status, env)
}

func writeEnvelope(w http.ResponseWriter, status int, env *Envelope) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	// Lỗi encode (vd client ngắt kết nối) không thể sửa sau khi đã WriteHeader;
	// bỏ qua một cách có chủ đích.
	_ = json.NewEncoder(w).Encode(env)
}

func metaForRequest(r *http.Request) *Meta {
	if r == nil {
		return nil
	}
	p := metaFrom.Load()
	if p == nil || *p == nil {
		return nil
	}
	return (*p)(r)
}
