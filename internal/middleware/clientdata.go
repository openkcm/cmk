package middleware

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/golang-jwt/jwt/v5"
	"github.com/openkcm/common-sdk/pkg/auth"
	"github.com/openkcm/common-sdk/pkg/commoncfg"
	"github.com/openkcm/common-sdk/pkg/storage/keyvalue"

	"github.com/openkcm/cmk/internal/constants"
	"github.com/openkcm/cmk/internal/flags"
	"github.com/openkcm/cmk/internal/log"
)

var (
	ErrNoClientDataHeader     = errors.New("no client data header found")
	ErrMissingSignatureHeader = errors.New("missing client data signature header")
	ErrPublicKeyNotFound      = errors.New("public key not found or invalid")
	ErrVerifySignatureFailed  = errors.New("failed to verify client data signature")
	ErrDecodeClientData       = errors.New("failed to decode client data from header")
)

// ClientDataMiddleware extracts client data from headers, verifies, and adds to context
// if feature gate is enabled, skip client data computation
// and pass empty context values
// this is to allow disabling client data computation
func ClientDataMiddleware(
	featureGates *commoncfg.FeatureGates,
	signingKeyStorage keyvalue.ReadOnlyStringToBytesStorage,
	authContextFields []string,
) func(http.Handler) http.Handler {
	clientDataComputationDisabled := featureGates.IsFeatureEnabled(flags.DisableClientDataComputation)
	if clientDataComputationDisabled {
		slog.Info("Client data computation is disabled by feature gate")
	}

	return func(next http.Handler) http.Handler {
		if clientDataComputationDisabled {
			return next
		}

		return clientDataHandler(signingKeyStorage, authContextFields, next)
	}
}

func clientDataHandler(
	signingKeyStorage keyvalue.ReadOnlyStringToBytesStorage,
	authContextFields []string,
	next http.Handler,
) http.Handler {
	return http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			clientData, err := extractClientData(r)
			if err != nil {
				logErrorAndContinue(r.Context(), err)
				next.ServeHTTP(w, r)

				return
			}

			pemData, exists := signingKeyStorage.Get(clientData.KeyID)
			if !exists {
				err := ErrPublicKeyNotFound
				logErrorAndContinue(r.Context(), err)
				next.ServeHTTP(w, r)

				return
			}

			if clientData.SignatureAlgorithm != auth.SignatureAlgorithmRS256 {
				err := fmt.Errorf(
					"%w: unsupported signature algorithm '%s'", ErrPublicKeyNotFound, clientData.SignatureAlgorithm,
				)
				logErrorAndContinue(r.Context(), err)
				next.ServeHTTP(w, r)

				return
			}

			publicKey, err := jwt.ParseRSAPublicKeyFromPEM(pemData)
			if err != nil {
				err = ErrPublicKeyNotFound
				logErrorAndContinue(r.Context(), err)
			}

			err = verifyClientDataSignature(r, clientData, publicKey)
			if err != nil {
				logErrorAndContinue(r.Context(), err)
				next.ServeHTTP(w, r)

				return
			}

			ctx := populateContextWithClientData(r.Context(), clientData, authContextFields)
			next.ServeHTTP(w, r.WithContext(ctx))
		},
	)
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

// populateContextWithClientData adds client data to request context
func populateContextWithClientData(
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
	ctx = context.WithValue(ctx, constants.ClientData, clientData)

	return ctx
}

// logErrorAndContinue logs client data errors with related context
func logErrorAndContinue(ctx context.Context, err error) {
	switch {
	case errors.Is(err, ErrNoClientDataHeader):
		log.Info(ctx, err.Error())
	case errors.Is(err, ErrMissingSignatureHeader):
		log.Warn(ctx, err.Error())
	case errors.Is(err, ErrPublicKeyNotFound):
		log.Warn(ctx, err.Error())
	default:
		log.Error(ctx, "Client data processing failed", err)
	}
}
