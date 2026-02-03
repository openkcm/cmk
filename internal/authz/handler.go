package authz

import (
	"context"
	"errors"

	"github.com/openkcm/cmk/internal/auditor"
	"github.com/openkcm/cmk/internal/constants"
	"github.com/openkcm/cmk/internal/errs"
	cmkcontext "github.com/openkcm/cmk/utils/context"
)

type TenantID string

type Entity struct {
	TenantID   TenantID
	Role       constants.Role
	UserGroups []string
}
type Handler struct {
	Entities          []Entity
	AuthorizationData AllowList
	Auditor           *auditor.Auditor
}

const EmptyTenantID = TenantID("")

var (
	ErrInvalidRequest        = errors.New("invalid request")
	ErrEmptyRequest          = errors.New("empty request")
	ErrAuthorizationDecision = errors.New("authorization decision error")
	ErrAuthorizationDenied   = errors.New("authorization denied")
	ErrWrongTenantID         = errors.New("wrong tenant ID in request")

	ErrExtractClientData  = errors.New("error extracting client data from context")
	ErrCreateAuthzRequest = errors.New("error creating authorization request")
	ErrExtractTenantID    = errors.New("error extracting tenant ID from context")
	ErrAuthzDecision      = errors.New("error making authorization decision")
)

var InfoAuthorizationPassed = "Authorization check passed"

func NewAuthorizationHandler(entities *[]Entity, auditor *auditor.Auditor) (*Handler, error) {
	authorizationData := &AllowList{}

	var err error

	// Create authorization data from entities
	if len(*entities) != 0 {
		authorizationData, err = NewAuthorizationData(*entities)
		if err != nil {
			return nil, err
		}
	}

	return &Handler{
		Entities:          *entities,
		AuthorizationData: *authorizationData,
		Auditor:           auditor,
	}, nil
}

// IsAllowed checks if the given User is allowed to perform the given Action on the given resource
func (as *Handler) IsAllowed(ctx context.Context, ar Request) (bool, error) {
	// Check if the request data is filled
	if ar.User.UserName == "" || ar.User.Groups == nil || ar.ResourceTypeName == "" || ar.Action == "" {
		// Deny
		LogDecision(ctx, ar, as.Auditor, false, Reason(ErrEmptyRequest.Error()))

		return false, errs.Wrap(ErrInvalidRequest, ErrEmptyRequest)
	}

	// Get the tenant from the context
	tenant, err := cmkcontext.ExtractTenantID(ctx)
	if err != nil {
		// Deny
		LogDecision(ctx, ar, as.Auditor, false, Reason(err.Error()))

		return false, errs.Wrap(ErrValidation, err)
	}

	if ar.TenantID != TenantID(tenant) {
		// Deny
		LogDecision(ctx, ar, as.Auditor, false, Reason(ErrWrongTenantID.Error()))

		return false, errs.Wrap(ErrAuthorizationDecision, ErrWrongTenantID)
	}

	for _, group := range ar.User.Groups {
		reqData := AuthorizationKey{
			TenantID:         ar.TenantID,
			UserGroup:        group,
			ResourceTypeName: ar.ResourceTypeName,
			Action:           ar.Action,
		}
		_, ok := as.AuthorizationData.AuthzKeys[reqData]

		if ok {
			// Allow
			LogDecision(ctx, ar, as.Auditor, true, Reason(InfoAuthorizationPassed))
			return true, nil
		}
	}

	// If no matching policy is found, deny authorization
	// Deny
	LogDecision(ctx, ar, as.Auditor, false, Reason(ErrAuthorizationDecision.Error()))

	return false, errs.Wrap(ErrAuthorizationDecision, ErrAuthorizationDenied)
}
