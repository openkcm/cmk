package apierrors

import (
	"errors"
	"net/http"

	"github.com/openkcm/cmk/internal/errs"
)

var ErrTransformSystemGroupsToAPI = errors.New("failed to transform system group to API")

var systemgroup = []errs.ExposedErrors[*APIError]{
	{
		InternalErrorChain: []error{ErrTransformSystemGroupsToAPI},
		ExposedError: &APIError{
			Code:    "TRANSFORM_SYSTEM_GROUPS_LIST",
			Message: "failed to transform system groups list",
			Status:  http.StatusInternalServerError,
		},
	},
}
