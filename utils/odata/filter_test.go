package odata_test

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"

	"github.com/openkcm/cmk/internal/repo"
	"github.com/openkcm/cmk/utils/odata"
)

type wantCond struct {
	field string
	value any
	op    repo.ComparisonOp
	or    bool
}

func buildQuery(conds ...wantCond) *repo.Query {
	q := repo.NewQuery()
	for _, c := range conds {
		ck := repo.NewCompositeKey()
		ck.Conds = append(ck.Conds, repo.Condition{
			Field: c.field,
			Value: repo.CompositeKeyEntry{Key: repo.Key{Value: c.value, Operation: c.op}},
		})
		ckg := repo.NewCompositeKeyGroup(ck)
		if c.or {
			ckg.IsStrict = false
		}
		q.Where(ckg)
	}
	return q
}

func TestFilterParse(t *testing.T) {
	tests := []struct {
		name   string
		filter string
		want   []odata.FilterItem
	}{
		{
			name:   "single condition",
			filter: "Status eq 'Active'",
			want:   []odata.FilterItem{{Conditional: "", Field: "Status", Operation: "eq", Value: "Active"}},
		},
		{
			name:   "and then or keeps connectors in order",
			filter: "a eq '1' and b eq '2' or c eq '3'",
			want: []odata.FilterItem{
				{Conditional: "", Field: "a", Operation: "eq", Value: "1"},
				{Conditional: "and", Field: "b", Operation: "eq", Value: "2"},
				{Conditional: "or", Field: "c", Operation: "eq", Value: "3"},
			},
		},
		{
			name:   "operators inside a literal are ignored",
			filter: "name eq 'a and b or c'",
			want:   []odata.FilterItem{{Conditional: "", Field: "name", Operation: "eq", Value: "a and b or c"}},
		},
		{
			name:   "leading and trailing spaces inside quotes are kept",
			filter: "name eq '  spaced  '",
			want:   []odata.FilterItem{{Conditional: "", Field: "name", Operation: "eq", Value: "  spaced  "}},
		},
		{
			name:   "escaped quote is unescaped",
			filter: "name eq 'it''s'",
			want:   []odata.FilterItem{{Conditional: "", Field: "name", Operation: "eq", Value: "it's"}},
		},
		{
			name:   "empty string literal",
			filter: "name eq ''",
			want:   []odata.FilterItem{{Conditional: "", Field: "name", Operation: "eq", Value: ""}},
		},
		{
			name:   "quotes at the edges of a literal",
			filter: "name eq '''quoted'''",
			want:   []odata.FilterItem{{Conditional: "", Field: "name", Operation: "eq", Value: "'quoted'"}},
		},
		{
			name:   "bare (unquoted) value",
			filter: "age eq 42",
			want:   []odata.FilterItem{{Conditional: "", Field: "age", Operation: "eq", Value: "42"}},
		},
		{
			name:   "non-eq operator is preserved",
			filter: "age ge 42 and name ne 'bob'",
			want: []odata.FilterItem{
				{Conditional: "", Field: "age", Operation: "ge", Value: "42"},
				{Conditional: "and", Field: "name", Operation: "ne", Value: "bob"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f, err := odata.NewFilter(&tt.filter, nil)
			assert.NoError(t, err)
			assert.Equal(t, tt.want, f.Items)
		})
	}
}

func TestFilterParseEmpty(t *testing.T) {
	tests := []struct {
		name  string
		value *string
	}{
		{name: "nil pointer", value: nil},
		{name: "empty string", value: new("")},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f, err := odata.NewFilter(tt.value, nil)
			assert.NoError(t, err)
			assert.Nil(t, f.Items)
		})
	}
}

