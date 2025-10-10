package config

import (
	"errors"
)

var (
	ErrLoadMTLSConfig = errors.New("failed to load MTLS config")
)
