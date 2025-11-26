package sql

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	multitenancy "github.com/bartventer/gorm-multitenancy/v8"

	"github.com/openkcm/cmk/internal/errs"
	"github.com/openkcm/cmk/internal/log"
	"github.com/openkcm/cmk/internal/model"
	"github.com/openkcm/cmk/internal/repo"
	"github.com/openkcm/cmk/internal/repo/violations"
	cmkcontext "github.com/openkcm/cmk/utils/context"
)

const (
	pqUniqueViolationErrCode = "23505" // see https://www.postgresql.org/docs/14/errcodes-appendix.html
	PublicSchema             = "public"
)

var (
	ErrPatchForeign              = errors.New("failed patching foreign key entity")
	ErrUnsupportedOrderDirective = errors.New("unsupported order directive")
)

// ResourceRepository represents the repository for managing Resource data.
type ResourceRepository struct {
	db *multitenancy.DB
}

// NewRepository creates and returns a new instance of ResourceRepository.
func NewRepository(db *multitenancy.DB) *ResourceRepository {
	return &ResourceRepository{
		db: db,
	}
}

// WithTenant runs GORM actions for a specific tenant
//
//nolint:cyclop
func (r *ResourceRepository) WithTenant(
	ctx context.Context,
	resource repo.Resource,
	fn func(tx *multitenancy.DB) error,
) error {
	var schemaName string

	if resource.IsSharedModel() {
		schemaName = PublicSchema
	} else {
		tenant, err := cmkcontext.ExtractTenantID(ctx)
		if err != nil {
			return errs.Wrap(repo.ErrWithTenant, err)
		}

		var existingTenant model.Tenant

		err = r.db.Where(repo.IDField+" = ?", tenant).First(&existingTenant).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return repo.ErrTenantNotFound
		} else if err != nil {
			return errs.Wrap(repo.ErrWithTenant, err)
		}

		schemaName = existingTenant.SchemaName
	}

	committer, ok := r.db.Statement.ConnPool.(gorm.TxCommitter)
	if committer != nil && ok {
		// If the connection pool is a TxCommitter, we are in a transaction.
		// We don't need to start a new transaction.
		reset, err := r.db.UseTenant(ctx, schemaName)

		defer func() {
			if reset != nil {
				resetErr := reset()
				if resetErr != nil {
					log.Error(ctx, "error resetting tenant", resetErr)
				}
			}
		}()

		if err != nil {
			return errs.Wrap(repo.ErrWithTenant, err)
		}

		return fn(r.db)
	}

	var err error

	txErr := r.db.WithTenant(
		ctx, schemaName, func(tx *multitenancy.DB) error {
			err = fn(tx)
			return err
		},
	)
	if txErr != nil {
		return errs.Wrap(repo.ErrTransaction, err)
	}

	return err
}

// Create adds meta information and stores a Resource.
func (r *ResourceRepository) Create(ctx context.Context, resource repo.Resource) error {
	return r.WithTenant(
		ctx, resource, func(tx *multitenancy.DB) error {
			err := tx.WithContext(ctx).Create(resource).Error
			if err != nil {
				log.Error(ctx, "error creating resource", err)

				if errors.Is(err, gorm.ErrDuplicatedKey) || violations.IsUniqueConstraint(err) {
					return errs.Wrap(repo.ErrUniqueConstraint, err)
				}

				return errs.Wrap(repo.ErrCreateResource, err)
			}

			return nil
		},
	)
}

// List retrieves records from the database based on the provided query parameters and model.
// Result is an address
func (r *ResourceRepository) List(
	ctx context.Context,
	resource repo.Resource,
	result any,
	query repo.Query,
) (int, error) {
	var count int64

	err := r.WithTenant(
		ctx, resource, func(tx *multitenancy.DB) error {
			db := applySelectQuery(tx.Model(result), query)

			db, err := applyQuery(db, query)
			if err != nil {
				return err
			}

			db = db.Count(&count)
			if db.Error != nil {
				return db.Error
			}

			for _, order := range query.OrderFields {
				switch order.Direction {
				case repo.Desc:
					db = db.Order(order.Field + " desc")
				case repo.Asc:
					db = db.Order(order.Field + " asc")
				default:
					return ErrUnsupportedOrderDirective
				}
			}

			res := applyPagination(db, query).Find(result)
			if res.Error != nil {
				return res.Error
			}

			return nil
		},
	)
	if err != nil {
		return 0, err
	}

	return int(count), nil
}