func TestFilterParseErrors(t *testing.T) {
	tests := []struct {
		name    string
		filter  string
		wantErr error
	}{
		{"odd quotes", "name eq 'oops", odata.ErrFilterNotToSpec},
		{"missing value", "name eq", odata.ErrFilterNotToSpec},
		{"missing operator and value", "name", odata.ErrFilterNotToSpec},
		{"field slot is reserved word", "and eq '1'", odata.ErrFilterNotToSpec},
		{"missing connector between conditions", "a eq '1' b eq '2'", odata.ErrFilterNotToSpec},
		{"dangling connector", "a eq '1' and", odata.ErrFilterNotToSpec},
		{"unsupported operator", "a lol '1'", odata.ErrFilterOperationNotSupported},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := odata.NewFilter(&tt.filter, nil)
			assert.ErrorIs(t, err, tt.wantErr)
		})
	}
}

func TestFilterGetQuery(t *testing.T) {
	id := uuid.New()

	tests := []struct {
		name   string
		filter string
		schema odata.FilterToRepoMap
		want   *repo.Query
	}{
		{
			name:   "string eq maps to DBName and applies modifier",
			filter: "status eq 'active'",
			schema: odata.FilterToRepoMap{
				"status": {Type: odata.String, DBName: "status_db", ValueModifier: odata.ToUpper},
			},
			want: buildQuery(wantCond{field: "status_db", value: "ACTIVE", op: repo.Equal}),
		},
		{
			name:   "int gt converts value",
			filter: "age gt 18",
			schema: odata.FilterToRepoMap{"age": {Type: odata.Int, DBName: "age_db"}},
			want:   buildQuery(wantCond{field: "age_db", value: int64(18), op: repo.GreaterThan}),
		},
		{
			name:   "uuid eq converts value",
			filter: "id eq '" + id.String() + "'",
			schema: odata.FilterToRepoMap{"id": {Type: odata.UUID, DBName: "id_db"}},
			want:   buildQuery(wantCond{field: "id_db", value: id, op: repo.Equal}),
		},
		{
			name:   "bool ne converts value",
			filter: "flag ne true",
			schema: odata.FilterToRepoMap{"flag": {Type: odata.Bool, DBName: "flag_db"}},
			want:   buildQuery(wantCond{field: "flag_db", value: true, op: repo.NotEqual}),
		},
		{
			name:   "and keeps strict, or relaxes strict",
			filter: "a eq '1' and b eq '2' or c eq '3'",
			schema: odata.FilterToRepoMap{
				"a": {Type: odata.String, DBName: "a_db"},
				"b": {Type: odata.String, DBName: "b_db"},
				"c": {Type: odata.String, DBName: "c_db"},
			},
			want: buildQuery(
				wantCond{field: "a_db", value: "1", op: repo.Equal},
				wantCond{field: "b_db", value: "2", op: repo.Equal},
				wantCond{field: "c_db", value: "3", op: repo.Equal, or: true},
			),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f, err := odata.NewFilter(&tt.filter, tt.schema)
			assert.NoError(t, err)

			got, err := f.GetQuery()
			assert.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestFilterGetQueryDBQuery(t *testing.T) {
	var gotValue any
	filter := "x eq 5"
	schema := odata.FilterToRepoMap{
		"x": {
			Type: odata.Int,
			DBQuery: func(q *repo.Query, value any) *repo.Query {
				gotValue = value
				return q
			},
		},
	}

	f, err := odata.NewFilter(&filter, schema)
	assert.NoError(t, err)

	query, err := f.GetQuery()
	assert.NoError(t, err)
	assert.Equal(t, int64(5), gotValue)
	assert.Equal(t, repo.NewQuery(), query)
}

func TestFilterGetQueryErrors(t *testing.T) {
	tests := []struct {
		name    string
		filter  string
		schema  odata.FilterToRepoMap
		wantErr error
	}{
		{
			name:    "field not in schema",
			filter:  "ghost eq '1'",
			schema:  odata.FilterToRepoMap{},
			wantErr: odata.ErrFilterNonSchema,
		},
		{
			name:    "value fails validator",
			filter:  "name eq 'toolong'",
			schema:  odata.FilterToRepoMap{"name": {Type: odata.String, DBName: "name_db", ValueValidator: odata.MaxLengthValidator(2)}},
			wantErr: odata.ErrFilterInvalidValue,
		},
		{
			name:    "unsupported operation",
			filter:  "age ge 42",
			schema:  odata.FilterToRepoMap{"age": {Type: odata.Int, DBName: "age_db"}},
			wantErr: odata.ErrFilterOperationNotSupported,
		},
		{
			name:    "unsupported type",
			filter:  "x eq '1'",
			schema:  odata.FilterToRepoMap{"x": {DBName: "x_db"}},
			wantErr: odata.ErrFilterTypeNotSupported,
		},
		{
			name:    "int conversion failure",
			filter:  "age eq abc",
			schema:  odata.FilterToRepoMap{"age": {Type: odata.Int, DBName: "age_db"}},
			wantErr: odata.ErrFilterValueConversionFailed,
		},
		{
			name:    "uuid conversion failure",
			filter:  "id eq 'not-a-uuid'",
			schema:  odata.FilterToRepoMap{"id": {Type: odata.UUID, DBName: "id_db"}},
			wantErr: odata.ErrFilterValueConversionFailed,
		},
		{
			name:    "bool conversion failure",
			filter:  "flag eq 'maybe'",
			schema:  odata.FilterToRepoMap{"flag": {Type: odata.Bool, DBName: "flag_db"}},
			wantErr: odata.ErrFilterNotToSpec,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f, err := odata.NewFilter(&tt.filter, tt.schema)
			assert.NoError(t, err)

			_, err = f.GetQuery()
			assert.ErrorIs(t, err, tt.wantErr)
		})
	}
}

func TestFilterGetFieldValues(t *testing.T) {
	t.Run("matches on DBName and converts", func(t *testing.T) {
		filter := "age eq 42"
		schema := odata.FilterToRepoMap{"age": {Type: odata.Int, DBName: "age_db"}}
		f, err := odata.NewFilter(&filter, schema)
		assert.NoError(t, err)

		vals, err := f.GetFieldValues("age_db")
		assert.NoError(t, err)
		assert.Equal(t, []any{int64(42)}, vals)
	})

	t.Run("returns all values for the same DBName", func(t *testing.T) {
		filter := "a eq '1' and a eq '2'"
		schema := odata.FilterToRepoMap{"a": {Type: odata.String, DBName: "a_db"}}
		f, err := odata.NewFilter(&filter, schema)
		assert.NoError(t, err)

		vals, err := f.GetFieldValues("a_db")
		assert.NoError(t, err)
		assert.Equal(t, []any{"1", "2"}, vals)
	})

	t.Run("no match returns empty", func(t *testing.T) {
		filter := "a eq '1'"
		schema := odata.FilterToRepoMap{"a": {Type: odata.String, DBName: "a_db"}}
		f, err := odata.NewFilter(&filter, schema)
		assert.NoError(t, err)

		vals, err := f.GetFieldValues("other_db")
		assert.NoError(t, err)
		assert.Empty(t, vals)
	})

	t.Run("field not in schema errors", func(t *testing.T) {
		filter := "ghost eq '1'"
		f, err := odata.NewFilter(&filter, odata.FilterToRepoMap{})
		assert.NoError(t, err)

		vals, err := f.GetFieldValues("ghost")
		assert.ErrorIs(t, err, odata.ErrFilterNonSchema)
		assert.Nil(t, vals)
	})

	t.Run("conversion error propagates", func(t *testing.T) {
		filter := "age eq abc"
		schema := odata.FilterToRepoMap{"age": {Type: odata.Int, DBName: "age_db"}}
		f, err := odata.NewFilter(&filter, schema)
		assert.NoError(t, err)

		vals, err := f.GetFieldValues("age_db")
		assert.ErrorIs(t, err, odata.ErrFilterValueConversionFailed)
		assert.Nil(t, vals)
	})
}
