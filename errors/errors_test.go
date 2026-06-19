package errors_test

import (
	stderrors "errors"
	"fmt"
	"net/http"
	"testing"

	apperrors "github.com/aupv9/common-go/errors"
)

func TestHTTPStatusByKind(t *testing.T) {
	tests := []struct {
		name string
		err  *apperrors.AppError
		want int
	}{
		{"invalid", apperrors.Invalid("BAD", "bad"), http.StatusBadRequest},
		{"not found", apperrors.NotFound("NF", "nf"), http.StatusNotFound},
		{"unauthorized", apperrors.Unauthorized("UA", "ua"), http.StatusUnauthorized},
		{"forbidden", apperrors.Forbidden("FB", "fb"), http.StatusForbidden},
		{"conflict", apperrors.Conflict("CF", "cf"), http.StatusConflict},
		{"unavailable", apperrors.Unavailable("UV", "uv"), http.StatusServiceUnavailable},
		{"internal", apperrors.Internal("IN", "in"), http.StatusInternalServerError},
		{"override", apperrors.Invalid("X", "x").WithHTTPStatus(http.StatusTeapot), http.StatusTeapot},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.err.HTTPStatus(); got != tc.want {
				t.Fatalf("HTTPStatus() = %d, want %d", got, tc.want)
			}
		})
	}
}

func TestUnwrapAndIs(t *testing.T) {
	cause := stderrors.New("db down")
	err := apperrors.Wrap(cause, apperrors.KindUnavailable, "DB_DOWN", "database unavailable")

	if !stderrors.Is(err, cause) {
		t.Fatal("errors.Is should reach wrapped cause")
	}

	// Is khớp theo Kind (Code rỗng = wildcard).
	if !stderrors.Is(err, &apperrors.AppError{Kind: apperrors.KindUnavailable}) {
		t.Fatal("errors.Is should match by Kind")
	}
	if stderrors.Is(err, &apperrors.AppError{Kind: apperrors.KindNotFound}) {
		t.Fatal("errors.Is should not match different Kind")
	}
}

func TestFrom(t *testing.T) {
	// AppError giữ nguyên.
	orig := apperrors.NotFound("NF", "nope")
	if got := apperrors.From(orig); got != orig {
		t.Fatal("From should return same AppError")
	}
	// Lỗi thường -> internal.
	wrapped := apperrors.From(fmt.Errorf("boom"))
	if wrapped.Kind != apperrors.KindInternal || wrapped.HTTPStatus() != http.StatusInternalServerError {
		t.Fatalf("From(plain) = %+v, want internal/500", wrapped)
	}
	// nil -> nil.
	if apperrors.From(nil) != nil {
		t.Fatal("From(nil) should be nil")
	}
}
