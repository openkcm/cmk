package authz

import (
	"context"
	"errors"
	"fmt"

	"github.com/openkcm/cmk/internal/errs"
	cmkcontext "github.com/openkcm/cmk/utils/context"
)

type User struct {
	UserName string
	Groups   []string
}
type Request[TResourceTypeName, TAction comparable] struct {
	ID               string            // required
	User             User              // required
	ResourceTypeName TResourceTypeName // optional
	Action           TAction           // optional
	TenantID         TenantID          // required
}

func (r Request[TResourceTypeName, TAction]) GetResourceTypeNameString() string {
	return fmt.Sprintf("%v", r.ResourceTypeName)
}

func (r Request[TResourceTypeName, TAction]) GetActionString() string {
	return fmt.Sprintf("%v", r.Action)
}

var (
	ErrValidation = errors.New("validation failed")
	ErrUserEmpty  = errors.New("user is empty")
)

func NewRequest[TResourceTypeName, TAction comparable](
	ctx context.Context, tenantID TenantID, user User, resourceTypeName TResourceTypeName, action TAction,
) (*Request[TResourceTypeName, TAction], error) {
	var req Request[TResourceTypeName, TAction]

	var err error

	req.TenantID = tenantID
	req.ResourceTypeName = resourceTypeName

	if user.UserName == "" || len(user.Groups) == 0 {
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
