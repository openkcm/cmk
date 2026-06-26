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
	ErrExtractTenantID                    = errors.New("could not extract tenant ID from context")
	ErrGetRequestID                       = errors.New("no requestID found in context")
	ErrExtractBusinessUserData            = errors.New("could not extract business data from context")
	ErrExtractBusinessUserDataAuthContext = errors.New("could not extract field from business data auth context")
	ErrExtractUserType                    = errors.New("could not extract user type from context")
	ErrInvalidUserType                    = errors.New("invalid user type")
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

// User data

func ExtractUserType(ctx context.Context) (string, error) {
	userType, ok := ctx.Value(constants.UserType).(constants.UserTypeValue)
	if !ok || userType == "" {
		return "", ErrExtractUserType
	}

	return string(userType), nil
}

// Business User Data

func InjectBusinessUserData(
	ctx context.Context,
	businessUserData *auth.ClientData,
	authContextFields []string,
) context.Context {
	filteredAuthCtx := make(map[string]string)

	for _, field := range authContextFields {
		if value, exists := businessUserData.AuthContext[field]; exists {
			filteredAuthCtx[field] = value
		}
	}

	businessUserData.AuthContext = filteredAuthCtx
	ctx = context.WithValue(ctx, constants.UserType, constants.BusinessUser)
	ctx = context.WithValue(ctx, constants.BusinessUserData, businessUserData)

	return ctx
}

func WithInjectBusinessUserData(businessUserData *auth.ClientData, authContextFields []string) Opt {
	return func(ctx context.Context) context.Context {
		return InjectBusinessUserData(ctx, businessUserData, authContextFields)
	}
}

func ExtractBusinessUserData(ctx context.Context) (*auth.ClientData, error) {
	// Note we don't check the user type here, as it's possible for internal users
	// to be associated with business data. Eg for the case with workflow task handling
	businessUserData, ok := ctx.Value(constants.BusinessUserData).(*auth.ClientData)
	if !ok || businessUserData == nil {
		return nil, ErrExtractBusinessUserData
	}

	return businessUserData, nil
}

func ExtractBusinessUserDataIdentifier(ctx context.Context) (string, error) {
	businessUserData, err := ExtractBusinessUserData(ctx)
	if err != nil {
		return "", err
	}

	return businessUserData.Identifier, nil
}

func ExtractBusinessUserDataGroups(ctx context.Context) ([]string, error) {
	businessUserData, err := ExtractBusinessUserData(ctx)
	if err != nil {
		return nil, err
	}

	return businessUserData.Groups, nil
}

func ExtractBusinessUserDataGroupsString(ctx context.Context) ([]string, error) {
	businessUserData, err := ExtractBusinessUserData(ctx)
	if err != nil {
		return nil, err
	}

	return businessUserData.Groups, nil
}

func ExtractBusinessUserDataAuthContextField(ctx context.Context, field string) (string, error) {
	businessUserData, err := ExtractBusinessUserData(ctx)
	if err != nil {
		return "", ErrExtractBusinessUserDataAuthContext
	}

	value, ok := businessUserData.AuthContext[field]
	if !ok || value == "" {
		return "", ErrExtractBusinessUserDataAuthContext
	}

	return value, nil
}

func ExtractBusinessUserDataIssuer(ctx context.Context) (string, error) {
	return ExtractBusinessUserDataAuthContextField(ctx, "issuer")
}

func ExtractBusinessUserDataAuthContext(ctx context.Context) (map[string]string, error) {
	businessUserData, err := ExtractBusinessUserData(ctx)
	if err != nil {
		return nil, err
	}

	authContext := maps.Clone(businessUserData.AuthContext)

	return authContext, nil
}

// Internal User Data

func InjectInternalUserData(
	ctx context.Context,
	role constants.InternalRole,
) (context.Context, error) {
	userType, ok := ctx.Value(constants.UserType).(constants.UserTypeValue)
	if ok && userType != constants.InternalUser {
		// We don't want business users changing to internal
		return ctx, ErrInvalidUserType
	}
	ctx = InjectRequestID(ctx, uuid.NewString())
	ctx = context.WithValue(ctx, constants.UserType, constants.InternalUser)
	ctx = context.WithValue(ctx, constants.InternalUserData, role)

	return ctx, nil
}

// ExtractUserIdentifier returns a string identifier for the caller regardless of user type.
// For business users this is the IAM identifier; for internal users it is the role string.
func ExtractUserIdentifier(ctx context.Context) (string, error) {
	businessUserData, err := ExtractBusinessUserData(ctx)
	if err == nil {
		return businessUserData.Identifier, nil
	}

	internalRole, err := ExtractInternalRole(ctx)
	if err != nil {
		return "", err
	}

	return string(internalRole), nil
}

func ExtractInternalRole(ctx context.Context) (constants.InternalRole, error) {
	userType, err := ExtractUserType(ctx)
	if err != nil {
		return "", err
	}
	if userType != string(constants.InternalUser) {
		return "", ErrInvalidUserType
	}
	internalRole, ok := ctx.Value(constants.InternalUserData).(constants.InternalRole)
	if !ok || internalRole == "" {
		return "", ErrExtractBusinessUserData
	}

	return internalRole, nil
}

// User type context conversions

func BusinessToInternalContext(
	ctx context.Context,
	role constants.InternalRole,
) (context.Context, error) {
	userType, ok := ctx.Value(constants.UserType).(constants.UserTypeValue)
	if ok && userType != constants.BusinessUser {
		return ctx, ErrInvalidUserType
	}

	tenantID, err := ExtractTenantID(ctx)
	if err != nil {
		return ctx, err
	}

	internalCtx := CreateTenantContext(context.TODO(), tenantID)
	internalCtx, err = InjectInternalUserData(internalCtx, role)
	if err != nil {
		return ctx, err
	}

	requestID, err := GetRequestID(ctx)
	if err == nil {
		internalCtx = InjectRequestID(internalCtx, requestID)
	}

	return internalCtx, nil
}

func GetFromContext[T any](ctx context.Context, key any) T {
	if val := ctx.Value(key); val != nil {
		return val.(T)
	}
	var zero T
	return zero
}