// Delete removes the Resource.
//
// It returns true if a record was deleted successfully,
// false if there was no record to delete,
// and error if there was an error during the deletion.
// If no query is provided it deletes the item by the primaryKey
func (r *ResourceRepository) Delete(
	ctx context.Context,
	resource repo.Resource,
	query repo.Query,
) (bool, error) {
	var result *gorm.DB

	err := r.WithTenant(
		ctx, resource, func(tx *multitenancy.DB) error {
			db, err := applyQuery(
				tx.Clauses(clause.Returning{}),
				query,
			)
			if err != nil {
				return err
			}

			result = applyPagination(db, query).Delete(resource)

			if result.Error != nil {
				log.Error(ctx, "error deleting resource", result.Error)
				return errs.Wrap(repo.ErrDeleteResource, result.Error)
			}

			return nil
		},
	)
	if err != nil {
		return false, err
	}

	return result.RowsAffected > 0, nil
}

// First fill given Resource with data, if found. Given Resource is used as query data.
// It will find the resource with the primary key as the where condition by omition
func (r *ResourceRepository) First(
	ctx context.Context,
	resource repo.Resource,
	query repo.Query,
) (bool, error) {
	var res *gorm.DB

	err := r.WithTenant(
		ctx, resource, func(tx *multitenancy.DB) error {
			dbQuery := applySelectQuery(tx.Model(resource), query)

			db, err := applyQuery(dbQuery, query)
			if err != nil {
				return err
			}

			dbQuery = applyPagination(db, query)

			res = dbQuery.First(resource)

			if res.Error != nil {
				log.Error(ctx, "error finding the resource", res.Error)

				if errors.Is(res.Error, gorm.ErrRecordNotFound) {
					return errs.Wrap(repo.ErrNotFound, res.Error)
				}

				return errs.Wrap(repo.ErrGetResource, res.Error)
			}

			return nil
		},
	)
	if err != nil {
		return false, err
	}

	return res.RowsAffected > 0, nil
}

// Patch will patch the resource with primary key as the where condition.
//
// It returns true if a record was patched successfully,
// and error if there was an error during the patch.
func (r *ResourceRepository) Patch(
	ctx context.Context,
	resource repo.Resource,
	query repo.Query,
) (bool, error) {
	var res *gorm.DB

	err := r.WithTenant(
		ctx, resource, func(tx *multitenancy.DB) error {
			res = tx.Model(resource)
			if query.Association.IsValid() {
				err := r.patchForeign(res, query.Association)
				if err != nil {
					return err
				}
			}

			res = applyUpdateQuery(
				res.Clauses(clause.Returning{}),
				query,
			).Updates(resource)

			db, err := applyQuery(res, query)
			if err != nil {
				return err
			}

			res = applyPagination(db, query)

			err = res.Error
			if err != nil {
				log.Error(ctx, "error updating resource", err)

				if errors.Is(err, gorm.ErrRecordNotFound) {
					return errs.Wrap(repo.ErrNotFound, err)
				}

				if violations.IsUniqueConstraint(err) ||
					errors.Is(err, gorm.ErrDuplicatedKey) {
					return errs.Wrap(repo.ErrUniqueConstraint, err)
				}

				return err
			}

			return nil
		},
	)
	if err != nil {
		return false, errs.Wrap(repo.ErrUpdateResource, err)
	}

	return res.RowsAffected > 0, nil
}

// Set will create an item or update it if it already exists
// It returns an error if there was an error during the operation
func (r *ResourceRepository) Set(ctx context.Context, resource repo.Resource) error {
	return r.WithTenant(
		ctx, resource, func(tx *multitenancy.DB) error {
			err := tx.Clauses(
				clause.OnConflict{
					UpdateAll: true,
				},
			).Create(resource).Error
			if err != nil {
				log.Error(ctx, "error setting the resource", err)
				return errs.Wrap(repo.ErrSetResource, err)
			}

			return nil
		},
	)
}

// Transaction wraps a function inside a database transaction.
// txFunc is a type TransactionFunc where we can define the transactional logic.
// if txFunc return no error then transaction is committed,
// else if txFunc return error then transaction is rolled back.
// Note: please dont use Goroutines inside the txFunc as this might lead to panic.
func (r *ResourceRepository) Transaction(ctx context.Context, txFunc repo.TransactionFunc) error {
	err := r.db.Transaction(
		func(db *multitenancy.DB) error {
			errorChan := make(chan error)

			go func() {
				errorChan <- txFunc(
					ctx,
					NewRepository(db),
				)
			}()

			select {
			case <-ctx.Done():
				return ctx.Err()
			case err := <-errorChan:
				return err
			}
		},
	)
	if err != nil {
		return errs.Wrap(repo.ErrTransaction, err)
	}

	return nil
}

