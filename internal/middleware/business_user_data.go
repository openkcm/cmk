package middleware

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/golang-jwt/jwt/v5"
	"github.com/openkcm/common-sdk/pkg/auth"
	"github.com/openkcm/common-sdk/pkg/storage/keyvalue"

	"github.com/openkcm/cmk/internal/api/write"
	"github.com/openkcm/cmk/internal/apierrors"
	"github.com/openkcm/cmk/internal/authz"
	"github.com/openkcm/cmk/internal/constants"
	"github.com/openkcm/cmk/internal/log"
	"github.com/openkcm/cmk/internal/manager"
	cmkcontext "github.com/openkcm/cmk/utils/context"
)

var (
	ErrNoBusinessUserDataHeader = errors.New("no client data header found")
	ErrMissingSignatureHeader   = errors.New("missing client data signature header")
	ErrPublicKeyNotFound        = errors.New("public key not found or invalid")
	ErrVerifySignatureFailed    = errors.New("failed to verify client data signature")
	ErrDecodeBusinessUserData   = errors.New("failed to decode business data from header")
	ErrTriedToBeSystem          = errors.New("attempted to be system")
)

// RoleGetter defines the interface for getting roles from group IAM identifiers for better unit testing
type RoleGetter interface {
	GetRoleFromIAM(ctx context.Context, iamIdentifiers []string) (constants.BusinessRole, error)
}

// BusinessUserDataMiddleware extracts client data from headers, verifies, and adds to context
func BusinessUserDataMiddleware(
	signingKeyStorage keyvalue.ReadOnlyStringToBytesStorage,
	authContextFields []string,
	roleGetter RoleGetter,
) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(
			func(w http.ResponseWriter, r *http.Request) {
				if canBypassBusinessUserData(r) {
					next.ServeHTTP(w, r)
					return
				}

				ctx, err := prepareContext(r, signingKeyStorage, authContextFields, roleGetter)
				if err != nil {
					log.Error(r.Context(), "Client data processing error", err)
					if errors.Is(err, manager.ErrMultipleRolesInGroups) ||
						errors.Is(err, manager.ErrZeroRolesInGroups) {
						e := apierrors.APIErrorMapper.Transform(r.Context(), err)
						write.ErrorResponse(r.Context(), w, e)
					} else {
						write.ErrorResponse(r.Context(), w, apierrors.ErrNoBusinessUserData)
					}

					return
				}

				next.ServeHTTP(w, r.WithContext(ctx)) //nolint:contextcheck
			},
		)
	}
}

func canBypassBusinessUserData(r *http.Request) bool {
	pattern := extractPattern(r.Pattern, constants.BasePath)
	return pattern == "GET /swagger"
}

// prepareContext extracts, validates, and verifies client data from request
func prepareContext(
	r *http.Request,
	signingKeyStorage keyvalue.ReadOnlyStringToBytesStorage,
	authContextFields []string,
	roleGetter RoleGetter,
) (context.Context, error) {
	businessUserData, err := extractBusinessUserData(r)
	if err != nil || businessUserData == nil {
		return r.Context(), err
	}

	pemData, exists := signingKeyStorage.Get(businessUserData.KeyID)
	if !exists {
		return r.Context(), ErrPublicKeyNotFound
	}

	if businessUserData.SignatureAlgorithm != auth.SignatureAlgorithmRS256 {
		return r.Context(), fmt.Errorf(
			"%w: unsupported signature algorithm '%s'", ErrPublicKeyNotFound, businessUserData.SignatureAlgorithm,
		)
	}

	publicKey, err := jwt.ParseRSAPublicKeyFromPEM(pemData)
	if err != nil {
		return r.Context(), ErrPublicKeyNotFound
	}

	err = verifyBusinessUserDataSignature(r, businessUserData, publicKey)
	if err != nil {
		return r.Context(), err
	}

	ctx := cmkcontext.InjectBusinessUserData(r.Context(), businessUserData, authContextFields)

	// Validate group roles after injecting the real client identity so that
	// GetRoleFromIAM has a proper identity in context.
	err = validateGroupRoles(r.WithContext(ctx), businessUserData, roleGetter)
	if err != nil {
		return r.Context(), err
	}

	return ctx, nil
}

