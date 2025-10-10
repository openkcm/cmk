package async

import "errors"

var (
	ErrEnqueueingTask    = errors.New("enqueue task")
	ErrClientShutdown    = errors.New("client shutdown")
	ErrStartingWorker    = errors.New("starting worker")
	ErrCreatingScheduler = errors.New("creating scheduler")
	ErrRunningScheduler  = errors.New("running scheduler")
	ErrReadingConfig     = errors.New("error reading scheduler task config file")
	ErrInvalidConfig     = errors.New("invalid scheduler task config")
)
