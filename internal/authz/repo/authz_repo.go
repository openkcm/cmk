package authz_repo

import (
	"context"
	"errors"

	"github.com/openkcm/cmk/internal/authz"
	authz_loader "github.com/openkcm/cmk/internal/authz/loader"
	"github.com/openkcm/cmk/internal/repo"
)

var (
	ErrUnauthorized = errors.New("action on resource unauthorized")
)

type AuthzRepo struct {
	repo        repo.Repo
	authzLoader *authz_loader.AuthzLoader[authz.RepoResourceTypeName, authz.RepoAction]
}

func NewAuthzRepo(
	repo repo.Repo, authzLoader *authz_loader.AuthzLoader[authz.RepoResourceTypeName,
		authz.RepoAction]) *AuthzRepo {
	return &AuthzRepo{
		repo:        repo,
		authzLoader: authzLoader,
	}
}

func (r *AuthzRepo) Create(
	ctx context.Context, resource repo.Resource) error {
	err := r.checkResourceAuthZ(ctx, resource, authz.RepoActionCreate)
	if err != nil {
		return err
	}
	return r.repo.Create(ctx, resource)
}

func (r *AuthzRepo) Count(
	ctx context.Context, resource repo.Resource, query repo.Query,
) (int, error) {
	err := r.checkResourceAuthZ(ctx, resource, authz.RepoActionCount)
	if err != nil {
		return 0, err
	}
	err = r.checkQueryAuthZ(ctx, query, authz.RepoActionCount)
	if err != nil {
		return 0, err
	}
	return r.repo.Count(ctx, resource, query)
}

func (r *AuthzRepo) OffboardTenant(ctx context.Context, tenantID string) error {
	return r.repo.OffboardTenant(ctx, tenantID)
}

func (r *AuthzRepo) List(
	ctx context.Context,
	resource repo.Resource,
	result any,
	query repo.Query,
) error {
	err := r.checkResourceAuthZ(ctx, resource, authz.RepoActionList)
	if err != nil {
		return err
	}
	err = r.checkQueryAuthZ(ctx, query, authz.RepoActionList)
	if err != nil {
		return err
	}
	return r.repo.List(ctx, resource, result, query)
}

func (r *AuthzRepo) Delete(
	ctx context.Context,
	resource repo.Resource,
	query repo.Query,
) (bool, error) {
	err := r.checkResourceAuthZ(ctx, resource, authz.RepoActionDelete)
	if err != nil {
		return false, err
	}
	err = r.checkQueryAuthZ(ctx, query, authz.RepoActionDelete)
	if err != nil {
		return false, err
	}
	return r.repo.Delete(ctx, resource, query)
}

func (r *AuthzRepo) First(
	ctx context.Context,
	resource repo.Resource,
	query repo.Query,
) (bool, error) {
	err := r.checkResourceAuthZ(ctx, resource, authz.RepoActionFirst)
	if err != nil {
		return false, err
	}
	err = r.checkQueryAuthZ(ctx, query, authz.RepoActionFirst)
	if err != nil {
		return false, err
	}
	return r.repo.First(ctx, resource, query)
}

func (r *AuthzRepo) Patch(
	ctx context.Context,
	resource repo.Resource,
	query repo.Query,
) (bool, error) {
	err := r.checkResourceAuthZ(ctx, resource, authz.RepoActionUpdate)
	if err != nil {
		return false, err
	}
	err = r.checkQueryAuthZ(ctx, query, authz.RepoActionUpdate)
	if err != nil {
		return false, err
	}
	return r.repo.Patch(ctx, resource, query)
}

func (r *AuthzRepo) Set(ctx context.Context, resource repo.Resource) error {
	err := r.checkResourceAuthZ(ctx, resource, authz.RepoActionDelete)
	if err != nil {
		return err
	}
	err = r.checkResourceAuthZ(ctx, resource, authz.RepoActionCreate)
	if err != nil {
		return err
	}
	return r.repo.Set(ctx, resource)
}

func (r *AuthzRepo) Transaction(ctx context.Context, txFunc repo.TransactionFunc) error {
	return r.repo.Transaction(ctx, txFunc)
}

func (r *AuthzRepo) checkResourceAuthZ(
	ctx context.Context, resource repo.Resource, action authz.RepoAction) error {
	err := r.authzLoader.LoadAllowList(ctx)
	if err != nil {
		return err
	}

	isAllowed, err := resource.CheckAuthz(ctx, r.authzLoader.AuthzHandler, action)
	if err != nil {
		return err
	}
	if !isAllowed {
		return ErrUnauthorized
	}
	return nil
}

func (r *AuthzRepo) checkQueryAuthZ(
	ctx context.Context, query repo.Query, action authz.RepoAction) error {
	err := r.authzLoader.LoadAllowList(ctx)
	if err != nil {
		return err
	}

	for _, join := range query.Joins {
		isAllowed, err := join.OnCondition.JoinTable.CheckAuthz(ctx, r.authzLoader.AuthzHandler, action)
		if err != nil {
			return err
		}
		if !isAllowed {
			return ErrUnauthorized
		}
	}
	return nil
}
