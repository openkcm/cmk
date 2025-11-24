package odata

import (
	"errors"
)

var (
	// ErrFilterNotToSpec is odata spec related error:
	ErrFilterNotToSpec = errors.New("odata filter parameter: not to odata spec")

	// All the following are CMK domain specific errors:

	// ErrSchemaOperationNotSupportedForType is an eror with the schema specification
	// and would be better as a compile-time check if it was possible
	ErrSchemaOperationNotSupportedForType = errors.New("odata filter schema: operation not supported for type")

	// All the following are errors from the filter parameter parse

	ErrFilterOperationNotSupported = errors.New("odata filter parameter: operation not supported")
	ErrFilterTypeNotSupported      = errors.New("odata filter parameter: type not supported")

	ErrFilterInvalidValue = errors.New("odata filter parameter: value invalid")

	ErrFilterNonSchema               = errors.New("odata filter parameter: non schema")
	ErrFilterTypeIncompatible        = errors.New("odata filter parameter: type incompatible")
	ErrFilterValueConversionFailed   = errors.New("odata filter parameter: type conversation failed")
	ErrFilterValueModificationFailed = errors.New("odata filter parameter: value modification failed")
)
