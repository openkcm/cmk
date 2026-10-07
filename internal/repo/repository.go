package repo

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sort"

	"github.com/google/uuid"

	cmkapi "github.com/openkcm/cmk/internal/api/cmk/generated"
	"github.com/openkcm/cmk/internal/authz"
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
	Set(ctx context.Context, resource Resource, query Query) error
	Transaction(ctx context.Context, txFunc TransactionFunc) error
	Count(ctx context.Context, resource Resource, query Query) (int, error)
	OffboardTenant(ctx context.Context, tenantID string) error
	GetFilterOptions(ctx context.Context, resource Resource, columns []Filter, query Query) error
}

// Resource defines the interface for Resource operations.
type Resource interface {
	IsSharedModel() bool
	TableName() string
	CheckAuthz(ctx context.Context,
		authzHandler *authz.Handler[authz.RepoResourceType, authz.RepoAction],
		action authz.RepoAction) (bool, error)
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

func HasConnectedKeys(ctx context.Context, r Repo, keyConfigID uuid.UUID) (bool, error) {
	count, err := r.Count(
		ctx,
		&model.Key{},
		*NewQuery().Where(
			NewCompositeKeyGroup(
				NewCompositeKey().Where(
					KeyConfigIDField, keyConfigID,
				),
			),
		),
	)
	if err != nil {
		return true, err
	}

	return count > 0, nil
}

func HasConnectedSystems(ctx context.Context, r Repo, keyConfigID uuid.UUID) (bool, error) {
	count, err := r.Count(
		ctx,
		&model.System{},
		*NewQuery().Where(
			NewCompositeKeyGroup(
				NewCompositeKey().Where(
					KeyConfigIDField, keyConfigID,
				),
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
				fmt.Sprintf("%s.%s", model.System{}.TableName(), IDField), systemID,
			),
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

// systemIDCompositeKey builds a non-strict composite key matching any of the
// given systems by their table-qualified ID.
func systemIDCompositeKey(systems []*model.System) CompositeKey {
	ck := NewCompositeKey()
	ck.IsStrict = false
	for _, s := range systems {
		ck = ck.Where(fmt.Sprintf("%s.%s", model.System{}.TableName(), IDField), s.ID)
	}

	return ck
}

func buildSystemPropertiesQuery(query *Query, ck CompositeKey) *Query {
	return query.Join(LeftJoin, JoinCondition{
		JoinTable: &model.SystemProperty{},
		JoinField: IDField,
		Table:     &model.System{},
		Field:     IDField,
	}).Join(LeftJoin, JoinCondition{
		JoinTable: &model.KeyConfiguration{},
		JoinField: IDField,
		Table:     &model.System{},
		Field:     KeyConfigIDField,
		Alias:     "key_config",
	}).Join(LeftJoin, JoinCondition{
		JoinTable: &model.KeyConfiguration{},
		JoinField: IDField,
		Table:     &model.System{},
		Field:     TargetKeyConfigIDField,
		Alias:     "target_key_config",
	}).Join(LeftJoin, JoinCondition{
		JoinTable: &model.Event{},
		JoinField: IdentifierField,
		Table:     &model.System{},
		Field:     IDField + "::text", // Safe cast to text to compare with event identifier
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
			fmt.Sprintf("%s.%s", "key_config", NameField),
			QueryFunction{},
		).SetAlias(SystemKeyconfigName),
		// Get TargetKeyConfigName with alias so it's injected into System TargetKeyConfigurationName
		NewSelectField(
			fmt.Sprintf("%s.%s", "target_key_config", NameField),
			QueryFunction{},
		).SetAlias(SystemTargetKeyconfigName),
		// Get ErrorMessage so it's injected into System ErrorMessage
		NewSelectField(
			fmt.Sprintf("%s.%s", model.Event{}.TableName(), ErrorMessageField),
			QueryFunction{},
		).SetAlias(ErrorMessageField),
		// Get ErrorCode with alias so it's injected into System ErrorCode
		NewSelectField(
			fmt.Sprintf("%s.%s", model.Event{}.TableName(), ErrorCodeField),
			QueryFunction{},
		).SetAlias(ErrorCodeField),
	).Where(
		NewCompositeKeyGroup(ck),
	).SetOffset(0).SetLimit(DefaultLimit) // Reset offset and limit as this is for the join table
}

func populateSystemWithProperties(
	systemsMap map[uuid.UUID]*model.System,
	row *model.JoinSystemAndProperties,
) *model.System {
	sys, exists := systemsMap[row.ID]
	if !exists {
		sys = &row.System
		sys.Properties = map[string]string{}
		sys.KeyConfigurationName = row.KeyConfigurationName
		sys.TargetKeyConfigurationName = row.TargetKeyConfigurationName
		sys.ErrorCode = row.ErrorCode
		sys.ErrorMessage = row.ErrorMessage
		systemsMap[row.ID] = sys
	}

	if row.Key != "" {
		sys.Properties[row.Key] = row.Value
	}

	return sys
}

//nolint:cyclop,funlen
func ListSystemGroupsAndSystem(
	ctx context.Context,
	r Repo,
	pagination Pagination,
	query *Query,
) ([]*model.SystemGroup, int, error) {
	systems, count, err := ListAndCount(ctx, r, pagination, model.System{}, query)
	if err != nil {
		return nil, 0, err
	}

	if len(systems) == 0 {
		return []*model.SystemGroup{}, count, nil
	}

	query = buildSystemPropertiesQuery(query, systemIDCompositeKey(systems))

	query = query.Join(LeftJoin, JoinCondition{
		JoinTable: &model.SystemGroup{},
		JoinField: IDField,
		Table:     &model.System{},
		Field:     SystemGroupID,
	}).Select(
		NewSelectField(
			fmt.Sprintf("%s.%s", model.SystemGroup{}.TableName(), NameField),
			QueryFunction{},
		).SetAlias("group_name"),
		NewSelectField(
			fmt.Sprintf("%s.%s", model.SystemGroup{}.TableName(), DescriptionField),
			QueryFunction{},
		).SetAlias("group_description"),
		NewSelectField(
			fmt.Sprintf("%s.%s", model.SystemGroup{}.TableName(), SuppressWarningField),
			QueryFunction{},
		).SetAlias("group_suppress_warning"),
	)

	systemsMap := map[uuid.UUID]*model.System{}
	systemsGroupMap := map[uuid.UUID]*model.SystemGroup{}

	err = ProcessInBatch(ctx, r, query, DefaultLimit, func(rows []*model.JoinSystemGroupsAndSystem) error {
		for _, row := range rows {
			sys := populateSystemWithProperties(systemsMap, &row.JoinSystemAndProperties)

			gid := uuid.Nil
			if sys.SystemGroupID != nil {
				gid = *sys.SystemGroupID
			}

			if _, ok := systemsGroupMap[gid]; !ok {
				group := row.SystemGroup
				group.ID = gid
				if gid == uuid.Nil {
					group.Name = "root"
				}
				systemsGroupMap[gid] = &group
			}
		}

		return nil
	})
	if err != nil {
		return nil, 0, err
	}

	for _, s := range systemsMap {
		gid := uuid.Nil
		if s.SystemGroupID != nil {
			gid = *s.SystemGroupID
		}
		systemsGroupMap[gid].Systems = append(systemsGroupMap[gid].Systems, s)
	}

	if err := fillWarnCoverage(ctx, r, systemsGroupMap); err != nil {
		return nil, 0, err
	}

	out := make([]*model.SystemGroup, 0, len(systemsGroupMap))
	for _, g := range systemsGroupMap {
		out = append(out, g)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].ID.String() > out[j].ID.String()
	})
	return out, count, nil
}

func fillWarnCoverage(ctx context.Context, r Repo, groups map[uuid.UUID]*model.SystemGroup) error {
	warnExpr := fmt.Sprintf(
		"bool_or(%s.%s = '%s') AND bool_or(%s.%s = '%s') AND NOT bool_or(COALESCE(%s.%s, false))",
		model.System{}.TableName(), StatusField, cmkapi.SystemStatusCONNECTED,
		model.System{}.TableName(), StatusField, cmkapi.SystemStatusDISCONNECTED,
		model.SystemGroup{}.TableName(), SuppressWarningField,
	)

	query := NewQuery().
		Join(LeftJoin, JoinCondition{
			JoinTable: &model.SystemGroup{},
			JoinField: IDField,
			Table:     &model.System{},
			Field:     SystemGroupID,
		}).
		Select(
			NewSelectField(fmt.Sprintf("%s.%s", model.System{}.TableName(), SystemGroupID), QueryFunction{}),
			NewSelectField(warnExpr, QueryFunction{}).SetAlias("warn_coverage"),
		).
		GroupBy(fmt.Sprintf("%s.%s", model.System{}.TableName(), SystemGroupID)).
		Order(OrderField{Field: SystemGroupID, Direction: Asc})

	return ProcessInBatch(ctx, r, query, DefaultLimit, func(rows []*model.SystemGroupCoverage) error {
		for _, row := range rows {
			gid := uuid.Nil
			if row.SystemGroupID != nil {
				gid = *row.SystemGroupID
			}
			group, ok := groups[gid]
			if ok {
				group.WarnCoverage = row.WarnCoverage
			}
		}

		return nil
	})
}

func ListAndCountSystemWithProperties(
	ctx context.Context,
	r Repo,
	pagination Pagination,
	query *Query,
) ([]*model.System, int, error) {
	systems, count, err := ListAndCount(ctx, r, pagination, model.System{}, query)
	if err != nil {
		return nil, 0, err
	}

	if len(systems) == 0 {
		return []*model.System{}, count, nil
	}

	query = buildSystemPropertiesQuery(query, systemIDCompositeKey(systems))

	systemsMap := map[uuid.UUID]*model.System{}

	err = ProcessInBatch(ctx, r, query, DefaultLimit, func(rows []*model.JoinSystemAndProperties) error {
		for _, row := range rows {
			populateSystemWithProperties(systemsMap, row)
		}

		return nil
	})
	if err != nil {
		return nil, 0, err
	}

	// Iterate over the original slice to preserve order
	sys := make([]*model.System, 0, len(systemsMap))
	for _, s := range systems {
		if enrichedSys, exists := systemsMap[s.ID]; exists {
			sys = append(sys, enrichedSys)
		}
	}

	return sys, count, nil
}

// BatchProcessOptions configures how ProcessInBatchWithOptions should handle batch processing.
type BatchProcessOptions struct {
	// DeleteMode indicates that items are being deleted during processing.
	// When true, the offset is not incremented to avoid skipping records.
	DeleteMode     bool
	IgnoreFailMode bool
}

// ProcessInBatchWithOptions retrieves and processes records in batches based on the provided query parameters.
// It iterates through all matching records using pagination to avoid loading large datasets into memory.
// The processFunc is called on the records, allowing custom processing logic.
// By default processing stops immediately if processFunc returns an error; see IgnoreFailMode below.
//
// Options:
//   - DeleteMode: When true, assumes items are being deleted during processing and keeps offset at 0
//     to avoid skipping records. This ensures all items are processed even as the total count decreases.
//   - IgnoreFailMode: When true, a processFunc error does not stop the batch loop; processing
//     continues through the remaining records and the last error encountered is returned at the end.
//     When false (default), the first processFunc error is returned immediately.
func ProcessInBatchWithOptions[T Resource](
	ctx context.Context,
	repo Repo,
	baseQuery *Query,
	batchSize int,
	options BatchProcessOptions,
	processFunc func([]*T) error,
) error {
	offset := 0
	var lastError error
	for {
		var items []*T

		query := baseQuery.SetLimit(batchSize).SetOffset(offset)

		err := repo.List(ctx, *new(T), &items, *query)
		if err != nil {
			return err
		}

		err = processFunc(items)
		if err != nil {
			lastError = err
			if !options.IgnoreFailMode {
				return err
			}
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

	return lastError
}

func GetKeyConfigPrimaryKey(ctx context.Context, r Repo, keyConfigID uuid.UUID) (*uuid.UUID, error) {
	keyConfig := &model.KeyConfiguration{
		ID: keyConfigID,
	}

	_, err := r.First(
		ctx,
		keyConfig,
		*NewQuery(),
	)

	if err != nil && !errors.Is(err, ErrNotFound) {
		return nil, err
	}
	return keyConfig.PrimaryKeyID, nil
}

func IsPrimaryKey(ctx context.Context, r Repo, key *model.Key) (bool, error) {
	pkeyID, err := GetKeyConfigPrimaryKey(ctx, r, key.KeyConfigurationID)
	if err != nil || pkeyID == nil {
		return false, err
	}

	return *pkeyID == key.ID, nil
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
