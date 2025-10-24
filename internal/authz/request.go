package authz

import (
	"context"
	"errors"

	"github.com/openkcm/cmk-core/internal/errs"
	cmkcontext "github.com/openkcm/cmk-core/utils/context"
)

type User struct {
	UserName string
	Groups   []UserGroup
}
type Request struct {
	ID               string           // required
	User             User             // required
	ResourceTypeName ResourceTypeName // optional
	Action           Action           // optional
	TenantID         TenantID         // required
}

var (
	ErrValidation                  = errors.New("validation failed")
	ErrUserEmpty                   = errors.New("user is empty")
	ErrActionInvalid               = errors.New("action is invalid")
	ErrResourceTypeInvalid         = errors.New("resource type is invalid")
	ErrResourceTypeOrActionInvalid = errors.New("resource type or action is invalid")
)

func NewRequest(
	ctx context.Context, tenantID TenantID, user User, resourceTypeName ResourceTypeName, action Action,
) (*Request, error) {
	var req Request

	var err error

	req.TenantID = tenantID

	err = req.SetUser(user)
	if err != nil {
		return nil, err
	}

	err = req.SetResourceType(resourceTypeName)
	if err != nil {
		return nil, err
	}

	err = req.SetAction(action)
	if err != nil {
		return nil, err
	}

	req.ID, err = cmkcontext.GetRequestID(ctx)
	if err != nil {
		return nil, err
	}

	return &req, nil
}

func (ar *Request) SetAction(action Action) error {
	// empty Action is allowed
	if action == "" {
		ar.Action = action
		return nil
	}

	// if ResourceTypeName is not set, check only if the Action is valid
	if ar.ResourceTypeName == "" {
		isValid, err := isValidAction(action)
		if !isValid {
			return err
		}
	} else {
		isValid, err := isValidActionForResource(action, ar.ResourceTypeName)
		if !isValid {
			return err
		}
	}

	ar.Action = action

	return nil
}

func (ar *Request) SetResourceType(resourceTypeName ResourceTypeName) error {
	// empty ResourceTypeName is allowed
	if resourceTypeName == "" {
		ar.ResourceTypeName = resourceTypeName
		return nil
	}

	isValid, err := isValidResourceType(resourceTypeName)
	if !isValid {
		return err
	}

	// if Action is set, check if it is valid for the given ResourceTypeName
	if ar.Action != "" {
		isValid, err = isValidActionForResource(ar.Action, resourceTypeName)
		if !isValid {
			return err
		}
	}

	ar.ResourceTypeName = resourceTypeName

	return nil
}

func (ar *Request) SetUser(user User) error {
	// empty Username is not allowed
	// empty Groups is allowed
	if user.UserName == "" || len(user.Groups) == 0 {
		return errs.Wrap(ErrValidation, ErrUserEmpty)
	}

	ar.User = user

	return nil
}

func isValidActionForResource(action Action, resourceTypeName ResourceTypeName) (bool, error) {
	// Check if the Action is valid for the given resource type
	if actions, typeExists := ResourceTypeActions[resourceTypeName]; typeExists {
		if _, actionExists := actions[action]; actionExists {
			return true, nil
		}
	}

	return false, errs.Wrapf(ErrResourceTypeOrActionInvalid, string(action))
}

func isValidAction(action Action) (bool, error) {
	if _, exists := ActionResourceTypes[action]; exists {
		return true, nil
	}

	return false, errs.Wrapf(ErrActionInvalid, string(action))
}

func isValidResourceType(resourceType ResourceTypeName) (bool, error) {
	if _, exists := ResourceTypeActions[resourceType]; exists {
		return true, nil
	}

	return false, errs.Wrapf(ErrResourceTypeInvalid, string(resourceType))
}
