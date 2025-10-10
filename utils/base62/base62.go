package base62

import (
	"fmt"
	"strings"

	"github.com/jxskiss/base62"

	"github.com/openkcm/cmk/internal/errs"
)

const (
	SchemaNamePrefix = "_"
)

// EncodeSchemaNameBase62 encodes the input string using base62 encoding and returns a schema name prefixed
// with SchemaNamePrefix. The resulting encoded schema name must be between 3 and 62 characters long.
// Returns an error if the input is empty or the encoded length is out of bounds.
//
// Postgresql allows max 63 bytes for schema name. Keeps schema names in db encoding, usually UTF-8.
// In UTF-8, ASCII characters are 1 byte (a-z, A-Z, 0-9, _). Non-ASCII characters can be more than 1 byte.
// ASCII characters are 1 byte. If 63 characters long string in golang contains only ASCII characters,
// it will be 63 bytes long.
func EncodeSchemaNameBase62(input string) (string, error) {
	if input == "" {
		return "", ErrEmptyTenantID
	}

	encoded := base62.EncodeToString([]byte(input))
	if len(encoded) < 3 || len(encoded) > 62 {
		return "", fmt.Errorf("%w got %d", ErrEncodedSchemaNameLength, len(encoded))
	}

	return SchemaNamePrefix + encoded, nil
}

func DecodeSchemaNameBase62(encoded string) (string, error) {
	result := strings.TrimPrefix(encoded, SchemaNamePrefix)

	decodedBytes, err := base62.DecodeString(result)
	if err != nil {
		return "", errs.Wrap(ErrDecodingSchemaName,
			fmt.Errorf("error decoding base58: %w", err))
	}

	return string(decodedBytes), nil
}
