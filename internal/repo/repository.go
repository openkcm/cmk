package repo

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"maps"
	"slices"

	"github.com/google/uuid"

	"github.com/openkcm/cmk/internal/errs"
	"github.com/openkcm/cmk/internal/model"
	cmkcontext "github.com/openkcm/cmk/utils/context"
)

// TransactionFunc is func signature for ExecTransaction.
type TransactionFunc func(context.Context) error

// Repo defines an interface for Repository operations.
type Repo interface {
	Create(ctx context.Context, resource Resource) error
	List(ctx context.Context, resource Resource, result any, query Query) error
	Delete(ctx context.Context, resource Resource, query Query) (bool, error)
	First(ctx context.Context, resource Resource, query Query) (bool, error)
	Patch(ctx context.Context, resource Resource, query Query) (bool, error)
	Set(ctx context.Context, resource Resource) error
	Transaction(ctx context.Context, txFunc TransactionFunc) error
	Count(ctx context.Context, resource Resource, query Query) (int, error)
	OffboardTenant(ctx context.Context, tenantID string) error
}

// Resource defines the interface for Resource operations.
type Resource interface {
	IsSharedModel() bool
	TableName() string
}

// UniqueConstraintError represents an error caused by a violation of a unique constraint in the database.
type UniqueConstraintError struct {
	Detail string
}

// Error returns an error message describing the unique constraint violation.
func (u *UniqueConstraintError) Error() string {
	return "resource must be unique: " + u.Detail
}

const DefaultLimit = 100

var (
	ErrInvalidUUID         = errors.New("invalid UUID format")
	ErrNotFound            = errors.New("resource not found")
	ErrUniqueConstraint    = errors.New("unique constraint violation")
	ErrCreateResource      = errors.New("failed to create resource")
	ErrSetResource         = errors.New("failed to set resource")
	ErrUpdateResource      = errors.New("failed to update resource")
	ErrDeleteResource      = errors.New("failed to delete resource")
	ErrGetResource         = errors.New("failed to get resource")
	ErrTransaction         = errors.New("failed to execute transaction")
	ErrWithTenant          = errors.New("failed to use tenant from context")
	ErrTenantNotFound      = errors.New("tenant not found")
	ErrInvalidFieldName    = errors.New("invalid field name")
	ErrKeyConfigName       = errors.New("failed getting keyconfig name")
	ErrSystemProperties    = errors.New("failed getting system properties")
	ErrBatcherResourceType = errors.New("all resources should be of the same type")

	SQLNullBoolNull = sql.NullBool{Valid: false, Bool: true}
)

// LoadEntity is a type constraint for entities from the database
// that contain models with attributes that can be lazy loaded.
type LoadEntity interface {
	model.System |
		model.KeyConfiguration
}

type Pagination struct {
	Skip  int
	Top   int
	Count bool
}

type Opt[T LoadEntity] func(*T) error

// ToSharedModel is a generic function used to lazy load model values that are not stored in the database.
// It applies a series of Opt functions to the provided entity, allowing additional fields to be loaded as needed.
// Returns the modified entity or an error if any Opt function fails.
func ToSharedModel[T LoadEntity](v *T, opts ...Opt[T]) (*T, error) {
	for _, o := range opts {
		err := o(v)
		if err != nil {
			return nil, err
		}
	}

	return v, nil
}

func HasConnectedSystems(ctx context.Context, r Repo, keyConfigID uuid.UUID) (bool, error) {
	count, err := r.Count(
		ctx,
		&model.System{},
		*NewQuery().Where(
			NewCompositeKeyGroup(
				NewCompositeKey().Where(
					KeyConfigIDField, keyConfigID),
			),
		),
	)
	if err != nil {
		return true, err
	}

	return count > 0, nil
}

