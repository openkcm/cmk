package odata_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/openkcm/cmk/utils/odata"
)

// TestQuery tests the Query struct
func TestQuery(t *testing.T) {
	tests := []struct {
		name     string
		query    odata.Query
		expected string
	}{
		{
			name:     "Concat",
			query:    odata.NewQuery().Concat("FirstName", "LastName"),
			expected: "concat(FirstName, LastName)",
		},
		{
			name:     "IndexOf",
			query:    odata.NewQuery().IndexOf("Name", "John"),
			expected: "indexof(Name,'John')",
		},
		{
			name:     "Length",
			query:    odata.NewQuery().Length("Name"),
			expected: "length(Name)",
		},
		{
			name:     "Substring",
			query:    odata.NewQuery().Substring("Name", 1),
			expected: "substring(Name,1)",
		},
		{
			name:     "ToLower",
			query:    odata.NewQuery().ToLower("Name"),
			expected: "tolower(Name)",
		},
		{
			name:     "ToUpper",
			query:    odata.NewQuery().ToUpper("Name"),
			expected: "toupper(Name)",
		},
		{
			name:     "Trim",
			query:    odata.NewQuery().Trim("Name"),
			expected: "trim(Name)",
		},
		{
			name:     "Ceiling",
			query:    odata.NewQuery().Ceiling("Value"),
			expected: "ceiling(Value)",
		},
		{
			name:     "Floor",
			query:    odata.NewQuery().Floor("Value"),
			expected: "floor(Value)",
		},
		{
			name:     "Round",
			query:    odata.NewQuery().Round("Value"),
			expected: "round(Value)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.query.String())
		})
	}
}

// TestOdataFilter tests the Filter struct
func TestOdataFilter(t *testing.T) {
	tests := []struct {
		name     string
		filter   *odata.Filter
		expected string
	}{
		{
			"Eq",
			odata.NewFilter().Eq("Name", "John"),
			"Name eq 'John'",
		},
		{
			"In",
			odata.NewFilter().
				In("Name", "John", "Doe"),
			"Name in ('John','Doe')",
		},
		{
			"And",
			odata.NewFilter().
				Eq("Name", "John").
				And().
				Eq("Age", "30"),
			"Name eq 'John' and Age eq '30'",
		},
		{
			"Group",
			odata.NewFilter().Group(func(f *odata.Filter) {
				f.Eq("Name", "John").
					And().
					Eq("Age", "30")
			}),
			"(Name eq 'John' and Age eq '30')",
		},
		{
			"MultipleFilters",
			odata.NewFilter().
				Eq("Name", "John").
				And().
				Ne("Status", "Inactive").
				Or().
				Gt("Age", "25"),
			"Name eq 'John' and Status ne 'Inactive' or Age gt 25",
		},
		{
			"ComplexGroup",
			odata.NewFilter().Group(func(f *odata.Filter) {
				f.Eq("Name", "John").
					And().
					Eq("Age", "30")
			}).
				Or().
				Group(func(f *odata.Filter) {
					f.Ne("Status", "Inactive").
						And().
						Gt("Age", "25")
				}),
			"(Name eq 'John' and Age eq '30') or (Status ne 'Inactive' and Age gt 25)",
		},
		{
			"UsingQuery",
			odata.NewFilter().
				Eq(odata.NewQuery().
					IndexOf("Name", "John"), "1"),
			"indexof(Name,'John') eq '1'",
		},
		{
			"Contains",
			odata.NewFilter().Contains("Name", "John"),
			"contains(Name,'John')",
		},
		{
			"EndsWith",
			odata.NewFilter().EndsWith("Name", "John"),
			"endswith(Name,'John')",
		},
		{
			"StartsWith",
			odata.NewFilter().StartsWith("Name", "John"),
			"startswith(Name,'John')",
		},
		{
			"MatchesPattern",
			odata.NewFilter().MatchesPattern("Name", "J.*n"),
			"matchesPattern(Name,'J.*n')",
		},
		{
			"Add",
			odata.NewFilter().Add("Value", "10"),
			"Value add 10",
		},
		{
			"Sub",
			odata.NewFilter().Sub("Value", "5"),
			"Value sub 5",
		},
		{
			"Mul",
			odata.NewFilter().Mul("Value", "2"),
			"Value mul 2",
		},
		{
			"Div",
			odata.NewFilter().Div("Value", "3"),
			"Value div 3",
		},
		{
			"DivBy",
			odata.NewFilter().DivBy("Value", "4"),
			"Value divby 4",
		},
		{
			"Mod",
			odata.NewFilter().Mod("Value", "7"),
			"Value mod 7",
		},
		{
			"Ge",
			odata.NewFilter().Ge("Age", "30"),
			"Age ge 30",
		},
		{
			"Lt",
			odata.NewFilter().Lt("Age", "30"),
			"Age lt 30",
		},
		{
			"Le",
			odata.NewFilter().Le("Age", "30"),
			"Age le 30",
		},
		{
			"Has",
			odata.NewFilter().Has("Tags", "VIP"),
			"Tags has VIP",
		},
		{
			"Not",
			odata.NewFilter().Not().Eq("Status", "Inactive"),
			"not Status eq 'Inactive'",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.filter.String())
		})
	}
}
