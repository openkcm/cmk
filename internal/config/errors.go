package config

import (
	"errors"
)

var (
	ErrLoadMTLSConfig          = errors.New("failed to load MTLS config")
	ErrSentinelMasterNameEmpty = errors.New("sentinel master name must be set when sentinel is enabled")
	ErrSentinelNoAddresses     = errors.New("at least one sentinel address must be set when sentinel is enabled")
)
