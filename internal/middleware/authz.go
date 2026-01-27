package middleware

import (
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"github.com/openkcm/cmk/internal/api/write"
	"github.com/openkcm/cmk/internal/apierrors"
	"github.com/openkcm/cmk/internal/authz"
	"github.com/openkcm/cmk/internal/constants"
	"github.com/openkcm/cmk/internal/controllers/cmk"
	"github.com/openkcm/cmk/internal/log"
	cmkcontext "github.com/openkcm/cmk/utils/context"
)

// AuthzMiddleware is a middleware that checks authorization for incoming requests

//nolint:funlen
func AuthzMiddleware(
	ctr *cmk.APIController,
) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(
			func(w http.ResponseWriter, r *http.Request) {
				ctx := r.Context()

				log.Debug(ctx, "request pattern", slog.String("pattern", r.Pattern))
				pattern := strings.Replace(r.Pattern, constants.BasePath, "", 1)

				// Check if the API is on the allow list
				_, exists := authz.AllowListByAPI[pattern]

				// API is on the allow list, skip authorization check
				if exists {
					log.Debug(
						ctx, "request pattern is on allow list, skipping authz check", slog.String("pattern", pattern),
					)
					next.ServeHTTP(w, r)

					return
				}

				restriction, exists := authz.RestrictionsByAPI[pattern]
				if !exists {
					// If no matching requirement is found, deny access
					log.Warn(ctx, "No authz restriction found for API", slog.String("api", pattern))
					write.ErrorResponse(ctx, w, apierrors.OAPIValidatorErrorMessage("Forbidden", http.StatusForbidden))

					return
				}

				allowed, err := authz.CheckAuthz(
					ctx, ctr.AuthzEngine.AuthzHandler, restriction.ResourceTypeName, restriction.Action,
				)
				if err != nil {
					log.Debug(ctx, "check authz error", log.ErrorAttr(err))
				}

				log.Debug(ctx, "Authz result", slog.String("allowed", strconv.FormatBool(allowed)))

				// If authorization fails, attempt to load the allow list for the tenant and check again
				if !allowed {
					tenantID, extractErr := cmkcontext.ExtractTenantID(ctx)
					if extractErr != nil {
						log.Debug(ctx, "ExtractTenantID error", log.ErrorAttr(extractErr))
						write.ErrorResponse(
							ctx, w, apierrors.OAPIValidatorErrorMessage("Forbidden", http.StatusForbidden),
						)

						return
					}

					loadErr := ctr.AuthzEngine.LoadAllowList(ctx, tenantID)
					if loadErr != nil {
						log.Debug(ctx, "LoadAllowList error", log.ErrorAttr(loadErr))
						write.ErrorResponse(
							ctx, w, apierrors.OAPIValidatorErrorMessage("Forbidden", http.StatusForbidden),
						)

						return
					}

					// Retry authorization after allow list is loaded
					allowed, err = authz.CheckAuthz(
						ctx, ctr.AuthzEngine.AuthzHandler, restriction.ResourceTypeName, restriction.Action,
					)
					log.Debug(
						ctx, "Authz result", slog.String("allowed", strconv.FormatBool(allowed)),
					)

					if err != nil {
						log.Debug(ctx, "check authz error", log.ErrorAttr(err))
					}

					if !allowed {
						write.ErrorResponse(
							ctx, w, apierrors.OAPIValidatorErrorMessage("Forbidden", http.StatusForbidden),
						)

						return
					}

					next.ServeHTTP(w, r)

					return
				}

				next.ServeHTTP(w, r)
			},
		)
	}
}
