package transform

import (
	"errors"
	"time"
)

const (
	DefTimeFormat = time.RFC3339
)

var (
	ErrAPIInvalidProperty    = errors.New("field is invalid")
	ErrAPIUnexpectedProperty = errors.New("unexpected field")
)
