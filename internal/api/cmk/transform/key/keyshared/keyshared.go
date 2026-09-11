package keyshared

import (
	"errors"
)

var (
	ErrFromAPI             = errors.New("error from api")
	ErrProviderIsRequired  = errors.New("provider is required")
	ErrRegionIsRequired    = errors.New("region is required")
	ErrAlgorithmIsRequired = errors.New("algorithm is required")
	ErrInvalidKeyProvider  = errors.New("invalid key provider")
)
