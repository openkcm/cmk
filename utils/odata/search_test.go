package odata_test

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/openkcm/cmk/internal/repo"
	"github.com/openkcm/cmk/utils/odata"
)

type wantTerm struct {
	value string
	or    bool
}

func buildSearchQuery(fields []string, terms ...wantTerm) *repo.Query {
	q := repo.NewQuery()
	for _, term := range terms {
		ck := repo.NewCompositeKey()
		ck.IsStrict = false // OR across fields

		for _, field := range fields {
			ck.Conds = append(ck.Conds, repo.Condition{
				Field: fmt.Sprintf("CAST(%s AS TEXT)", field),
				Value: repo.CompositeKeyEntry{Key: repo.Key{
					Value:     "%" + term.value + "%",
					Operation: repo.Contains,
				}},
			})
		}

		ckg := repo.NewCompositeKeyGroup(ck)
		if term.or {
			ckg.IsStrict = false
		}

		q.Where(ckg)
	}

	return q
}

func TestSearchParse(t *testing.T) {
	tests := []struct {
		name   string
		search string
		want   []odata.SearchItem
	}{
		{
			name:   "single term",
			search: "alice",
			want:   []odata.SearchItem{{Conditional: "", Value: "alice"}},
		},
		{
			name:   "and then or keeps connectors in order",
			search: "a and b or c",
			want: []odata.SearchItem{
				{Conditional: "", Value: "a"},
				{Conditional: "and", Value: "b"},
				{Conditional: "or", Value: "c"},
			},
		},
		{
			name:   "quoted term keeps spaces and connectors as literal",
			search: "'a and b'",
			want:   []odata.SearchItem{{Conditional: "", Value: "a and b"}},
		},
		{
			name:   "escaped quote is unescaped",
			search: "'it''s'",
			want:   []odata.SearchItem{{Conditional: "", Value: "it's"}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s, err := odata.NewSearch(&tt.search, "name")
			assert.NoError(t, err)
			assert.Equal(t, tt.want, s.Items)
		})
	}
}

func TestSearchParseEmpty(t *testing.T) {
	tests := []struct {
		name   string
		search *string
		fields []repo.QueryField
	}{
		{name: "nil search", search: nil, fields: []repo.QueryField{"name"}},
		{name: "empty search", search: new(""), fields: []repo.QueryField{"name"}},
		{name: "no fields", search: new("alice"), fields: nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s, err := odata.NewSearch(tt.search, tt.fields...)
			assert.NoError(t, err)
			assert.Nil(t, s.Items)
		})
	}
}

func TestSearchParseErrors(t *testing.T) {
	tests := []struct {
		name   string
		search string
	}{
		{"missing connector between terms", "a b"},
		{"dangling connector", "a and"},
		{"leading connector", "and a"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := odata.NewSearch(&tt.search, "name")
			assert.ErrorIs(t, err, odata.ErrFilterNotToSpec)
		})
	}
}

func TestSearchGetQuery(t *testing.T) {
	tests := []struct {
		name   string
		search string
		fields []repo.QueryField
		want   *repo.Query
	}{
		{
			name:   "single term single field",
			search: "alice",
			fields: []repo.QueryField{"name"},
			want:   buildSearchQuery([]string{"name"}, wantTerm{value: "alice"}),
		},
		{
			name:   "single term ORs across all fields",
			search: "alice",
			fields: []repo.QueryField{"name", "email", "id"},
			want:   buildSearchQuery([]string{"name", "email", "id"}, wantTerm{value: "alice"}),
		},
		{
			name:   "and keeps strict between terms",
			search: "alice and bob",
			fields: []repo.QueryField{"name"},
			want: buildSearchQuery([]string{"name"},
				wantTerm{value: "alice"},
				wantTerm{value: "bob"},
			),
		},
		{
			name:   "or relaxes strict between terms",
			search: "alice or bob",
			fields: []repo.QueryField{"name", "email"},
			want: buildSearchQuery([]string{"name", "email"},
				wantTerm{value: "alice"},
				wantTerm{value: "bob", or: true},
			),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s, err := odata.NewSearch(&tt.search, tt.fields...)
			assert.NoError(t, err)

			got, err := s.GetQuery()
			assert.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestSearchGetQueryEmpty(t *testing.T) {
	tests := []struct {
		name   string
		search *string
		fields []repo.QueryField
	}{
		{name: "nil search", search: nil, fields: []repo.QueryField{"name"}},
		{name: "no fields", search: new("alice"), fields: nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s, err := odata.NewSearch(tt.search, tt.fields...)
			assert.NoError(t, err)

			got, err := s.GetQuery()
			assert.NoError(t, err)
			assert.Equal(t, repo.NewQuery(), got)
		})
	}
}
