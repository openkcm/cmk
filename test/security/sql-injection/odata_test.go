package sqlinjection_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/openkcm/cmk/internal/repo"
	"github.com/openkcm/cmk/utils/odata"
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
		cond := repo.Condition{
			Field: fields[i],
			Value: entry,
		}
		ck.Conds = append(ck.Conds, cond)
		ckg := repo.NewCompositeKeyGroup(ck)
		query.Where(ckg)
	}

	return query
}

func TestOdata_ForSqlInjection(t *testing.T) {
	tests := []struct {
		name          string
		filterMap     odata.FilterToRepoMap
		filterString  string
		expectedQuery *repo.Query
		expectedError error
	}{
		{
			name: "attempted injection for int type",
			filterMap: odata.FilterToRepoMap{
				"test": {Type: odata.Int, DBName: "testDB"},
			},
			filterString:  "test eq 1 OR 1=1",
			expectedQuery: nil,
			expectedError: odata.ErrFilterNotToSpec,
		},
		{
			name: "attempted injection for string type quoted",
			filterMap: odata.FilterToRepoMap{
				"test": {Type: odata.String, DBName: "testDB"},
			},
			filterString:  "test eq '1 OR 1=1'",
			expectedQuery: makeExpectedQuery([]string{"test"}, []any{"1 OR 1=1"}),
			expectedError: nil,
		},
		{
			name: "attempted injection for string type part quoted",
			filterMap: odata.FilterToRepoMap{
				"test": {Type: odata.String, DBName: "testDB"},
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
			filter, err := odata.NewFilter(&tt.filterString, tt.filterMap)

			var query *repo.Query
			if err == nil {
				query, err = filter.GetQuery()
			}

			assert.Equal(t, tt.expectedError, err)
			if tt.expectedError == nil {
				assert.Equal(t, tt.expectedQuery, query)
			}
		})
	}
}
