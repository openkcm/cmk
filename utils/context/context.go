package context

import (
	"context"
	"errors"
	"maps"

	"github.com/bartventer/gorm-multitenancy/middleware/nethttp/v8"
	"github.com/google/uuid"
	"github.com/openkcm/common-sdk/pkg/auth"

	"github.com/openkcm/cmk/internal/constants"
	"github.com/openkcm/cmk/internal/errs"
)

var (
	ErrExtractTenantID              = errors.New("could not extract tenant ID from context")
	ErrGetRequestID                 = errors.New("no requestID found in context")
	ErrExtractClientData            = errors.New("could not extract client data from context")
	ErrExtractClientDataAuthContext = errors.New("could not extract field from client data auth context")
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

func ExtractClientData(ctx context.Context) (*auth.ClientData, error) {
	clientData, ok := ctx.Value(constants.ClientData).(*auth.ClientData)
	if !ok || clientData == nil {
		return nil, ErrExtractClientData
	}

	return clientData, nil
}

func ExtractClientDataIdentifier(ctx context.Context) (string, error) {
	clientData, err := ExtractClientData(ctx)
	if err != nil {
		return "", err
	}

	return clientData.Identifier, nil
}

func ExtractClientDataGroups(ctx context.Context) ([]constants.UserGroup, error) {
	clientData, err := ExtractClientData(ctx)
	if err != nil {
		return nil, err
	}

	clientGroups := make([]constants.UserGroup, len(clientData.Groups))
	for i, g := range clientData.Groups {
		clientGroups[i] = constants.UserGroup(g)
	}

	return clientGroups, nil
}

func ExtractClientDataAuthContextField(ctx context.Context, field string) (string, error) {
	clientData, err := ExtractClientData(ctx)
	if err != nil {
		return "", ErrExtractClientDataAuthContext
	}

	value, ok := clientData.AuthContext[field]
	if !ok || value == "" {
		return "", ErrExtractClientDataAuthContext
	}

	return value, nil
}

// ExtractClientDataIssuer extracts the issuer from client data auth context
func ExtractClientDataIssuer(ctx context.Context) (string, error) {
	return ExtractClientDataAuthContextField(ctx, "issuer")
}

func ExtractClientDataAuthContext(ctx context.Context) (map[string]string, error) {
	clientData, err := ExtractClientData(ctx)
	if err != nil {
		return nil, err
	}

	authContext := maps.Clone(clientData.AuthContext)

	return authContext, nil
}
