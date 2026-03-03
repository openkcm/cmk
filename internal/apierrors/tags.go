package apierrors

import (
	"errors"
	"net/http"

	"github.com/openkcm/cmk/internal/errs"
)

var (
	ErrCreatingTags                    = errors.New("failed to create tags")
	ErrGettingTagsByKeyConfigurationID = errors.New("failed to get tags by KeyConfigurationID")
)

var tags = []errs.ExposedErrors[*APIError]{
	{
		InternalErrorChain: []error{ErrCreatingTags},
		ExposedError: &APIError{
			Code:    "CREATE_TAGS",
			Message: "Failed to create tags",
			Status:  http.StatusInternalServerError,
		},
	},
	{
		InternalErrorChain: []error{ErrGettingTagsByKeyConfigurationID},
		ExposedError: &APIError{
			Code:    "GET_TAGS_BY_KEY_CONFIGURATION_ID",
			Message: "Failed to get tags by KeyConfigurationID",
			Status:  http.StatusInternalServerError,
		},
	},
}
