package testutils

import (
	"errors"
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
	"gorm.io/gorm"

	multitenancy "github.com/bartventer/gorm-multitenancy/v8"

	"github.com/openkcm/cmk-core/internal/errs"
	"github.com/openkcm/cmk-core/internal/model"
)

// callbackError - the error callback name.
const callbackError = "error"

// callback types
const (
	callbackCreate = "create"
	callbackUpdate = "update"
	callbackDelete = "delete"
	callbackQuery  = "query"
	callbackRow    = "row"
	callbackRaw    = "raw"
)

// gorm default callback names
const (
	gormCreate = "gorm:create"
	gormUpdate = "gorm:update"
	gormQuery  = "gorm:query"
	gormDelete = "gorm:delete"
	gormRow    = "gorm:row"
	gormRaw    = "gorm:raw"
)

// allCallbacks - all available callbacks.
var allCallbacks = []string{
	callbackCreate,
	callbackUpdate,
	callbackDelete,
	callbackQuery,
	callbackRow,
	callbackRaw,
}

var (
	ErrRegisterFailed   = errors.New("failed to register callback")
	ErrUnregisterFailed = errors.New("failed to unregister callback")
)

// ErrorForced - a helper to force an error in the database using gorm
type ErrorForced struct {
	err       error
	t         *testing.T
	db        *multitenancy.DB
	callbacks []string
}

// NewDBErrorForced - creates a new ErrorForced instance.
func NewDBErrorForced(db *multitenancy.DB, forcedErr error) *ErrorForced {
	return &ErrorForced{
		err: forcedErr,
		db:  db,
	}
}

// WithCreate - forces the error in the create callback.
func (e *ErrorForced) WithCreate() *ErrorForced {
	e.callbacks = append(e.callbacks, callbackCreate)
	return e
}

// WithUpdate - forces the error in the update callback.
func (e *ErrorForced) WithUpdate() *ErrorForced {
	e.callbacks = append(e.callbacks, callbackUpdate)
	return e
}

// WithDelete - forces the error in the delete callback.
func (e *ErrorForced) WithDelete() *ErrorForced {
	e.callbacks = append(e.callbacks, callbackDelete)
	return e
}

// WithQuery - forces the error in the query callback.
func (e *ErrorForced) WithQuery() *ErrorForced {
	e.callbacks = append(e.callbacks, callbackQuery)
	return e
}

// WithRow - forces the error in the row callback.
func (e *ErrorForced) WithRow() *ErrorForced {
	e.callbacks = append(e.callbacks, callbackRow)
	return e
}

// WithRaw - forces the error in the raw callback.
func (e *ErrorForced) WithRaw() *ErrorForced {
	e.callbacks = append(e.callbacks, callbackRaw)
	return e
}

// Register - registers the error callback.
func (e *ErrorForced) Register() {
	if len(e.callbacks) == 0 {
		e.callbacks = allCallbacks
	}

	for _, callback := range e.callbacks {
		err := e.registerCallback(callback)
		assert.NoError(e.t, err)
	}
}

// Unregister - unregisters the error callback.
func (e *ErrorForced) Unregister() {
	for _, callback := range e.callbacks {
		err := e.unregisterCallback(callback)
		assert.NoError(e.t, err)
	}

	e.callbacks = nil
}

// registerCallback - registers the error callback.
func (e *ErrorForced) registerCallback(callback string) error {
	var err error

	switch callback {
	case callbackCreate:
		err = e.db.Callback().
			Create().
			Before(gormCreate).
			Register(callbackError, func(db *gorm.DB) {
				err = db.AddError(e.err)
			})
	case callbackUpdate:
		err = e.db.Callback().
			Update().
			Before(gormUpdate).
			Register(callbackError, func(db *gorm.DB) {
				err = db.AddError(e.err)
			})
	case callbackDelete:
		err = e.db.Callback().
			Delete().
			Before(gormDelete).
			Register(callbackError, func(db *gorm.DB) {
				err = db.AddError(e.err)
			})
	case callbackQuery:
		err = e.db.Callback().
			Query().
			Before(gormQuery).
			Register(callbackError, func(db *gorm.DB) {
				if db.Statement.Schema.ModelType == reflect.TypeOf(model.Tenant{}) {
					// Skip tenant model queries
					return
				}

				err = db.AddError(e.err)
			})
	}

	if err != nil {
		return errs.Wrap(ErrRegisterFailed, err)
	}

	return nil
}

// unregisterCallback - unregisters the error callback.
func (e *ErrorForced) unregisterCallback(callback string) error {
	var err error

	switch callback {
	case callbackCreate:
		err = e.db.Callback().Create().Before(gormCreate).Remove(callbackError)
	case callbackUpdate:
		err = e.db.Callback().Update().Before(gormUpdate).Remove(callbackError)
	case callbackDelete:
		err = e.db.Callback().Delete().Before(gormDelete).Remove(callbackError)
	case callbackQuery:
		err = e.db.Callback().Query().Before(gormQuery).Remove(callbackError)
	case callbackRow:
		err = e.db.Callback().Row().Before(gormRow).Remove(callbackError)
	case callbackRaw:
		err = e.db.Callback().Raw().Before(gormRaw).Remove(callbackError)
	}

	if err != nil {
		return errs.Wrap(ErrUnregisterFailed, err)
	}

	return nil
}
