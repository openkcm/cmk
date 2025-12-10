package odata_test

import (
	"strconv"
	"strings"
	"testing"

	"github.com/google/uuid"
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

func TestBasicSchema(t *testing.T) {
	testUUID, err := uuid.Parse("14641e57-bd17-4d91-9552-aa0468dc6c91")
	assert.NoError(t, err)

	tests := []struct {
		name          string
		filterSchema  odata.FilterSchema
		filterString  string
		expectedQuery *repo.Query
		expectedError error
	}{
		{
			name:          "empty filter",
			filterSchema:  odata.FilterSchema{},
			filterString:  "",
			expectedQuery: makeExpectedQuery([]string{}, []any{}),
			expectedError: nil,
		},
		{
			name: "single filter",
			filterSchema: odata.FilterSchema{
				Entries: []odata.FilterSchemaEntry{
					{"test", odata.Int, "testDB", odata.WhereQuery, nil, nil},
				},
			},
			filterString:  "test eq 1",
			expectedQuery: makeExpectedQuery([]string{"testDB"}, []any{int64(1)}),
			expectedError: nil,
		},
		{
			name: "two filters same field",
			filterSchema: odata.FilterSchema{
				Entries: []odata.FilterSchemaEntry{
					{"test", odata.Int, "testDB", odata.WhereQuery, nil, nil},
				},
			},
			filterString: "test eq 1 and test eq 2",
			expectedQuery: makeExpectedQuery([]string{"testDB", "testDB"},
				[]any{int64(1), int64(2)}),
			expectedError: nil,
		},
		{
			name: "two filters same field with whitespace",
			filterSchema: odata.FilterSchema{
				Entries: []odata.FilterSchemaEntry{
					{"test", odata.Int, "testDB", odata.WhereQuery, nil, nil},
				},
			},
			filterString: "   test    eq   1    and    test  eq     2    ",
			expectedQuery: makeExpectedQuery([]string{"testDB", "testDB"},
				[]any{int64(1), int64(2)}),
			expectedError: nil,
		},
		{
			name: "three filters three fields and types",
			filterSchema: odata.FilterSchema{
				Entries: []odata.FilterSchemaEntry{
					{"test1", odata.Int, "test1DB", odata.WhereQuery, nil, nil},
					{"test2", odata.String, "test2DB", odata.WhereQuery, nil, nil},
					{"test3", odata.Bool, "test3DB", odata.WhereQuery, nil, nil},
					{"test4", odata.UUID, "test4DB", odata.WhereQuery, nil, nil},
				},
			},
			filterString: `test1 eq 1 and test2 eq 'teststr' and test3 eq true and test4 eq ` +
				`'14641e57-bd17-4d91-9552-aa0468dc6c91'`,
			expectedQuery: makeExpectedQuery([]string{"test1DB", "test2DB",
				"test3DB", "test4DB"},
				[]any{int64(1), "teststr", true, testUUID}),
			expectedError: nil,
		},
		{
			name: "empty string",
			filterSchema: odata.FilterSchema{
				Entries: []odata.FilterSchemaEntry{
					{"test", odata.String, "testDB", odata.WhereQuery, nil, nil},
				},
			},
			filterString:  "test eq ''",
			expectedQuery: makeExpectedQuery([]string{"testDB"}, []any{""}),
			expectedError: nil,
		},
		{
			name: "unquoted string",
			filterSchema: odata.FilterSchema{
				Entries: []odata.FilterSchemaEntry{
					{"test", odata.String, "testDB", odata.WhereQuery, nil, nil},
				},
			},
			filterString:  "test eq teststr",
			expectedQuery: makeExpectedQuery([]string{"testDB"}, []any{""}),
			expectedError: odata.ErrFilterNotToSpec,
		},
		{
			name:          "with filter specifier",
			filterSchema:  odata.FilterSchema{},
			filterString:  "$filter=",
			expectedQuery: nil,
			expectedError: odata.ErrFilterNotToSpec,
		},
		{
			name:          "non supported or",
			filterSchema:  odata.FilterSchema{},
			filterString:  "test eq 1 or test eq 2",
			expectedQuery: nil,
			expectedError: odata.ErrFilterOperationNotSupported,
		},
		{
			name:          "non supported not",
			filterSchema:  odata.FilterSchema{},
			filterString:  "not test gt 5",
			expectedQuery: nil,
			expectedError: odata.ErrFilterOperationNotSupported,
		},
		{
			name:          "non supported ne",
			filterSchema:  odata.FilterSchema{},
			filterString:  "test eq 1 or test ne 2",
			expectedQuery: nil,
			expectedError: odata.ErrFilterOperationNotSupported,
		},
		{
			name:          "non supported gt",
			filterSchema:  odata.FilterSchema{},
			filterString:  "test eq 1 or test gt 2",
			expectedQuery: nil,
			expectedError: odata.ErrFilterOperationNotSupported,
		},
		{
			name:          "non supported ge",
			filterSchema:  odata.FilterSchema{},
			filterString:  "test ge 2",
			expectedQuery: nil,
			expectedError: odata.ErrFilterOperationNotSupported,
		},
		{
			name:          "non supported lt",
			filterSchema:  odata.FilterSchema{},
			filterString:  "test lt 2",
			expectedQuery: nil,
			expectedError: odata.ErrFilterOperationNotSupported,
		},
		{
			name:          "non supported le",
			filterSchema:  odata.FilterSchema{},
			filterString:  "test le 1",
			expectedQuery: nil,
			expectedError: odata.ErrFilterOperationNotSupported,
		},
		{
			name:          "non supported ()",
			filterSchema:  odata.FilterSchema{},
			filterString:  "test eq 1 or (test eq 2 and test eq 4)",
			expectedQuery: nil,
			expectedError: odata.ErrFilterOperationNotSupported,
		},
		{
			name: "invalid filter",
			filterSchema: odata.FilterSchema{
				Entries: []odata.FilterSchemaEntry{
					{"test", odata.Int, "testDB", odata.WhereQuery, nil, nil},
				},
			},
			filterString:  "test eq 1 eq 2",
			expectedQuery: nil,
			expectedError: odata.ErrFilterNotToSpec,
		},
		{
			name: "bad int",
			filterSchema: odata.FilterSchema{
				Entries: []odata.FilterSchemaEntry{
					{"test", odata.Int, "testDB", odata.WhereQuery, nil, nil},
				},
			},
			filterString:  "test eq re and test eq 2",
			expectedQuery: nil,
			expectedError: odata.ErrFilterNotToSpec,
		},
		{
			name: "bad bool",
			filterSchema: odata.FilterSchema{
				Entries: []odata.FilterSchemaEntry{
					{"test", odata.Bool, "testDB", odata.WhereQuery, nil, nil},
				},
			},
			filterString:  "test eq re and test eq true",
			expectedQuery: nil,
			expectedError: odata.ErrFilterNotToSpec,
		},
		{
			name: "non schema filter",
			filterSchema: odata.FilterSchema{
				Entries: []odata.FilterSchemaEntry{
					{"test1", odata.Int, "test1DB", odata.WhereQuery, nil, nil},
					{"test2", odata.String, "test2DB", odata.WhereQuery, nil, nil},
					{"test3", odata.Bool, "test3DB", odata.WhereQuery, nil, nil},
				},
			},
			filterString:  "test4 eq 1 and test2 eq 'teststr' and test3 eq true",
			expectedQuery: nil,
			expectedError: odata.ErrFilterNonSchema,
		},
	}

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

func TestDBNonDefaultQueries(t *testing.T) {
	filterSchema := odata.FilterSchema{
		Entries: []odata.FilterSchemaEntry{
			{"test", odata.Int, "testDB", odata.NoQuery, nil, nil},
		},
	}
	filterString := "test eq 1"
	expectedQuery := makeExpectedQuery([]string{}, []any{})

	fieldMap := odata.NewQueryOdataMapper(filterSchema)
	err := fieldMap.ParseFilter(&filterString)
	assert.NoError(t, err)
	assert.Equal(t, expectedQuery, fieldMap.GetQuery())
}

func TestOperations(t *testing.T) {
	tests := []struct {
		name          string
		filterSchema  odata.FilterSchema
		filterString  string
		expectedQuery *repo.Query
		expectedError error
	}{
		{
			name: "and operation within string",
			filterSchema: odata.FilterSchema{
				Entries: []odata.FilterSchemaEntry{
					{"test1", odata.Int, "test1DB", odata.WhereQuery, nil, nil},
					{"test2", odata.String, "test2DB", odata.WhereQuery, nil, nil},
					{"test3", odata.Bool, "test3DB", odata.WhereQuery, nil, nil},
				},
			},
			filterString: "test1 eq 1 and test2 eq 'test and str' and test3 eq true",
			expectedQuery: makeExpectedQuery([]string{"test1DB", "test2DB", "test3DB"},
				[]any{int64(1), "test and str", true}),
			expectedError: nil,
		},
		{
			name: "or operation within string",
			filterSchema: odata.FilterSchema{
				Entries: []odata.FilterSchemaEntry{
					{"test1", odata.Int, "test1DB", odata.WhereQuery, nil, nil},
					{"test2", odata.String, "test2DB", odata.WhereQuery, nil, nil},
					{"test3", odata.Bool, "test3DB", odata.WhereQuery, nil, nil},
				},
			},
			filterString: "test1 eq 1 and test2 eq 'test or str' and test3 eq true",
			expectedQuery: makeExpectedQuery([]string{"test1DB", "test2DB", "test3DB"},
				[]any{int64(1), "test or str", true}),
			expectedError: nil,
		},
		{
			name: "eq operation within string",
			filterSchema: odata.FilterSchema{
				Entries: []odata.FilterSchemaEntry{
					{"test1", odata.Int, "test1DB", odata.WhereQuery, nil, nil},
					{"test2", odata.String, "test2DB", odata.WhereQuery, nil, nil},
					{"test3", odata.Bool, "test3DB", odata.WhereQuery, nil, nil},
				},
			},
			filterString: "test1 eq 1 and test2 eq 'test eq str' and test3 eq true",
			expectedQuery: makeExpectedQuery([]string{"test1DB", "test2DB", "test3DB"},
				[]any{int64(1), "test eq str", true}),
			expectedError: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fieldMap := odata.NewQueryOdataMapper(tt.filterSchema)
			err := fieldMap.ParseFilter(&tt.filterString)
			assert.Equal(t, tt.expectedError, err)
			assert.Equal(t, tt.expectedQuery, fieldMap.GetQuery())
		})
	}
}

func TestStringQuoteEscape(t *testing.T) {
	tests := []struct {
		name          string
		filterString  string
		expectedQuery *repo.Query
	}{
		{
			name:          "empty string",
			filterString:  "test eq ''",
			expectedQuery: makeExpectedQuery([]string{"testDB"}, []any{""}),
		},
		{
			name:          "escaped quote",
			filterString:  "test eq ''''",
			expectedQuery: makeExpectedQuery([]string{"testDB"}, []any{"'"}),
		},
		{
			name:          "two escaped quotes",
			filterString:  "test eq ''''''",
			expectedQuery: makeExpectedQuery([]string{"testDB"}, []any{"''"}),
		},
		{
			name:          "escaped quote",
			filterString:  "test eq ''''",
			expectedQuery: makeExpectedQuery([]string{"testDB"}, []any{"'"}),
		},
		{
			name:          "two clumps with quote",
			filterString:  "test eq '''_'''",
			expectedQuery: makeExpectedQuery([]string{"testDB"}, []any{"'_'"}),
		},
		{
			name:          "two clumps with two quotes",
			filterString:  "test eq '''''_'''''",
			expectedQuery: makeExpectedQuery([]string{"testDB"}, []any{"''_''"}),
		},
		{
			name:          "speech case - single quote",
			filterString:  "test eq 'He said, ''I can''t believe it''s not butter'''",
			expectedQuery: makeExpectedQuery([]string{"testDB"}, []any{"He said, 'I can't believe it's not butter'"}),
		},
		{
			name:          "speech case - double quotes",
			filterString:  "test eq 'He said, \"I can''t believe it''s not butter\"'",
			expectedQuery: makeExpectedQuery([]string{"testDB"}, []any{"He said, \"I can't believe it's not butter\""}),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filterSchema := odata.FilterSchema{
				Entries: []odata.FilterSchemaEntry{
					{"test", odata.String, "testDB", odata.WhereQuery, nil, nil},
				},
			}
			fieldMap := odata.NewQueryOdataMapper(filterSchema)
			err := fieldMap.ParseFilter(&tt.filterString)
			assert.NoError(t, err)
			assert.Equal(t, tt.expectedQuery, fieldMap.GetQuery())
		})
	}
}