func GetSystemByIDWithProperties(ctx context.Context, r Repo, systemID uuid.UUID, query *Query) (*model.System, error) {
	query.Where(
		NewCompositeKeyGroup(
			NewCompositeKey().Where(
				fmt.Sprintf("%s.%s", model.System{}.TableName(), IDField), systemID),
		),
	)

	systems, _, err := ListAndCountSystemWithProperties(ctx, r, Pagination{}, query)
	if err != nil {
		return nil, errs.Wrap(ErrGetResource, err)
	}

	if len(systems) < 1 {
		return nil, ErrNotFound
	}

	return systems[0], nil
}

//nolint:funlen
func ListAndCountSystemWithProperties(
	ctx context.Context,
	r Repo,
	pagination Pagination,
	query *Query,
) ([]*model.System, int, error) {
	var systems []*model.System
	var count int

	systems, count, err := ListAndCount(ctx, r, pagination, model.System{}, query)
	if err != nil {
		return nil, 0, err
	}

	ck := NewCompositeKey()

	ck.IsStrict = false
	for _, s := range systems {
		ck = ck.Where(fmt.Sprintf("%s.%s", model.System{}.TableName(), IDField), s.ID)
	}

	loadQuery := query.
		Join(LeftJoin, JoinCondition{
			JoinTable: &model.SystemProperty{},
			JoinField: IDField,
			Table:     &model.System{},
			Field:     IDField,
		}).Join(LeftJoin, JoinCondition{
		JoinTable: &model.KeyConfiguration{},
		JoinField: IDField,
		Table:     &model.System{},
		Field:     KeyConfigIDField,
	}).Join(LeftJoin, JoinCondition{
		JoinTable: &model.Event{},
		JoinField: IdentifierField,
		Table:     &model.System{},
		Field:     IdentifierField,
	}).Select(
		// Get All System Fields
		NewSelectField(model.System{}.TableName(), QueryFunction{
			Function: AllFunc,
		}),
		// Get all Systems Props Fields
		NewSelectField(model.SystemProperty{}.TableName(), QueryFunction{
			Function: AllFunc,
		}),
		// Get KeyConfigName with alias so it's injected into System KeyConfigName
		NewSelectField(
			fmt.Sprintf("%s.%s", model.KeyConfiguration{}.TableName(), NameField),
			QueryFunction{},
		).SetAlias(SystemKeyconfigName),
		// Get ErrorMessage so it's injected into System ErrorMessage
		NewSelectField(
			fmt.Sprintf("%s.%s", model.Event{}.TableName(), ErrorMessageField),
			QueryFunction{},
		),
		// Get ErrorCode with alias so it's injected into System ErrorCode
		NewSelectField(
			fmt.Sprintf("%s.%s", model.Event{}.TableName(), ErrorCodeField),
			QueryFunction{},
		),
	).Where(
		NewCompositeKeyGroup(ck),
	).SetOffset(0).SetLimit(DefaultLimit) // Reset offset and limit as this is for the join table

	systemsMap := map[uuid.UUID]*model.System{}

	err = ProcessInBatch(ctx, r, loadQuery, DefaultLimit, func(rows []*model.JoinSystem) error {
		for _, row := range rows {
			sys, exists := systemsMap[row.ID]
			if !exists {
				sys = &row.System
				sys.Properties = map[string]string{}
				sys.KeyConfigurationName = row.KeyConfigurationName
				sys.ErrorCode = row.ErrorCode
				sys.ErrorMessage = row.ErrorMessage
				systemsMap[row.ID] = sys
			}

			if row.Key == "" {
				continue
			}

			sys.Properties[row.Key] = row.Value
		}

		return nil
	})
	if err != nil {
		return nil, 0, err
	}

	sys := slices.Collect(maps.Values(systemsMap))

	return sys, count, nil
}

// BatchProcessOptions configures how ProcessInBatchWithOptions should handle batch processing.
type BatchProcessOptions struct {
	// DeleteMode indicates that items are being deleted during processing.
	// When true, the offset is not incremented to avoid skipping records.
	DeleteMode bool
}

