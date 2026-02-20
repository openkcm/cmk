package eventprocessor

import (
	"regexp"
	"strings"

	"github.com/openkcm/cmk/internal/constants"
)

type Error struct {
	Message string
	Code    string
}

// ParseOrbitalError returns a parsed error in orbital format
// If there is no code or no entries to parse, it sets the code to the default value
// The parsing is done with an expression expect a ":"
// right after the error code in SCREAMING_SNAKE_CASE
func ParseOrbitalError(errorMessage string) Error {
	code, message, found := strings.Cut(errorMessage, ":")

	// Matches screaming snake case until a without any whitespaces
	// and only matches alphabetic characters and "_"
	reg := regexp.MustCompile("^[A-Z]+(?:_[A-Z]+)*$")

	// Couldn't find ":" or the error does not contain an orbital error like code
	if !found || !reg.MatchString(code) {
		return Error{Message: errorMessage, Code: constants.DefaultErrorCode}
	}

	return Error{Message: message, Code: code}
}
