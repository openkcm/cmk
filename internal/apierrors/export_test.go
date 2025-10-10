package apierrors

import (
	"context"

	"github.com/openkcm/cmk/internal/api/cmkapi"
)

var (
	CountMatchingErrors = countMatchingErrors
	DefaultMapper       = defaultMapper
)

func (m *APIErrorMapper) Transform(ctx context.Context, err error) *cmkapi.ErrorMessage {
	return m.transform(ctx, err)
}