// extractBusinessUserData retrieves and decodes client data from request headers
func extractBusinessUserData(r *http.Request) (*auth.ClientData, error) {
	businessUserDataHeader := r.Header.Get(auth.HeaderClientData)
	if businessUserDataHeader == "" {
		return nil, ErrNoBusinessUserDataHeader
	}

	businessUserData, err := auth.DecodeFrom(businessUserDataHeader)
	if err != nil {
		return nil, fmt.Errorf("%w: '%s': %w", ErrDecodeBusinessUserData, businessUserDataHeader, err)
	}

	if businessUserData == nil {
		return nil, fmt.Errorf("%w: '%s'", ErrMissingSignatureHeader, businessUserDataHeader)
	}

	for _, group := range businessUserData.Groups {
		log.Debug(r.Context(), "extracted client data group:", slog.String("group", group))
	}

	log.Debug(r.Context(), "extracted client data:", slog.String("type", businessUserData.Type))
	log.Debug(r.Context(), "extracted client data:", slog.String("region", businessUserData.Region))

	for k, v := range businessUserData.AuthContext {
		log.Debug(r.Context(), "extracted client data auth context:", slog.String(k, v))
	}

	log.Debug(r.Context(), "extracted client data:", slog.String("keyId", businessUserData.KeyID))
	log.Debug(
		r.Context(), "extracted client data:",
		slog.String("signatureAlgorithm", string(businessUserData.SignatureAlgorithm)),
	)

	return businessUserData, nil
}

// verifyBusinessUserDataSignature checks the signature of client data
func verifyBusinessUserDataSignature(r *http.Request, businessUserData *auth.ClientData, publicKey any) error {
	businessUserDataSignatureHeader := r.Header.Get(auth.HeaderClientDataSignature)
	if businessUserDataSignatureHeader == "" {
		return ErrMissingSignatureHeader
	}

	err := businessUserData.Verify(publicKey, businessUserDataSignatureHeader)
	if err != nil {
		return fmt.Errorf("%w: %w", ErrVerifySignatureFailed, err)
	}

	return nil
}

// validateGroupRoles ensures that all groups in client data belong to only one role type
func validateGroupRoles(
	r *http.Request,
	businessUserData *auth.ClientData,
	roleGetter RoleGetter,
) error {
	ctx := r.Context()

	// Always check that groups are provided - reject tenant access if no groups
	if len(businessUserData.Groups) == 0 {
		log.Debug(ctx, "No groups provided in client data for tenant access")
		return manager.ErrZeroRolesInGroups
	}

	// Check if the API is on the allow list
	pattern := extractPattern(r.Pattern, constants.BasePath)
	_, isAllowedAPI := authz.AllowListByAPI[pattern]

	// If API is allowed, skip mixed role validation
	if isAllowedAPI {
		log.Debug(
			ctx, "API is on allow list, skipping group role validation",
			slog.String("pattern", pattern),
		)
		return nil
	}

	// For non-allowed APIs, if there is only one group, no need to validate for mixed roles
	if len(businessUserData.Groups) == 1 {
		return nil
	}

	// Get all roles associated with the groups using existing GroupManager method
	roles, err := roleGetter.GetRoleFromIAM(ctx, businessUserData.Groups)
	if errors.Is(err, manager.ErrMultipleRolesInGroups) {
		log.Debug(
			ctx, "Segregation of roles not fulfilled in client data groups",
			slog.Any("roles", roles),
		)
		return err
	}
	if err != nil {
		return fmt.Errorf("failed to get roles for groups: %w", err)
	}

	return nil
}
