package odata

import (
	"fmt"

	"github.com/openkcm/cmk/internal/repo"
)

type SearchItem struct {
	Conditional string // "" for first, then "and" or "or" (connector before it)
	Value       string
}

type Search struct {
	Items  []SearchItem
	Fields []repo.QueryField
}

func NewSearch(search *string, fields ...repo.QueryField) (*Search, error) {
	f := &Search{Fields: fields}
	if search == nil || len(*search) == 0 || len(fields) == 0 {
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
	if len(f.Items) == 0 || len(f.Fields) == 0 {
		return query, nil
	}

	for _, item := range f.Items {
		ck := repo.NewCompositeKey()
		ck.IsStrict = false

		for _, field := range f.Fields {
			ck.Conds = append(ck.Conds, repo.Condition{
				Field: fmt.Sprintf("CAST(%s AS TEXT)", field),
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

	// If there is an even ammount of tokens it's invalid for search
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
