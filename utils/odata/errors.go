package odata

import "errors"

var (
	ErrFilterNotToSpec             = errors.New("odata filter parameter: not to odata spec")
	ErrFilterOperationNotSupported = errors.New("odata filter parameter: operation not supported")
	ErrFilterInvalidValue          = errors.New("odata filter parameter: value invalid")
	ErrFilterTypeNotSupported      = errors.New("odata filter parameter: type not supported")
	ErrFilterValueConversionFailed = errors.New("odata filter parameter: type conversion failed")
	ErrFilterNonSchema             = errors.New("odata filter parameter: invalid db mapping")
)
