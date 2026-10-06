package apierrors_test

import (
	"context"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/openkcm/cmk/internal/apierrors"
	"github.com/openkcm/cmk/internal/errs"
	"github.com/openkcm/cmk/internal/manager"
	"github.com/openkcm/cmk/internal/pluginregistry/service/api/keymanagement"
)

func TestAPIErrorMapper_ImportKeyMaterialDecryptionFailed(t *testing.T) {
	err := errs.Wrap(manager.ErrImportKeyMaterialsToProvider, keymanagement.ErrImportKeyMaterialFailed)

	result := apierrors.APIErrorMapper.Transform(context.Background(), err)

	assert.Equal(t, "INVALID_WRAPPED_KEY_MATERIAL", result.Code)
	assert.Equal(t, "Key material decryption failed: invalid or incorrectly wrapped key material.", result.Message)
	assert.Equal(t, http.StatusBadRequest, result.Status)
}

func TestAPIErrorMapper_ImportKeyMaterialDecryptionFailed_NotMatchedWithoutBothErrors(t *testing.T) {
	err := errs.Wrapf(manager.ErrImportKeyMaterialsToProvider, "some provider error")

	result := apierrors.APIErrorMapper.Transform(context.Background(), err)

	assert.NotEqual(t, "INVALID_WRAPPED_KEY_MATERIAL", result.Code)
}
