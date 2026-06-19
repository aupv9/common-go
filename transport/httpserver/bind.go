package httpserver

import (
	"errors"

	apperrors "github.com/aupv9/common-go/errors"
	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
)

// validate là validator dùng chung (thread-safe, cache reflection).
var validate = validator.New()

// BindJSON parse body JSON vào *T rồi validate theo tag `validate`. Lỗi parse
// hoặc validate được trả về dưới dạng AppError{Invalid} với metadata field→lý do
// — handler chỉ cần response.Error(w, r, err).
//
//	type CreateOrder struct {
//	    Item string `json:"item" validate:"required"`
//	    Qty  int    `json:"qty" validate:"gt=0"`
//	}
//	req, err := httpserver.BindJSON[CreateOrder](c)
//	if err != nil { response.Error(c.Writer, c.Request, err); return }
func BindJSON[T any](c *gin.Context) (*T, error) {
	var out T
	if err := c.ShouldBindJSON(&out); err != nil {
		return nil, apperrors.Invalid("INVALID_BODY", "invalid request body").
			WithMetadata("reason", err.Error())
	}
	if err := validate.Struct(&out); err != nil {
		return nil, validationError(err)
	}
	return &out, nil
}

// Validate chạy validate.Struct trên một giá trị đã có (vd bind từ query/uri).
func Validate(v any) error {
	if err := validate.Struct(v); err != nil {
		return validationError(err)
	}
	return nil
}

// validationError chuyển lỗi validator thành AppError{Invalid} kèm map field→tag.
func validationError(err error) error {
	var ve validator.ValidationErrors
	if !errors.As(err, &ve) {
		return apperrors.Invalid("VALIDATION_FAILED", "validation failed").
			WithMetadata("reason", err.Error())
	}
	fields := make(map[string]any, len(ve))
	for _, fe := range ve {
		fields[fe.Field()] = fe.Tag()
	}
	return apperrors.Invalid("VALIDATION_FAILED", "validation failed").
		WithMetadata("fields", fields)
}
