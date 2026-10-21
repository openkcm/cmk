package odata

import (
	"strings"

	"github.com/openkcm/cmk/internal/repo"
)

const MaxOrderClauses = 10

// OrderToRepoItem maps a user-facing field to a DB column. Only mapped fields
// reach the repo, which puts the field into ORDER BY unquoted, so the map is
// the guard against injection.
type OrderToRepoItem struct {
	DBName repo.QueryField
}

type OrderToRepoMap map[string]OrderToRepoItem

type OrderJoinField struct {
	UserField string
	Column    string
	Join      repo.JoinCondition
}

type Order struct {
	fields     []repo.OrderField
	joinFields []OrderJoinField
	joins      []repo.JoinClause
}

func NewOrder(orderby *string, m OrderToRepoMap, joinFields []OrderJoinField) (*Order, error) {
	o := &Order{joinFields: joinFields}
	if orderby == nil || len(strings.TrimSpace(*orderby)) == 0 {
		return o, nil
	}

	clauses := strings.Split(*orderby, ",")
	if len(clauses) > MaxOrderClauses {
		return nil, ErrOrderNotToSpec
	}

	for _, clause := range clauses {
		field, dir, err := parseOrderClause(clause)
		if err != nil {
			return nil, err
		}

		if jf, ok := o.lookupJoinField(field); ok {
			o.joins = append(o.joins, repo.JoinClause{Type: repo.LeftJoin, OnCondition: jf.Join})
			o.fields = append(o.fields, repo.OrderField{Field: jf.Column, Direction: dir})
			continue
		}

		item, ok := m[field]
		if !ok {
			return nil, ErrOrderNonSchema
		}

		o.fields = append(o.fields, repo.OrderField{Field: item.DBName, Direction: dir})
	}

	return o, nil
}

// GetOrder returns a query holding the needed joins and the order fields.
// repo.Query.Merge drops OrderFields, so the caller must merge the joins and
// apply the order separately.
func (o *Order) GetOrder() (*repo.Query, []repo.OrderField, error) {
	joinQuery := repo.NewQuery()
	for _, j := range o.joins {
		joinQuery.Join(j.Type, j.OnCondition)
	}

	return joinQuery, o.fields, nil
}

func (o *Order) lookupJoinField(field string) (OrderJoinField, bool) {
	for _, jf := range o.joinFields {
		if jf.UserField == field {
			return jf, true
		}
	}

	return OrderJoinField{}, false
}

func parseOrderClause(clause string) (repo.QueryField, repo.OrderDirection, error) {
	tokens := strings.Fields(clause)
	switch len(tokens) {
	case 1:
		return tokens[0], repo.Asc, nil
	case 2:
		dir, err := parseDirection(tokens[1])
		if err != nil {
			return "", "", err
		}

		return tokens[0], dir, nil
	default:
		return "", "", ErrOrderNotToSpec
	}
}

func parseDirection(s string) (repo.OrderDirection, error) {
	switch strings.ToLower(s) {
	case string(repo.Asc):
		return repo.Asc, nil
	case string(repo.Desc):
		return repo.Desc, nil
	default:
		return "", ErrOrderInvalidDirection
	}
}
