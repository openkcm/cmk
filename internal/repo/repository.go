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
)

// TransactionFunc is func signature for ExecTransaction.
type TransactionFunc func(context.Context, Repo) error

// Repo defines an interface for Repository operations.
type Repo interface {
	Create(ctx context.Context, resource Resource) error
	List(ctx context.Context, resource Resource, result any, query Query) (int, error)
	Delete(ctx context.Context, resource Resource, query Query) (bool, error)
	First(ctx context.Context, resource Resource, query Query) (bool, error)
	Patch(ctx context.Context, resource Resource, query Query) (bool, error)
	Set(ctx context.Context, resource Resource) error
	Transaction(ctx context.Context, txFunc TransactionFunc) error
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
	ErrInvalidUUID      = errors.New("invalid UUID format")
	ErrNotFound         = errors.New("resource not found")
	ErrUniqueConstraint = errors.New("unique constraint violation")
	ErrCreateResource   = errors.New("failed to create resource")
	ErrUpdateResource   = errors.New("failed to update resource")
	ErrDeleteResource   = errors.New("failed to delete resource")
	ErrGetResource      = errors.New("failed to get resource")
	ErrTransaction      = errors.New("failed to execute transaction")
	ErrWithTenant       = errors.New("failed to use tenant from context")
	ErrTenantNotFound   = errors.New("tenant not found")
	ErrInvalidFieldName = errors.New("invalid field name")
	ErrKeyConfigName    = errors.New("failed getting keyconfig name")
	ErrSystemProperties = errors.New("failed getting system properties")

	SQLNullBoolNull = sql.NullBool{Valid: false, Bool: true}
)

// LoadEntity is a type constraint for entities from the database
// that contain models with attributes that can be lazy loaded.
type LoadEntity interface {
	model.System |
		model.KeyConfiguration
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
	var sys []*model.System

	count, err := r.List(
		ctx,
		model.System{},
		&sys,
		*NewQuery().Where(
			NewCompositeKeyGroup(
				NewCompositeKey().Where(
					KeyConfigIDField, keyConfigID),
			),
		).SetLimit(0),
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

	systems, _, err := ListSystemWithProperties(ctx, r, query)
	if err != nil {
		return nil, errs.Wrap(ErrGetResource, err)
	}

	if len(systems) < 1 {
		return nil, ErrNotFound
	}

	return systems[0], nil
}

func ListSystemWithProperties(ctx context.Context, r Repo, query *Query) ([]*model.System, int, error) {
	loadQuery := *query
	loadQuery = *loadQuery.
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
	)

	var rows []*model.JoinSystem
	// Count is ignored because it returns N systems + M system_props
	_, err := r.List(ctx, &model.System{}, &rows, loadQuery)
	if err != nil {
		return nil, 0, err
	}

	systemsMap := map[uuid.UUID]*model.System{}

	for _, row := range rows {
		sys, exists := systemsMap[row.ID]
		if !exists {
			sys = &row.System
			sys.Properties = map[string]string{}
			sys.KeyConfigurationName = row.KeyConfigurationName
			systemsMap[row.ID] = sys
		}

		if row.Key == "" {
			continue
		}

		sys.Properties[row.Key] = row.Value
	}

	systems := slices.Collect(maps.Values(systemsMap))

	// Get correct system count
	count, err := r.List(ctx, &model.System{}, &[]*model.System{}, *query.SetLimit(1))
	if err != nil {
		return nil, 0, err
	}

	return systems, count, nil
}

// ProcessInBatch retrieves and processes records in batches from the database based on the provided query parameters.
// It iterates through all matching records using pagination to avoid loading large datasets into memory.
// The processFunc is called on the records, allowing custom processing logic.
// Processing stops immediately if processFunc returns an error.
func ProcessInBatch[T Resource](
	ctx context.Context,
	repo Repo,
	baseQuery *Query,
	batchSize int,
	processFunc func([]*T) error,
) error {
	offset := 0

	for {
		var items []*T

		query := baseQuery.SetLimit(batchSize).SetOffset(offset)

		count, err := repo.List(ctx, *new(T), &items, *query)
		if err != nil {
			return err
		}

		err = processFunc(items)
		if err != nil {
			return err
		}

		offset += batchSize

		if offset >= count {
			break
		}
	}

	return nil
}
