package grpcserver

import (
	apperrors "github.com/aupv9/common-go/errors"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// codeByKind map errors.Kind sang gRPC code. Mapping đặt ở đây (không trong
// package errors) để giữ errors là package lá chỉ phụ thuộc stdlib.
var codeByKind = map[apperrors.Kind]codes.Code{
	apperrors.KindInvalid:      codes.InvalidArgument,
	apperrors.KindNotFound:     codes.NotFound,
	apperrors.KindUnauthorized: codes.Unauthenticated,
	apperrors.KindForbidden:    codes.PermissionDenied,
	apperrors.KindConflict:     codes.Aborted,
	apperrors.KindUnavailable:  codes.Unavailable,
	apperrors.KindInternal:     codes.Internal,
}

// toStatus chuyển một error bất kỳ sang *status.Status. AppError được map theo
// Kind và mang Code ổn định trong message; lỗi khác coi là Internal. Trả nil
// nếu err nil.
func toStatus(err error) *status.Status {
	if err == nil {
		return nil
	}
	// Nếu đã là status (vd từ tầng dưới) thì giữ nguyên.
	if s, ok := status.FromError(err); ok && s.Code() != codes.Unknown {
		return s
	}

	ae := apperrors.From(err)
	code, ok := codeByKind[ae.Kind]
	if !ok {
		code = codes.Internal
	}
	return status.New(code, ae.Message)
}

// codeFor trả gRPC code tương ứng error (dùng để log).
func codeFor(err error) codes.Code {
	if err == nil {
		return codes.OK
	}
	if s := toStatus(err); s != nil {
		return s.Code()
	}
	return codes.Unknown
}