func (r *ResourceRepository) Migrate(ctx context.Context, schemaName string) error {
	err := r.db.MigrateTenantModels(ctx, schemaName)
	if err != nil {
		return errs.Wrap(repo.ErrMigratingTenantModels, err)
	}

	return nil
}

func applySelectQuery(db *gorm.DB, query repo.Query) *gorm.DB {
	if len(query.SelectFields) > 0 {
		fields := make([]string, 0, len(query.SelectFields))

		for _, f := range query.SelectFields {
			fields = append(fields, f.SelectStatement())
		}

		s := strings.Join(fields, ",")
		db = db.Select(s)

		if len(query.Group) > 0 {
			sel := strings.Join(query.Group, ",")
			db = db.Group(sel)
		}
	}

	return db
}

// apply update operations on the db action
//
//nolint:unqueryvet
func applyUpdateQuery(db *gorm.DB, query repo.Query) *gorm.DB {
	if query.UpdateFields.All {
		db = db.Select("*")
	}

	if !query.UpdateFields.All && len(query.UpdateFields.Fields) > 0 {
		sel := strings.Join(query.UpdateFields.Fields, ",")
		db = db.Select(sel)
	}

	return db
}

func (r *ResourceRepository) patchForeign(db *gorm.DB, assoc repo.Association) error {
	err := db.Association(assoc.Field).Replace(assoc.Value)
	if err != nil {
		return errs.Wrap(ErrPatchForeign, err)
	}

	return nil
}

// applyQuery applies the query to the database.
func applyQuery(db *gorm.DB, query repo.Query) (*gorm.DB, error) {
	if len(query.Joins) > 0 {
		for _, join := range query.Joins {
			db = db.Joins(join.JoinStatement())
		}
	}

	if len(query.CompositeKeyGroup) > 0 {
		baseQuery := db.Session(&gorm.Session{NewDB: true})

		for i, ck := range query.CompositeKeyGroup {
			tk, err := handleCompositeKey(db, ck.CompositeKey)
			if err != nil {
				return nil, err
			}

			if i == 0 {
				baseQuery = baseQuery.Where(tk)
				continue
			}

			if ck.IsStrict {
				baseQuery = baseQuery.Where(tk)
			} else {
				baseQuery = baseQuery.Or(tk)
			}
		}

		db = db.Where(baseQuery)
	}

	if len(query.PreloadModel) > 0 {
		for _, pr := range query.PreloadModel {
			db = db.Preload(pr)
		}
	}

	return db, nil
}

func applyPagination(db *gorm.DB, query repo.Query) *gorm.DB {
	if query.Limit <= 0 {
		query.Limit = repo.DefaultLimit
	}

	return db.Offset(query.Offset).Limit(query.Limit)
}

// handleCompositeKey applies the composite key to the query.
func handleCompositeKey(db *gorm.DB, compositeKey repo.CompositeKey) (*gorm.DB, error) {
	tx := db.Session(&gorm.Session{NewDB: true})

	for _, cond := range compositeKey.Conds {
		entry := cond.Value
		if entry.Err != nil {
			return nil, entry.Err
		}

		tx = applyFieldCondition(tx, cond.Field, entry.Key, compositeKey.IsStrict)
	}

	return tx, nil
}

func applyFieldCondition(tx *gorm.DB, field string, key repo.Key, isStrict bool) *gorm.DB {
	switch key.Operation {
	case repo.GreaterThan, repo.LessThan:
		return applyCondition(tx, field, string(key.Operation), key.Value, isStrict)
	case repo.Equal:
		return applyFieldEqualCondition(tx, field, key, isStrict)
	}

	return nil
}

func applyFieldEqualCondition(tx *gorm.DB, field string, key repo.Key, isStrict bool) *gorm.DB {
	switch key.Value {
	case repo.NotEmpty:
		return tx.Where(field+" IS NOT NULL").Where(field+" != ?", "")
	case repo.Empty:
		return tx.Where(field+" IS NULL OR "+field+" = ?", "")
	case repo.FalseNull:
		return tx.Where(field+" IS NULL OR "+field+" = ?", false)
	default:
		v := reflect.ValueOf(key.Value)
		isSlice := (v.Kind() == reflect.Slice || v.Kind() == reflect.Array) && v.Type() != reflect.TypeFor[uuid.UUID]()

		if isSlice {
			return applyCondition(tx, field, "IN", key.Value, isStrict)
		}

		return applyCondition(tx, field, "=", key.Value, isStrict)
	}
}

func applyCondition(tx *gorm.DB, field, operator string, value any, isStrict bool) *gorm.DB {
	if isStrict {
		return tx.Where(fmt.Sprintf("%s %s (?)", field, operator), value)
	}

	return tx.Or(fmt.Sprintf("%s %s ?", field, operator), value)
}
