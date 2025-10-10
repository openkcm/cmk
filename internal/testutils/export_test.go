package testutils

const (
	ExportCallbackCreate = callbackCreate
	ExportCallbackUpdate = callbackUpdate
	ExportCallbackDelete = callbackDelete
	ExportCallbackQuery  = callbackQuery
	ExportCallbackRow    = callbackRow
	ExportCallbackRaw    = callbackRaw
)

var (
	ExportAll = allCallbacks
)

// ExportCallbacks - exports the error callbacks.
func (e *ErrorForced) ExportCallbacks() []string {
	return e.callbacks
}
