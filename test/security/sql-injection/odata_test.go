package sqlinjection_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.tools.sap/kms/cmk/internal/repo"
	"github.tools.sap/kms/cmk/utils/odata"
)

func makeExpectedQuery(fields []string, values []any) *repo.Query {
	query := repo.NewQuery()

	for i := range fields {
		ck := repo.NewCompositeKey()
		entry := repo.CompositeKeyEntry{
			Key: repo.Key{
				Value:     values[i],
				Operation: repo.Equal,
			},
		}
		cond := repo.Condition{Field: fields[i],
			Value: entry}
		ck.Conds = append(ck.Conds, cond)
		ckg := repo.NewCompositeKeyGroup(ck)
		query.Where(ckg)
	}

	return query.SetLimit(20)
}

func TestOdata_ForSqlInjection(t *testing.T) {
	tests := []struct {
		name          string
		filterSchema  odata.FilterSchema
		filterString  string
		expectedQuery *repo.Query
		expectedError error
	}{
		{
			name: "attempted injection for int type",
			filterSchema: odata.FilterSchema{
				Entries: []odata.FilterSchemaEntry{
					{
						FilterName: "test", FilterType: odata.Int,
						DBName: "testDB", DBQuery: odata.WhereQuery,
						ValueModifier: nil, ValueValidator: nil,
					},
				},
			},
			filterString:  "test eq 1 OR 1=1",
			expectedQuery: nil,
			expectedError: odata.ErrFilterNotToSpec,
		},
		{
			name: "attempted injection for string type quoted",
			filterSchema: odata.FilterSchema{
				Entries: []odata.FilterSchemaEntry{
					{
						FilterName: "test", FilterType: odata.String,
						DBName: "testDB", DBQuery: odata.WhereQuery,
						ValueModifier: nil, ValueValidator: nil,
					},
				},
			},
			filterString:  "test eq '1 OR 1=1'",
			expectedQuery: makeExpectedQuery([]string{"testDB"}, []any{"1 OR 1=1"}),
			expectedError: nil,
		},
		{
			name: "attempted injection for string type part quoted",
			filterSchema: odata.FilterSchema{
				Entries: []odata.FilterSchemaEntry{
					{
						FilterName: "test", FilterType: odata.String,
						DBName: "testDB", DBQuery: odata.WhereQuery,
						ValueModifier: nil, ValueValidator: nil,
					},
				},
			},
			filterString:  "test eq '1' OR 1=1",
			expectedQuery: nil,
			expectedError: odata.ErrFilterNotToSpec,
		},
	}

	// These tests aren't very useful, since we really need the repo to
	// apply escaping. The test eq '1 OR 1=1' will actually make it through
	// to the next layer and will therefore require proper escaping my the repo.
	// Also see the repo tests.
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fieldMap := odata.NewQueryOdataMapper(tt.filterSchema)
			err := fieldMap.ParseFilter(&tt.filterString)
			assert.Equal(t, tt.expectedError, err)

			if err == nil {
				assert.Equal(t, tt.expectedQuery, fieldMap.GetQuery())
			}
		})
	}
}
