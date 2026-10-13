package odata

import (
	"regexp"
	"slices"
	"strings"
)

const (
	None = ""
	And  = "and"
	Or   = "or"
)

// Separate everything into a slice element, mainting matching ” as one token
// e.g. ["a","eq","'1'","and","b","eq","'2'"]
func tokenise(s string) []string {
	tokeniser := regexp.MustCompile(`'(?:[^']|'')*'|\S+`)
	return tokeniser.FindAllString(s, -1)
}

func unquote(tok string) (string, error) {
	if len(tok) < 2 || tok[0] != '\'' {
		return tok, nil
	}

	if tok[len(tok)-1] != '\'' {
		return "", ErrFilterNotToSpec // opened quote, never closed
	}

	return strings.ReplaceAll(tok[1:len(tok)-1], "''", "'"), nil
}

func isReserved(token string) bool {
	reserved := []string{
		None,
		And,
		Or,
	}
	return slices.Contains(reserved, token) || isSupportedOperation(token)
}
