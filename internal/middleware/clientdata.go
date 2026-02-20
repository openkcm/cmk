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
	"github.com/openkcm/cmk/internal/constants"
	"github.com/openkcm/cmk/internal/log"
	"github.com/openkcm/cmk/internal/manager"
	cmkcontext "github.com/openkcm/cmk/utils/context"
)

var (
	ErrNoClientDataHeader     = errors.New("no client data header found")
	ErrMissingSignatureHeader = errors.New("missing client data signature header")
	ErrPublicKeyNotFound      = errors.New("public key not found or invalid")
	ErrVerifySignatureFailed  = errors.New("failed to verify client data signature")
	ErrDecodeClientData       = errors.New("failed to decode client data from header")
	ErrTriedToBeSystem        = errors.New("attempted to be system")
)

// RoleGetter defines the interface for getting roles from group IAM identifiers for better unit testing
type RoleGetter interface {
	GetRoleFromIAM(ctx context.Context, iamIdentifiers []string) (constants.Role, error)
}

// ClientDataMiddleware extracts client data from headers, verifies, and adds to context
func ClientDataMiddleware(
	signingKeyStorage keyvalue.ReadOnlyStringToBytesStorage,
	authContextFields []string,
	roleGetter RoleGetter,
) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(
			func(w http.ResponseWriter, r *http.Request) {
				ctx, err := prepareClientContext(r, signingKeyStorage, authContextFields, roleGetter)
				if err != nil {
					log.Debug(r.Context(), "Client data processing error", log.ErrorAttr(err))
					if errors.Is(err, manager.ErrMultipleRolesInGroups) ||
						errors.Is(err, manager.ErrZeroRolesInGroups) {
						e := apierrors.APIErrorMapper.Transform(r.Context(), err)
						write.ErrorResponse(r.Context(), w, e)
					} else {
						write.ErrorResponse(r.Context(), w, apierrors.ErrNoClientData)
					}

					return
				}

				next.ServeHTTP(w, r.WithContext(ctx)) //nolint:contextcheck
			},
		)
	}
}

// prepareClientContext extracts, validates, and verifies client data from request
func prepareClientContext(
	r *http.Request,
	signingKeyStorage keyvalue.ReadOnlyStringToBytesStorage,
	authContextFields []string,
	roleGetter RoleGetter,
) (context.Context, error) {
	clientData, err := extractClientData(r)
	if err != nil || clientData == nil {
		return r.Context(), err
	}

	// Validate that all groups belong to only one role type
	// either KeyAdminRole, TenantAdminRole, or TenantAuditorRole.
	// Also reject tenant access if no groups are provided.
	err = validateGroupRoles(r.Context(), clientData, roleGetter)
	if err != nil {
		return r.Context(), err
	}

	pemData, exists := signingKeyStorage.Get(clientData.KeyID)
	if !exists {
		return r.Context(), ErrPublicKeyNotFound
	}

	if clientData.SignatureAlgorithm != auth.SignatureAlgorithmRS256 {
		return r.Context(), fmt.Errorf(
			"%w: unsupported signature algorithm '%s'", ErrPublicKeyNotFound, clientData.SignatureAlgorithm,
		)
	}

	publicKey, err := jwt.ParseRSAPublicKeyFromPEM(pemData)
	if err != nil {
		return r.Context(), ErrPublicKeyNotFound
	}

	err = verifyClientDataSignature(r, clientData, publicKey)
	if err != nil {
		return r.Context(), err
	}

	ctx := cmkcontext.InjectClientData(r.Context(), clientData, authContextFields)

	return ctx, nil
}

// extractClientData retrieves and decodes client data from request headers
func extractClientData(r *http.Request) (*auth.ClientData, error) {
	clientDataHeader := r.Header.Get(auth.HeaderClientData)
	if clientDataHeader == "" {
		return nil, ErrNoClientDataHeader
	}

	clientData, err := auth.DecodeFrom(clientDataHeader)
	if err != nil {
		return nil, fmt.Errorf("%w: '%s': %w", ErrDecodeClientData, clientDataHeader, err)
	}

	if clientData == nil {
		return nil, fmt.Errorf("%w: '%s'", ErrMissingSignatureHeader, clientDataHeader)
	}

	for _, group := range clientData.Groups {
		log.Debug(r.Context(), "extracted client data group:", slog.String("group", group))
	}

	log.Debug(r.Context(), "extracted client data:", slog.String("Type", clientData.Type))
	log.Debug(r.Context(), "extracted client data:", slog.String("Region", clientData.Region))

	for k, v := range clientData.AuthContext {
		log.Debug(r.Context(), "extracted client data auth context:", slog.String(k, v))
	}

	log.Debug(r.Context(), "extracted client data:", slog.String("keyID", clientData.KeyID))
	log.Debug(
		r.Context(), "extracted client data:",
		slog.String("SignatureAlgorithm", string(clientData.SignatureAlgorithm)),
	)

	if clientData.Identifier == constants.SystemUser.String() {
		return nil, ErrTriedToBeSystem
	}

	return clientData, nil
}

// verifyClientDataSignature checks the signature of client data
func verifyClientDataSignature(r *http.Request, clientData *auth.ClientData, publicKey any) error {
	clientDataSignatureHeader := r.Header.Get(auth.HeaderClientDataSignature)
	if clientDataSignatureHeader == "" {
		return ErrMissingSignatureHeader
	}

	err := clientData.Verify(publicKey, clientDataSignatureHeader)
	if err != nil {
		return fmt.Errorf("%w: %w", ErrVerifySignatureFailed, err)
	}

	return nil
}

// validateGroupRoles ensures that all groups in client data belong to only one role type
func validateGroupRoles(
	ctx context.Context,
	clientData *auth.ClientData,
	roleGetter RoleGetter,
) error {
	// If there are no groups or only one group, no need to validate
	if len(clientData.Groups) == 1 {
		return nil
	} else if len(clientData.Groups) == 0 {
		log.Debug(ctx, "No groups provided in client data for tenant access")
		return manager.ErrZeroRolesInGroups
	}

	// Get all roles associated with the groups using existing GroupManager method
	roles, err := roleGetter.GetRoleFromIAM(ctx, clientData.Groups)
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
