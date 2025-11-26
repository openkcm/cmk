package errs_test

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"

	keystoreErrs "github.com/openkcm/plugin-sdk/pkg/plugin/keystore/errors"

	"github.com/openkcm/cmk/internal/errs"
)

var (
	ErrTestX = errors.New("test error x")
	ErrTestY = errors.New("test error y")
)

func TestGRPCError_Error(t *testing.T) {
	e := errs.GRPCError{
		Code:        "CODE",
		BaseMessage: "base",
		Reason:      "reason",
	}
	assert.Equal(t, "base: reason", e.Error())
}

func TestGRPCError_Is(t *testing.T) {
	e := errs.GRPCError{Code: "CODE"}
	target := errs.GRPCError{Code: "CODE"}
	assert.True(t, e.Is(target))
	assert.False(t, e.Is(nil))
}

func TestGRPCError_As(t *testing.T) {
	e := errs.GRPCError{Code: "CODE"}

	var target *errs.GRPCError

	assert.True(t, e.As(target))
	assert.False(t, e.As(nil))
}

func TestGRPCError_FromStatusError(t *testing.T) {
	meta := map[string]string{"foo": "bar"}
	grpcErr := keystoreErrs.NewGrpcErrorWithDetails(
		keystoreErrs.StatusInvalidKeyAccessData,
		"REASON",
		meta,
	)
	e := errs.GRPCError{BaseMessage: "msg"}
	result := e.FromStatusError(grpcErr)
	assert.Equal(t, "REASON", result.Reason)
	assert.Equal(t, meta, result.Metadata)
}

func TestGetGRPCErrorContext_ReturnsContext(t *testing.T) {
	meta := map[string]string{"foo": "bar"}
	e := errs.GRPCError{Reason: "r", Metadata: meta}
	joined := errors.Join(ErrTestX, e, ErrTestY)

	ctx := errs.GetGRPCErrorContext(joined)
	assert.NotNil(t, ctx)
	assert.Equal(t, "r", ctx["reason"])
	assert.Equal(t, "bar", ctx["foo"])
}

func TestGetGRPCErrorContext_NoJoinedError(t *testing.T) {
	assert.Nil(t, errs.GetGRPCErrorContext(ErrTestX))
}

func TestGetGRPCErrorContext_JoinedErrorTooShort(t *testing.T) {
	joined := errors.Join(ErrTestX)
	assert.Nil(t, errs.GetGRPCErrorContext(joined))
}

func TestGetGRPCErrorContext_NoGRPCErrorInChain(t *testing.T) {
	joined := errors.Join(ErrTestX, ErrTestY)
	assert.Nil(t, errs.GetGRPCErrorContext(joined))
}
