package grpcserver

import (
	stderrors "errors"
	"testing"

	apperrors "github.com/aupv9/common-go/errors"
	"google.golang.org/grpc/codes"
)

func TestToStatusByKind(t *testing.T) {
	cases := []struct {
		err  error
		want codes.Code
	}{
		{apperrors.Invalid("C", "m"), codes.InvalidArgument},
		{apperrors.NotFound("C", "m"), codes.NotFound},
		{apperrors.Unauthorized("C", "m"), codes.Unauthenticated},
		{apperrors.Forbidden("C", "m"), codes.PermissionDenied},
		{apperrors.Conflict("C", "m"), codes.Aborted},
		{apperrors.Unavailable("C", "m"), codes.Unavailable},
		{apperrors.Internal("C", "m"), codes.Internal},
		{stderrors.New("plain"), codes.Internal}, // non-AppError → Internal
	}
	for _, tc := range cases {
		if got := toStatus(tc.err).Code(); got != tc.want {
			t.Errorf("toStatus(%v).Code() = %v, want %v", tc.err, got, tc.want)
		}
	}
}

func TestToStatusNil(t *testing.T) {
	if toStatus(nil) != nil {
		t.Error("toStatus(nil) should be nil")
	}
	if codeFor(nil) != codes.OK {
		t.Error("codeFor(nil) should be OK")
	}
}
