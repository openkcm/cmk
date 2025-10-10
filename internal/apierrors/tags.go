package apierrors

import (
	"errors"
	"net/http"

	"github.com/openkcm/cmk/internal/api/cmkapi"
)

var (
	ErrCreatingTags                    = errors.New("failed to create tags")
	ErrGettingTagsByKeyConfigurationID = errors.New("failed to get tags by KeyConfigurationID")
)

var tags = []APIErrors{
	{
		Errors: []error{ErrCreatingTags},
		ExposedError: cmkapi.DetailedError{
			Code:    "CREATE_TAGS",
			Message: "Failed to create tags",
			Status:  http.StatusInternalServerError,
		},
	},
	{
		Errors: []error{ErrGettingTagsByKeyConfigurationID},
		ExposedError: cmkapi.DetailedError{
			Code:    "GET_TAGS_BY_KEY_CONFIGURATION_ID",
			Message: "Failed to get tags by KeyConfigurationID",
			Status:  http.StatusInternalServerError,
		},
	},
}
