package odata

import (
	"fmt"
	"strings"
)

// Filter represents an OData filter.
type Filter struct {
	filters []string
}

// NewFilter creates a new instance of Filter.
func NewFilter() *Filter {
	return &Filter{}
}

// Eq adds an equality filter to the Filter.
func (fb *Filter) Eq(field Query, value string) *Filter {
	return fb.addFilter("%s eq '%s'", field, value)
}

// Ne adds a not equal filter to the Filter.
func (fb *Filter) Ne(field Query, value string) *Filter {
	return fb.addFilter("%s ne '%s'", field, value)
}

// Gt adds a greater than filter to the Filter.
func (fb *Filter) Gt(field Query, value string) *Filter {
	return fb.addFilter("%s gt %v", field, value)
}

// Ge adds a greater than or equal filter to the Filter.
func (fb *Filter) Ge(field Query, value string) *Filter {
	return fb.addFilter("%s ge %v", field, value)
}

// Lt adds a less than filter to the Filter.
func (fb *Filter) Lt(field Query, value string) *Filter {
	return fb.addFilter("%s lt %v", field, value)
}

// Le adds a less than or equal filter to the Filter.
func (fb *Filter) Le(field Query, value string) *Filter {
	return fb.addFilter("%s le %v", field, value)
}

// Has adds a has filter to the Filter.
func (fb *Filter) Has(field Query, value string) *Filter {
	return fb.addFilter("%s has %s", field, value)
}

// In adds an in filter to the Filter.
func (fb *Filter) In(field Query, values ...string) *Filter {
	return fb.addFilter("%s in ('%s')", field, strings.Join(values, "','"))
}

// And adds an and filter to the Filter.
func (fb *Filter) And() *Filter {
	return fb.addFilter("and")
}

// Or adds an or filter to the Filter.
func (fb *Filter) Or() *Filter {
	return fb.addFilter("or")
}

// Not adds a not filter to the Filter.
func (fb *Filter) Not() *Filter {
	return fb.addFilter("not")
}

// Add adds an add filter to the Filter.
func (fb *Filter) Add(field Query, value string) *Filter {
	return fb.addFilter("%s add %v", field, value)
}

// Sub adds a subtract filter to the Filter.
func (fb *Filter) Sub(field Query, value string) *Filter {
	return fb.addFilter("%s sub %v", field, value)
}

// Mul adds a multiply filter to the Filter.
func (fb *Filter) Mul(field Query, value string) *Filter {
	return fb.addFilter("%s mul %v", field, value)
}

// Div adds a divide filter to the Filter.
func (fb *Filter) Div(field Query, value string) *Filter {
	return fb.addFilter("%s div %v", field, value)
}

// DivBy adds a divide by filter to the Filter.
func (fb *Filter) DivBy(field Query, value string) *Filter {
	return fb.addFilter("%s divby %v", field, value)
}

// Mod adds a modulo filter to the Filter.
func (fb *Filter) Mod(field Query, value string) *Filter {
	return fb.addFilter("%s mod %v", field, value)
}

// Group groups filters together using the provided group function.
func (fb *Filter) Group(groupFunc func(*Filter)) *Filter {
	groupFilter := NewFilter()
	groupFunc(groupFilter)
	fb.filters = append(fb.filters, fmt.Sprintf("(%s)", groupFilter.String()))

	return fb
}

// Contains adds a contains filter to the Filter.
func (fb *Filter) Contains(field Query, value string) *Filter {
	return fb.addFilter("contains(%s,'%s')", field, value)
}

// EndsWith adds an endswith filter to the Filter.
func (fb *Filter) EndsWith(field Query, value string) *Filter {
	return fb.addFilter("endswith(%s,'%s')", field, value)
}

// StartsWith adds a startswith filter to the Filter.
func (fb *Filter) StartsWith(field Query, value string) *Filter {
	return fb.addFilter("startswith(%s,'%s')", field, value)
}

// MatchesPattern adds a matchesPattern filter to the Filter.
func (fb *Filter) MatchesPattern(field Query, pattern string) *Filter {
	return fb.addFilter("matchesPattern(%s,'%s')", field, pattern)
}

// String returns the string representation of the Filter.
func (fb *Filter) String() string {
	return strings.Join(fb.filters, " ")
}

// Query represents an OData query.
type Query string

// NewQuery creates a new instance of Query.
func NewQuery() *Query {
	return (*Query)(new(string))
}

// String returns the string representation of the Query.
func (q *Query) String() string {
	return string(*q)
}

// addFilter adds a formatted filter string to the Filter.
func (fb *Filter) addFilter(format string, args ...any) *Filter {
	fb.filters = append(fb.filters, fmt.Sprintf(format, args...))
	return fb
}

// Concat concatenates the provided fields in the Query.
func (q *Query) Concat(fields ...string) Query {
	return Query(fmt.Sprintf("concat(%s)", strings.Join(fields, ", ")))
}

// IndexOf returns the index of the value in the field in the Query.
func (q *Query) IndexOf(field, value string) Query {
	return Query(fmt.Sprintf("indexof(%s,'%s')", field, value))
}

// Length returns the length of the field in the Query.
func (q *Query) Length(field string) Query {
	return Query(fmt.Sprintf("length(%s)", field))
}

// Substring returns the substring of the field starting at the specified position in the Query.
func (q *Query) Substring(field string, pos int) Query {
	return Query(fmt.Sprintf("substring(%s,%d)", field, pos))
}

// ToLower converts the field to lowercase in the Query.
func (q *Query) ToLower(field string) Query {
	return Query(fmt.Sprintf("tolower(%s)", field))
}

// ToUpper converts the field to uppercase in the Query.
func (q *Query) ToUpper(field string) Query {
	return Query(fmt.Sprintf("toupper(%s)", field))
}

// Trim removes leading and trailing spaces from the field in the Query.
func (q *Query) Trim(field string) Query {
	return Query(fmt.Sprintf("trim(%s)", field))
}

// Ceiling returns the ceiling of the field in the Query.
func (q *Query) Ceiling(field string) Query {
	return Query(fmt.Sprintf("ceiling(%s)", field))
}

// Floor returns the floor of the field in the Query.
func (q *Query) Floor(field string) Query {
	return Query(fmt.Sprintf("floor(%s)", field))
}

// Round returns the round of the field in the Query.
func (q *Query) Round(field string) Query {
	return Query(fmt.Sprintf("round(%s)", field))
}
