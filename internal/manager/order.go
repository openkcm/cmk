package manager

import (
	"github.com/openkcm/cmk/internal/repo"
)

func applyOrderOrDefault(params repo.Params, query *repo.Query, applyDefault func(*repo.Query)) error {
	if og, ok := params.(repo.OrderGetter); ok {
		joins, order, err := og.GetOrder()
		if err != nil {
			return err
		}

		if len(order) > 0 {
			query.Merge(joins).Order(order...)
			return nil
		}
	}

	applyDefault(query)

	return nil
}
