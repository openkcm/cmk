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
	assert.Equal(t, "Key material decryption failed: invalid or incorrectly wrapped key material, or the key material is wrapped with expired import parameters.", result.Message)
	assert.Equal(t, http.StatusBadRequest, result.Status)
}

func TestAPIErrorMapper_ImportKeyMaterialDecryptionFailed_NotMatchedWithoutBothErrors(t *testing.T) {
	err := errs.Wrapf(manager.ErrImportKeyMaterialsToProvider, "some provider error")

	result := apierrors.APIErrorMapper.Transform(context.Background(), err)

	assert.NotEqual(t, "INVALID_WRAPPED_KEY_MATERIAL", result.Code)
}

func TestAPIErrorMapper_KeyStateNotDeletable(t *testing.T) {
	managerErr := errs.Wrap(manager.ErrDeleteKey, manager.ErrKeyStateNotDeletable)
	err := errs.Wrap(apierrors.ErrDeleteKey, managerErr)

	result := apierrors.APIErrorMapper.Transform(context.Background(), err)

	assert.Equal(t, "INVALID_KEY_DELETE", result.Code)
	assert.Equal(t, http.StatusBadRequest, result.Status)
	assert.Contains(t, result.Message, "current state")
}
