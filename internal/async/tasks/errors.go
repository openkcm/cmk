package tasks

import "errors"

var (
	ErrRunningTask    = errors.New("task failed")
	ErrRoleStillEmpty = errors.New("system role still empty after enrichment")
)
