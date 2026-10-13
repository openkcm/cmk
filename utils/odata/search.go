package odata

import (
	"github.com/openkcm/cmk/internal/repo"
)

type SearchItem struct {
	Conditional string // "" for first, then "and" or "or" (connector before it)
	Value       string
}

type SearchJoinField struct {
	Column string
	Join   repo.JoinCondition
}

type Search struct {
	Items      []SearchItem
	Fields     []repo.QueryField
	JoinFields []SearchJoinField
}

// NewSearch does not support nested OData operations ()
// This currently does not support nested operations ()
// If needed in the future the data structure needs to be swap from a slice to a tree
// As this is not a simple implementation it was skipped for now
func NewSearch(search *string, fields []repo.QueryField, joinFields []SearchJoinField) (*Search, error) {
	f := &Search{Fields: fields, JoinFields: joinFields}
	if search == nil || len(*search) == 0 || (len(fields) == 0 && len(joinFields) == 0) {
		return f, nil
	}

	tokens := tokenise(*search)

	items, err := buildSearchItems(tokens)
	if err != nil {
		return nil, err
	}
	f.Items = items

	return f, nil
}

func (f *Search) GetQuery() (*repo.Query, error) {
	query := repo.NewQuery()
	if len(f.Items) == 0 || (len(f.Fields) == 0 && len(f.JoinFields) == 0) {
		return query, nil
	}

	for _, jf := range f.JoinFields {
		query.Join(repo.LeftJoin, jf.Join)
	}

	for _, item := range f.Items {
		ck := repo.NewCompositeKey()
		ck.IsStrict = false

		for _, field := range f.Fields {
			ck.Conds = append(ck.Conds, repo.Condition{
				Field: field,
				Value: repo.CompositeKeyEntry{
					Key: repo.Key{
						// Between % to act as wildcard
						Value:     "%" + item.Value + "%",
						Operation: repo.Contains,
					},
				},
			})
		}

		for _, jf := range f.JoinFields {
			ck.Conds = append(ck.Conds, repo.Condition{
				Field: jf.Column,
				Value: repo.CompositeKeyEntry{
					Key: repo.Key{
						// Between % to act as wildcard
						Value:     "%" + item.Value + "%",
						Operation: repo.Contains,
					},
				},
			})
		}

		ckg := repo.NewCompositeKeyGroup(ck)
		if item.Conditional == Or {
			ckg.IsStrict = false
		}

		query.Where(ckg)
	}

	return query, nil
}

func buildSearchItems(tokens []string) ([]SearchItem, error) {
	items := make([]SearchItem, 0)
	logic := ""
	i := 0

	// If there is an even amount of tokens it's invalid for search
	// as it's built on VALUE loop([COND VALUE]).
	if len(tokens)%2 == 0 {
		return nil, ErrFilterNotToSpec
	}

	for i < len(tokens) {
		val := tokens[i]
		if isReserved(val) {
			return nil, ErrFilterNotToSpec
		}

		value, err := unquote(val)
		if err != nil {
			return nil, err
		}
		items = append(items, SearchItem{
			Conditional: logic,
			Value:       value,
		})

		i++
		if i == len(tokens) {
			return items, nil
		}

		if tokens[i] != And && tokens[i] != Or {
			return nil, ErrFilterNotToSpec
		}

		logic = tokens[i]
		i++

		if i == len(tokens) {
			return nil, ErrFilterNotToSpec
		}
	}
	return items, nil
}
