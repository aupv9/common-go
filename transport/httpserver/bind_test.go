package httpserver_test

import (
	"net/http/httptest"
	"strings"
	"testing"

	apperrors "github.com/aupv9/common-go/errors"
	"github.com/aupv9/common-go/transport/httpserver"
	"github.com/gin-gonic/gin"
)

type createOrder struct {
	Item string `json:"item" validate:"required"`
	Qty  int    `json:"qty" validate:"gt=0"`
}

func bindCtx(body string) *gin.Context {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	req := httptest.NewRequest("POST", "/", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	c.Request = req
	return c
}

func TestBindJSONValid(t *testing.T) {
	got, err := httpserver.BindJSON[createOrder](bindCtx(`{"item":"widget","qty":3}`))
	if err != nil {
		t.Fatalf("BindJSON: %v", err)
	}
	if got.Item != "widget" || got.Qty != 3 {
		t.Errorf("got %+v", got)
	}
}

func TestBindJSONInvalidBody(t *testing.T) {
	_, err := httpserver.BindJSON[createOrder](bindCtx(`{not json`))
	ae, ok := apperrors.As(err)
	if !ok || ae.Code != "INVALID_BODY" {
		t.Fatalf("err = %v, want AppError INVALID_BODY", err)
	}
}

func TestBindJSONValidationFails(t *testing.T) {
	_, err := httpserver.BindJSON[createOrder](bindCtx(`{"item":"","qty":0}`))
	ae, ok := apperrors.As(err)
	if !ok || ae.Code != "VALIDATION_FAILED" {
		t.Fatalf("err = %v, want AppError VALIDATION_FAILED", err)
	}
	if ae.Kind != apperrors.KindInvalid {
		t.Errorf("kind = %v, want invalid", ae.Kind)
	}
	if ae.Metadata["fields"] == nil {
		t.Error("expected field errors in metadata")
	}
}