func TestStringErrors(t *testing.T) {
	tests := []struct {
		name         string
		filterString string
	}{
		{
			name:         "no quotes",
			filterString: "test eq 1",
		},
		{
			name:         "one quote",
			filterString: "test eq '1",
		},
		{
			name:         "three quotes and good operation",
			filterString: "test eq '1'' and test2 eq 2",
		},
		{
			name:         "three quotes",
			filterString: "test eq '1''",
		},
		{
			name:         "five quotes",
			filterString: "test eq ''''1'",
		},
		{
			name:         "one and three clumped quotes",
			filterString: "test eq '1'2''''",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filterSchema := odata.FilterSchema{
				Entries: []odata.FilterSchemaEntry{
					{"test", odata.String, "testDB", odata.WhereQuery, nil, nil},
					{"test2", odata.Int, "test2DB", odata.WhereQuery, nil, nil},
				},
			}
			fieldMap := odata.NewQueryOdataMapper(filterSchema)
			err := fieldMap.ParseFilter(&tt.filterString)
			assert.Error(t, err)
			assert.Equal(t, odata.ErrFilterNotToSpec, err)
		})
	}
}

func TestGetUUID(t *testing.T) {
	testUUID, err := uuid.Parse("14641e57-bd17-4d91-9552-aa0468dc6c91")
	assert.NoError(t, err)

	tests := []struct {
		name          string
		filterString  string
		testField     string
		expectedError error
		expectedValue any
	}{
		{
			name:          "no uuid when no filter fields",
			filterString:  "",
			testField:     "test1DB",
			expectedError: nil,
			expectedValue: nil,
		},
		{
			name:          "uuid got when uuid filter",
			filterString:  "test1 eq '14641e57-bd17-4d91-9552-aa0468dc6c91'",
			testField:     "test1DB",
			expectedError: nil,
			expectedValue: testUUID,
		},
		{
			name:          "no uuid got when no uuid in filter",
			filterString:  "test2 eq '1'",
			testField:     "test1DB",
			expectedError: nil,
			expectedValue: nil,
		},
		{
			name:          "no uuid when uuid not in filter",
			filterString:  "test1 eq '14641e57-bd17-4d91-9552-aa0468dc6c91' and test2 eq '1'",
			testField:     "test1DB",
			expectedError: nil,
			expectedValue: testUUID,
		},
		{
			name:          "error when filter param not uuid",
			filterString:  "test2 eq '14641e57-bd17-4d91-9552-aa0468dc6c91'",
			testField:     "test2DB",
			expectedError: odata.ErrFilterTypeIncompatible,
			expectedValue: nil,
		},
		{
			name:          "error when uuid not uuid type",
			filterString:  "test2 eq '1' and test3 eq false",
			testField:     "test2DB",
			expectedError: odata.ErrFilterTypeIncompatible,
			expectedValue: "1",
		},
		{
			name:          "error when uuid not correct type and not in filter",
			filterString:  "test1 eq '14641e57-bd17-4d91-9552-aa0468dc6c91' and test2 eq '1'",
			testField:     "test3DB",
			expectedError: odata.ErrFilterTypeIncompatible,
			expectedValue: nil,
		},
		{
			name:          "error when uuid not correct type and in filter",
			filterString:  "test1 eq '14641e57-bd17-4d91-9552-aa0468dc6c91' and test2 eq '1' and test3 eq false",
			testField:     "test3DB",
			expectedError: odata.ErrFilterTypeIncompatible,
			expectedValue: false,
		},
	}

	for _, tt := range tests {
		filterSchema := odata.FilterSchema{
			Entries: []odata.FilterSchemaEntry{
				{"test1", odata.UUID, "test1DB", odata.WhereQuery, nil, nil},
				{"test2", odata.String, "test2DB", odata.WhereQuery, nil, nil},
				{"test3", odata.Bool, "test3DB", odata.WhereQuery, nil, nil},
			},
		}

		t.Run(tt.name, func(t *testing.T) {
			fieldMap := odata.NewQueryOdataMapper(filterSchema)
			err := fieldMap.ParseFilter(&tt.filterString)
			assert.NoError(t, err)

			testUUID, err := fieldMap.GetUUID(tt.testField)
			if tt.expectedError != nil {
				assert.Error(t, err)
				assert.Equal(t, tt.expectedError, err)
			} else {
				assert.NoError(t, err)

				if tt.expectedValue == nil {
					assert.Equal(t, uuid.Nil, testUUID)
				}
			}
		})
	}
}

