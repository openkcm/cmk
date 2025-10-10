package base62

import "errors"

var (
	ErrEmptyTenantID           = errors.New("empty tenant ID")
	ErrDecodingSchemaName      = errors.New("error decoding schema name")
	ErrEncodedSchemaNameLength = errors.New("encoded schema name has invalid length")
)
