package odata

import (
	"github.com/google/uuid"

	"github.com/openkcm/cmk/internal/constants"
	"github.com/openkcm/cmk/internal/repo"
	"github.com/openkcm/cmk/utils/ptr"
)

// QueryOdataMapper maps odata fields and parameters to repo queries. It implements the
// repo.QueryMapper interface, which describes mapping of generic data to repo query parameters.
type QueryOdataMapper struct {
	filterSchema FilterSchema
	skip         *int
	top          *int

	// The following are internal datas constructed from the parsed odata filter string.
	// There is redundancy here. So that we have both quick look ups when getting values
	// by field and to ensure deterministic query ordering.
	parseFilterFields []string
	parseFilterValues []any
	parseFilterMap    map[string]any
}

// NewQueryOdataMapper returns a new QueryOdataMapper with the provided filter schema.
func NewQueryOdataMapper(schema FilterSchema) *QueryOdataMapper {
	return &QueryOdataMapper{filterSchema: schema}
}

var _ repo.QueryMapper = (*QueryOdataMapper)(nil) // Assert interface impl

// GetQuery is a QueryMapper interface function which returns a *repo.Query with where clauses
// added from the mapped odata filter parameter string.
func (mf *QueryOdataMapper) GetQuery() *repo.Query {
	query := repo.NewQuery()

	for i := range mf.parseFilterFields {
		entry, _ := mf.filterSchema.getEntryFromDBName(mf.parseFilterFields[i])
		if entry.DBQuery == WhereQuery {
			ck := repo.NewCompositeKey()
			entry := repo.CompositeKeyEntry{
				Key: repo.Key{
					Value:     mf.parseFilterValues[i],
					Operation: repo.Equal,
				},
			}
			cond := repo.Condition{Field: mf.parseFilterFields[i],
				Value: entry}
			ck.Conds = append(ck.Conds, cond)
			ckg := repo.NewCompositeKeyGroup(ck)
			query.Where(ckg)
		}
	}

	skip := ptr.GetIntOrDefault(mf.skip, constants.DefaultSkip)
	top := ptr.GetIntOrDefault(mf.top, constants.DefaultTop)

	return query.SetOffset(skip).SetLimit(top)
}

// GetUUID is a QueryMapper interface function which returns the uuid value for a
// specified query field, if it exists.
func (mf *QueryOdataMapper) GetUUID(field repo.QueryField) (uuid.UUID, error) {
	val, ok := mf.parseFilterMap[field]
	if !ok || val == nil {
		// Use the schema for the type check, since these was never a filter
		// value provided. Really just a sanity check.
		err := mf.filterSchema.assertTypeFromDBName(field, UUID)
		if err != nil {
			return uuid.Nil, err
		}

		return uuid.Nil, nil
	}

	converted, ok := val.(uuid.UUID)
	if !ok {
		return uuid.Nil, ErrFilterTypeIncompatible
	}

	return converted, nil
}

// Non QueryMapper interface functions:

// SetPaging is used in the controller to set the paging.
func (mf *QueryOdataMapper) SetPaging(skip, top *int) {
	mf.skip = skip
	mf.top = top
}

// ParseFilter is used in the controller to parse the http odata parameter string.
func (mf *QueryOdataMapper) ParseFilter(param *string) error {
	err := mf.filterSchema.validate()
	if err != nil {
		return err
	}

	if param == nil {
		return nil
	}

	paramVal := *param

	parsedFilterFields, parsedFilterValues, err := parseFilter(paramVal)
	if err != nil {
		return err
	}

	mf.parseFilterMap = make(map[string]any, len(parsedFilterFields))

	for i, field := range parsedFilterFields {
		convertedField, convertedValue, err := mf.filterSchema.apply(
			field, parsedFilterValues[i])
		if err != nil {
			return err
		}

		mf.setFieldValue(convertedField, convertedValue)
	}

	return nil
}

// setFieldValue sets each parsed filter field and value into the internal data.
func (mf *QueryOdataMapper) setFieldValue(field string, value any) {
	mf.parseFilterFields = append(mf.parseFilterFields, field)
	mf.parseFilterValues = append(mf.parseFilterValues, value)
	mf.parseFilterMap[field] = value
}
