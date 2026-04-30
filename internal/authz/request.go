package authz

import (
	"context"
	"errors"
	"fmt"

	"github.com/openkcm/cmk/internal/constants"
	"github.com/openkcm/cmk/internal/errs"
	cmkcontext "github.com/openkcm/cmk/utils/context"
)

var (
	ErrEmptyRequest     = errors.New("empty request")
	ErrWrongTenantID    = errors.New("wrong tenant ID in request")
	ErrNoAuthzForSource = errors.New("incompatible source")
)

type UserRequest interface {
	IsEmpty() bool
	IsValidContext(ctx context.Context) error
}

type InternalUserRequest struct {
	Role constants.InternalRole
}

func (u InternalUserRequest) IsEmpty() bool {
	return u.Role == ""
}

func (r InternalUserRequest) IsValidContext(ctx context.Context) error {
	source, err := cmkcontext.ExtractSource(ctx)
	if err != nil {
		return err
	}
	if source != string(constants.InternalSource) {
		return ErrNoAuthzForSource
	}

	return nil
}

type BusinessUserRequest struct {
	TenantID TenantID
	UserName string
	Groups   []string
}

func (u BusinessUserRequest) IsEmpty() bool {
	return u.UserName == "" || len(u.Groups) == 0
}

func (r BusinessUserRequest) IsValidContext(ctx context.Context) error {
	// Get the tenant from the context
	tenant, err := cmkcontext.ExtractTenantID(ctx)
	if err != nil {
		return err
	}

	if r.TenantID != TenantID(tenant) {
		return ErrWrongTenantID
	}

	source, err := cmkcontext.ExtractSource(ctx)
	if err != nil {
		return err
	}
	if source != string(constants.BusinessSource) {
		return ErrNoAuthzForSource
	}

	return nil
}

type Request[TUser UserRequest, TResourceTypeName, TAction comparable] struct {
	ID               string            // required
	User             TUser             // required
	ResourceTypeName TResourceTypeName // optional
	Action           TAction           // optional
}

func (r Request[Tuser, TResourceTypeName, TAction]) IsValidContext(ctx context.Context) error {
	if r.User.IsEmpty() {
		return ErrEmptyRequest
	}

	err := r.User.IsValidContext(ctx)
	if err != nil {
		return err
	}
	return nil
}

func (r Request[Tuser, TResourceTypeName, TAction]) IsEmpty() bool {
	// Check if the request data is filled
	var emptyAction TAction
	var emptyResourceTypeName TResourceTypeName
	if r.ResourceTypeName == emptyResourceTypeName || r.Action == emptyAction {
		return true
	}
	if r.User.IsEmpty() {
		return true
	}
	return false
}

func (r Request[Tuser, TResourceTypeName, TAction]) GetResourceTypeNameString() string {
	return fmt.Sprintf("%v", r.ResourceTypeName)
}

func (r Request[Tuser, TResourceTypeName, TAction]) GetActionString() string {
	return fmt.Sprintf("%v", r.Action)
}

var (
	ErrValidation = errors.New("validation failed")
	ErrUserEmpty  = errors.New("user is empty")
)

func NewRequest[TUser UserRequest, TResourceTypeName, TAction comparable](
	ctx context.Context, user TUser,
	resourceTypeName TResourceTypeName, action TAction,
) (*Request[TUser, TResourceTypeName, TAction], error) {
	var req Request[TUser, TResourceTypeName, TAction]

	var err error

	req.User = user
	req.ResourceTypeName = resourceTypeName

	if user.IsEmpty() {
		return nil, errs.Wrap(ErrValidation, ErrUserEmpty)
	}
	req.User = user
	req.Action = action

	req.ID, err = cmkcontext.GetRequestID(ctx)
	if err != nil {
		return nil, err
	}

	return &req, nil
}