func TestValueModifier(t *testing.T) {
	testUUID, err := uuid.Parse("14641e57-bd17-4d91-9552-aa0468dc6c91")
	assert.NoError(t, err)

	filterSchema := odata.FilterSchema{
		Entries: []odata.FilterSchemaEntry{
			{"test1", odata.Int, "test1DB", odata.WhereQuery,
				func(s string) (string, bool) {
					i, _ := strconv.ParseInt(s, 10, 64)
					return strconv.Itoa(int(i * 20)), true
				},
				nil},
			{"test2", odata.String, "test2DB", odata.WhereQuery,
				func(s string) (string, bool) { return strings.ToUpper(s), true },
				nil},
			{"test3", odata.Bool, "test3DB", odata.WhereQuery,
				func(string) (string, bool) { return "false", true },
				nil},
			{"test4", odata.UUID, "test4DB", odata.WhereQuery,
				func(s string) (string, bool) { return s, true },
				nil},
		},
	}
	filterString := `test1 eq 1 and test2 eq 'teststr' and test3 eq true and test4 eq ` +
		`'14641e57-bd17-4d91-9552-aa0468dc6c91'`
	expectedQuery := makeExpectedQuery([]string{"test1DB", "test2DB",
		"test3DB", "test4DB"},
		[]any{int64(20), "TESTSTR", false, testUUID})

	fieldMap := odata.NewQueryOdataMapper(filterSchema)
	err = fieldMap.ParseFilter(&filterString)
	assert.NoError(t, err)
	assert.Equal(t, expectedQuery, fieldMap.GetQuery())
}

func TestValueValidation(t *testing.T) {
	filterSchema := odata.FilterSchema{
		Entries: []odata.FilterSchemaEntry{
			{"test1", odata.Int, "test1DB", odata.WhereQuery, nil,
				func(s string) bool {
					i, _ := strconv.ParseInt(s, 10, 64)
					return i < 10
				},
			},
		},
	}

	tests := []struct {
		name          string
		filterString  string
		expectedError bool
	}{
		{
			name:          "empty filter",
			filterString:  "test1 eq 1",
			expectedError: false,
		},
		{
			name:          "empty filter",
			filterString:  "test1 eq 10",
			expectedError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fieldMap := odata.NewQueryOdataMapper(filterSchema)

			err := fieldMap.ParseFilter(&tt.filterString)
			if !tt.expectedError {
				assert.NoError(t, err)
			} else {
				assert.Error(t, err)
				assert.Equal(t, odata.ErrFilterInvalidValue, err)
			}
		})
	}
}
