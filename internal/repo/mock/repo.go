package mock

import (
	"context"
	"reflect"

	"github.com/bartventer/gorm-multitenancy/middleware/nethttp/v8"

	"github.com/openkcm/cmk/internal/errs"
	"github.com/openkcm/cmk/internal/repo"
	cmkcontext "github.com/openkcm/cmk/utils/context"
)

// InMemoryRepository represents the repository for managing mock Resource data.
type InMemoryRepository struct {
	db *InMemoryMultitenancyDB
}

// NewInMemoryRepository creates and returns a new instance of InMemoryRepository.
func NewInMemoryRepository() *InMemoryRepository {
	return &InMemoryRepository{
		db: NewInMemoryMultitenancyDB(),
	}
}

// WithTenant runs actions for a specific tenant
func (r *InMemoryRepository) WithTenant(
	ctx context.Context,
	resource repo.Resource,
) (*InMemoryDB, error) {
	if resource.IsSharedModel() {
		ctx = context.WithValue(ctx, nethttp.TenantKey, "public")
	}

	tenant, err := cmkcontext.ExtractTenantID(ctx)
	if err != nil {
		return nil, err
	}

	return r.db.GetDB(tenant), nil
}

// Create adds meta information and stores a Resource.
func (r *InMemoryRepository) Create(
	ctx context.Context,
	resource repo.Resource,
) error {
	tenantDB, err := r.WithTenant(ctx, resource)
	if err != nil {
		return err
	}

	err = tenantDB.Create(resource)
	if err != nil {
		return err
	}

	return nil
}

// List retrieves records from the database based on the provided query parameters and model.
//

func (r *InMemoryRepository) List(
	ctx context.Context,
	resource repo.Resource,
	result any,
	_ repo.Query,
) (int, error) {
	tenantDB, err := r.WithTenant(ctx, resource)
	if err != nil {
		return 0, err
	}

	results, count := tenantDB.GetAll(result)

	err = assignList(result, results)
	if err != nil {
		return 0, err
	}

	return count, nil
}

// Delete removes the Resource
//
// It returns true if a record was deleted successfully.
// false if there was no record to delete
func (r *InMemoryRepository) Delete(
	ctx context.Context,
	resource repo.Resource,
	_ repo.Query,
) (bool, error) {
	tenantDB, err := r.WithTenant(ctx, resource)
	if err != nil {
		return false, err
	}

	err = tenantDB.Delete(resource)
	if err != nil {
		return false, errs.Wrap(ErrRepoDelete, err)
	}

	return true, nil
}

func (r *InMemoryRepository) First(
	ctx context.Context,
	resource repo.Resource,
	_ repo.Query,
) (bool, error) {
	tenantDB, err := r.WithTenant(ctx, resource)
	if err != nil {
		return false, err
	}

	result, err := tenantDB.Get(resource)
	if err != nil {
		return false, errs.Wrap(ErrRepoFirst, err)
	}

	reflect.ValueOf(resource).Elem().Set(reflect.ValueOf(result))

	return true, nil
}

// Patch will patch the resource with primary key as the were condition.
func (r *InMemoryRepository) Patch(
	ctx context.Context,
	resource repo.Resource,
	_ repo.Query,
) (bool, error) {
	tenantDB, err := r.WithTenant(ctx, resource)
	if err != nil {
		return false, err
	}

	exist, err := r.First(ctx, resource, repo.Query{})
	if exist {
		return true, tenantDB.Update(resource)
	}

	return false, errs.Wrap(ErrRepoPatch, err)
}

// Set will create an item or update it if it already exists
func (r *InMemoryRepository) Set(
	ctx context.Context,
	resource repo.Resource,
) error {
	tenantDB, err := r.WithTenant(ctx, resource)
	if err != nil {
		return err
	}

	exist, _ := r.First(ctx, resource, repo.Query{})
	if exist {
		return tenantDB.Update(resource)
	}

	return tenantDB.Create(resource)
}

// Count returns the number of records that match the given query
func (r *InMemoryRepository) Count(
	ctx context.Context,
	resource repo.Resource,
	_ repo.Query,
) (int, error) {
	tenantDB, err := r.WithTenant(ctx, resource)
	if err != nil {
		return 0, err
	}

	_, count := tenantDB.GetAll(resource)

	return count, nil
}

// Transaction will give transaction locking on particular rows.
func (r *InMemoryRepository) Transaction(
	ctx context.Context,
	txFunc repo.TransactionFunc,
) error {
	txInMemoryRepository := *r

	err := txFunc(ctx)
	if err != nil {
		return ErrTransactionFailed
	}

	r.db = txInMemoryRepository.db

	return nil
}

func (r *InMemoryRepository) OffboardTenant(_ context.Context, schemaName string) error {
	delete(r.db.databases, schemaName)
	return nil
}

func assignList(result any, list []repo.Resource) error {
	resultVal := reflect.ValueOf(result)
	if resultVal.Kind() != reflect.Ptr {
		return ErrMustPointerToSlice
	}

	sliceVal := resultVal.Elem()
	if sliceVal.Kind() != reflect.Slice {
		return ErrMustBeSlice
	}

	elemType := sliceVal.Type().Elem()
	newSlice := reflect.MakeSlice(reflect.SliceOf(elemType), 0, len(list))

	for _, item := range list {
		itemVal := reflect.ValueOf(item)

		if !itemVal.Type().AssignableTo(elemType) {
			return ErrItemNotAssignable
		}

		newSlice = reflect.Append(newSlice, itemVal)
	}

	resultVal.Elem().Set(newSlice)

	return nil
}
