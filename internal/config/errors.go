package config

import (
	"errors"
)

var (
	ErrLoadMTLSConfig     = errors.New("failed to load MTLS config")
	ErrLoadUsernameConfig = errors.New("failed to load username config")
	ErrLoadPasswordConfig = errors.New("failed to load password config")
)