// ProcessInBatchWithOptions retrieves and processes records in batches based on the provided query parameters.
// It iterates through all matching records using pagination to avoid loading large datasets into memory.
// The processFunc is called on the records, allowing custom processing logic.
// Processing stops immediately if processFunc returns an error.
//
// Options:
//   - DeleteMode: When true, assumes items are being deleted during processing and keeps offset at 0
//     to avoid skipping records. This ensures all items are processed even as the total count decreases.
func ProcessInBatchWithOptions[T Resource](
	ctx context.Context,
	repo Repo,
	baseQuery *Query,
	batchSize int,
	options BatchProcessOptions,
	processFunc func([]*T) error,
) error {
	offset := 0

	for {
		var items []*T

		query := baseQuery.SetLimit(batchSize).SetOffset(offset)

		err := repo.List(ctx, *new(T), &items, *query)
		if err != nil {
			return err
		}

		err = processFunc(items)
		if err != nil {
			return err
		}

		// No more items to process
		if len(items) == 0 {
			break
		}

		// In delete mode, keep offset at 0 since items are removed from the beginning
		// In normal mode, increment offset to paginate through results
		if !options.DeleteMode {
			offset += batchSize
		}

		count, err := repo.Count(ctx, *new(T), *query)
		if err != nil {
			return err
		}
		// Stop if we've processed all items (only relevant in non-delete mode)
		if !options.DeleteMode && offset >= count {
			break
		}
	}

	return nil
}

// ProcessInBatch retrieves and processes records in batches from the database based on the provided query parameters.
// It iterates through all matching records using pagination to avoid loading large datasets into memory.
// The processFunc is called on the records, allowing custom processing logic.
// Processing stops immediately if processFunc returns an error.
//
// Note: If you are deleting items during processing, use ProcessInBatchWithOptions with DeleteMode enabled
// to avoid skipping records.
func ProcessInBatch[T Resource](
	ctx context.Context,
	repo Repo,
	baseQuery *Query,
	batchSize int,
	processFunc func([]*T) error,
) error {
	return ProcessInBatchWithOptions(ctx, repo, baseQuery, batchSize, BatchProcessOptions{}, processFunc)
}

// ListAndCount lists items paginated and returns total count of elements
// Total count is only returned if pagination.count is true
func ListAndCount[T Resource](
	ctx context.Context,
	r Repo,
	pagination Pagination,
	item T,
	query *Query,
) ([]*T, int, error) {
	var res []*T
	resource := *new(T)
	var top int

	if pagination.Top == 0 {
		top = DefaultLimit
	} else {
		top = pagination.Top
	}

	query = query.SetLimit(top).SetOffset(pagination.Skip)
	err := r.List(ctx, resource, &res, *query)
	if err != nil {
		return nil, 0, errs.Wrap(ErrListingItems, err)
	}
	if !pagination.Count {
		return res, 0, nil
	}

	count, err := r.Count(ctx, resource, *query)
	if err != nil {
		return nil, 0, errs.Wrap(ErrCountingItem, err)
	}
	return res, count, nil
}

func GetTenant(ctx context.Context, r Repo) (*model.Tenant, error) {
	tenantID, err := cmkcontext.ExtractTenantID(ctx)
	if err != nil {
		return nil, err
	}

	return GetTenantByID(ctx, r, tenantID)
}

func GetTenantByID(ctx context.Context, r Repo, tenantID string) (*model.Tenant, error) {
	tenant := &model.Tenant{}
	ck := NewCompositeKey().Where(IDField, tenantID)
	query := NewQuery().Where(NewCompositeKeyGroup(ck))

	_, err := r.First(ctx, tenant, *query)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return nil, ErrTenantNotFound
		}

		return nil, errs.Wrap(ErrGetResource, err)
	}

	return tenant, nil
}
