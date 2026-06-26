package commands

import "errors"

var (
	// ErrNonZeroExit is returned when a component exits with a non-zero exit code
	ErrNonZeroExit = errors.New("component exited with non-zero exit code")
)
