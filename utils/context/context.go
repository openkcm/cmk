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
	ErrExtractSource                = errors.New("could not extract source from context")
	ErrInvalidSource                = errors.New("invalid Source")
)

type Opt func(ctx context.Context) context.Context

//nolint:fatcontext
func New(ctx context.Context, opts ...Opt) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}

	for _, opt := range opts {
		ctx = opt(ctx)
	}
	return ctx
}

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

func WithTenant(tenantSchema string) Opt {
	return func(ctx context.Context) context.Context {
		return CreateTenantContext(ctx, tenantSchema)
	}
}

type key string

const requestIDKey = key("requestID")

func InjectRequestID(ctx context.Context, requestID string) context.Context {
	if requestID == "" {
		requestID = uuid.NewString()
	}
	return context.WithValue(ctx, requestIDKey, requestID)
}

func GetRequestID(ctx context.Context) (string, error) {
	requestID, ok := ctx.Value(requestIDKey).(string)
	if !ok || requestID == "" {
		return "", ErrGetRequestID
	}

	return requestID, nil
}

func InjectBusinessClientData(
	ctx context.Context,
	clientData *auth.ClientData,
	authContextFields []string,
) context.Context {
	filteredAuthCtx := make(map[string]string)

	for _, field := range authContextFields {
		if value, exists := clientData.AuthContext[field]; exists {
			filteredAuthCtx[field] = value
		}
	}

	clientData.AuthContext = filteredAuthCtx
	ctx = context.WithValue(ctx, constants.Source, constants.BusinessSource)
	ctx = context.WithValue(ctx, constants.ClientData, clientData)

	return ctx
}

func WithInjectBusinessClientData(clientData *auth.ClientData, authContextFields []string) Opt {
	return func(ctx context.Context) context.Context {
		return InjectBusinessClientData(ctx, clientData, authContextFields)
	}
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

func ExtractClientDataGroups(ctx context.Context) ([]string, error) {
	clientData, err := ExtractClientData(ctx)
	if err != nil {
		return nil, err
	}

	return clientData.Groups, nil
}

func ExtractClientDataGroupsString(ctx context.Context) ([]string, error) {
	clientData, err := ExtractClientData(ctx)
	if err != nil {
		return nil, err
	}

	return clientData.Groups, nil
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

func InjectInternalClientData(
	ctx context.Context,
	role constants.InternalRole,
) (context.Context, error) {
	source, ok := ctx.Value(constants.Source).(constants.SourceValue)
	if ok && source != constants.InternalSource {
		// We don't want business users changing to internal
		return ctx, ErrInvalidSource
	}
	ctx = InjectRequestID(ctx, uuid.NewString())
	ctx = context.WithValue(ctx, constants.Source, constants.InternalSource)
	ctx = context.WithValue(ctx, constants.InternalData, role)

	return ctx, nil
}

func BusinessToInternalContext(
	ctx context.Context,
	role constants.InternalRole,
) (context.Context, error) {
	source, ok := ctx.Value(constants.Source).(constants.SourceValue)
	if ok && source != constants.BusinessSource {
		return ctx, ErrInvalidSource
	}

	tenantID, err := ExtractTenantID(ctx)
	if err != nil {
		return ctx, err
	}

	internalCtx := CreateTenantContext(nil, tenantID)
	internalCtx, err = InjectInternalClientData(internalCtx, role)
	if err != nil {
		return ctx, err
	}

	requestID, err := GetRequestID(ctx)
	if err == nil {
		internalCtx = InjectRequestID(internalCtx, requestID)
	}

	return internalCtx, nil
}

func ExtractInternalRole(ctx context.Context) (constants.InternalRole, error) {
	internalRole, ok := ctx.Value(constants.InternalData).(constants.InternalRole)
	if !ok || internalRole == "" {
		return "", ErrExtractClientData
	}

	return internalRole, nil
}

func ExtractSource(ctx context.Context) (string, error) {
	source, ok := ctx.Value(constants.Source).(constants.SourceValue)
	if !ok || source == "" {
		return "", ErrExtractSource
	}

	return string(source), nil
}
