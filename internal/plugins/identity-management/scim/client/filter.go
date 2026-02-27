package client

import (
	"fmt"
	"strings"
)

type FilterOperator string

const (
	FilterOperatorEqual      FilterOperator = "eq"
	FilterOperatorGreater    FilterOperator = "gt"
	FilterOperatorEqualCI    FilterOperator = "eq_ci" // Case-insensitive
	FilterOperatorNotEqual   FilterOperator = "ne"
	FilterOperatorContains   FilterOperator = "co"
	FilterOperatorStartsWith FilterOperator = "sw"
	FilterOperatorEndsWith   FilterOperator = "ew"
)

// FilterExpression is an interface for filter expressions in SCIM.
// It can be a comparison or logical operation.
type FilterExpression interface {
	ToString() string
}

// NullFilterExpression is a placeholder for an empty/nil filter expression.
type NullFilterExpression struct{}

func (f NullFilterExpression) ToString() string {
	return ""
}

// FilterComparison represents a comparison filter expression.
type FilterComparison struct {
	Attribute string
	Operator  FilterOperator
	Value     string
}

func (f FilterComparison) ToString() string {
	return fmt.Sprintf("%s %s \"%s\"", f.Attribute, f.Operator, f.Value)
}

// FilterLogicalGroupAnd represents a logical AND group of filter expressions.
type FilterLogicalGroupAnd struct {
	Expressions []FilterExpression
}

func (f FilterLogicalGroupAnd) ToString() string {
	exprStrings := make([]string, len(f.Expressions))
	for i, expr := range f.Expressions {
		exprStrings[i] = expr.ToString()
	}

	return fmt.Sprintf("(%s)", strings.Join(exprStrings, " and "))
}

// FilterLogicalGroupOr represents a logical OR group of filter expressions.
type FilterLogicalGroupOr struct {
	Expressions []FilterExpression
}

func (f FilterLogicalGroupOr) ToString() string {
	exprStrings := make([]string, len(f.Expressions))
	for i, expr := range f.Expressions {
		exprStrings[i] = expr.ToString()
	}

	return fmt.Sprintf("(%s)", strings.Join(exprStrings, " or "))
}

// FilterLogicalGroupNot represents a logical NOT operation on a filter expression.
type FilterLogicalGroupNot struct {
	Expression FilterExpression
}

func (f FilterLogicalGroupNot) ToString() string {
	return "not " + f.Expression.ToString()
}
