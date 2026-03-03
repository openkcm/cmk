package eventprocessor

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"github.com/openkcm/cmk/internal/constants"
	"github.com/openkcm/cmk/internal/errs"
)

type OrbitalError struct {
	Message string
	Code    string
}

func (e *OrbitalError) SetContext(context *map[string]any) {}

func (e *OrbitalError) DefaultError() *OrbitalError {
	return &OrbitalError{
		Message: constants.DefaultErrorMessage,
		Code:    constants.DefaultErrorCode,
	}
}

func (e *OrbitalError) IsDefaultError() bool {
	return e.Message == constants.DefaultErrorMessage &&
		e.Code == constants.DefaultErrorCode
}

func (e *OrbitalError) String() string {
	return fmt.Sprintf("%s:%s", e.Code, e.Message)
}

// ParseOrbitalError returns a parsed error in orbital format
// If there is no code or no entries to parse, it sets the code to the default value
// The parsing is done with an expression expect a ":"
// right after the error code in SCREAMING_SNAKE_CASE
func ParseOrbitalError(errorMessage string) OrbitalError {
	code, message, found := strings.Cut(errorMessage, ":")

	// Matches screaming snake case until a without any whitespaces
	// and only matches alphabetic characters and "_"
	reg := regexp.MustCompile("^[A-Z]+(?:_[A-Z]+)*$")

	// Couldn't find ":" or the error does not contain an orbital error like code
	if !found || !reg.MatchString(code) {
		return OrbitalError{Message: errorMessage, Code: constants.DefaultErrorCode}
	}

	return OrbitalError{Message: message, Code: code}
}

// GetOrbitalError returns the string format of an orbital error
// If the error is not mapped to an orbital error return it as a string
func GetOrbitalError(ctx context.Context, err error) string {
	orbErr := errorMapper.Transform(ctx, err)
	errMessage := orbErr.String()
	if orbErr.IsDefaultError() {
		errMessage = err.Error()
	}
	return errMessage
}

var errorMapper = errs.NewMapper(
	[]errs.ExposedErrors[*OrbitalError]{
		{
			InternalErrorChain: []error{ErrNoConnectedRegionsForKey},
			ExposedError: &OrbitalError{
				Message: "no connected regions found for key",
				Code:    "NO_CONNECTED_REGIONS_FOR_KEY",
			},
		},
		{
			InternalErrorChain: []error{ErrTargetNotConfigured},
			ExposedError: &OrbitalError{
				Message: "target not configured for region",
				Code:    "UNCONFIGURED_REGION",
			},
		},
		{
			InternalErrorChain: []error{ErrKeyAccessMetadataNotFound},
			ExposedError: &OrbitalError{
				Message: "key does not support system region",
				Code:    "UNSUPPORTED_REGION",
			},
		},
	},
	nil,
)
