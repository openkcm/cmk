package apierrors

import (
	"slices"

	"github.com/openkcm/cmk/internal/errs"
)

type APIError struct {
	Code    string
	Message string
	Status  int
	Context *map[string]any
}

func (e *APIError) SetContext(context *map[string]any) {
	e.Context = context
}

func (e *APIError) DefaultError() *APIError {
	return InternalServerErrorMessage()
}

var APIErrorMapper = errs.NewMapper(slices.Concat(
	keyConfiguration,
	keyVersion,
	workflow,
	system,
	label,
	tags,
	key,
	tenantconfig,
	groups,
	tenants,
	userinfo,
	defaultMapper,
), highPrio)
