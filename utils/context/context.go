package context

import (
	"context"
	"errors"

	"github.com/bartventer/gorm-multitenancy/middleware/nethttp/v8"
	"github.com/google/uuid"

	"github.com/openkcm/cmk-core/internal/errs"
)

var (
	ErrExtractTenantID = errors.New("could not extract tenant ID from context")
	ErrGetRequestID    = errors.New("no requestID found in context")
)

func ExtractTenantID(ctx context.Context) (string, error) {
	tenantID, ok := ctx.Value(nethttp.TenantKey).(string)
	if !ok || tenantID == "" {
		return "", errs.Wrap(ErrExtractTenantID, nethttp.ErrTenantInvalid)
	}

	return tenantID, nil
}

func CreateTenantContext(ctx context.Context, tenantSchema string) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}

	return context.WithValue(ctx, nethttp.TenantKey, tenantSchema)
}

type key string

const requestID = key("requestID")

func InjectRequestID(ctx context.Context) context.Context {
	return context.WithValue(ctx, requestID, uuid.NewString())
}

func GetRequestID(ctx context.Context) (string, error) {
	requestID, ok := ctx.Value(requestID).(string)
	if !ok || requestID == "" {
		return "", ErrGetRequestID
	}

	return requestID, nil
}
