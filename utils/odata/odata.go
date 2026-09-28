package odata

import (
	"github.com/openkcm/cmk/internal/constants"
	"github.com/openkcm/cmk/internal/repo"
	"github.com/openkcm/cmk/utils/ptr"
)

type OData struct {
	skip   *int
	top    *int
	count  *bool
	filter *string
	search *string

	repoToFilterMap FilterToRepoMap
	loadedFilter    *Filter
}

type Option func(*OData)

func WithPagination(skip *int, top *int, count *bool) Option {
	return func(o *OData) {
		o.skip = skip
		o.top = top
		o.count = count
	}
}

func WithFilter(filter *string, repoToFilterMap FilterToRepoMap) Option {
	return func(o *OData) {
		o.filter = filter
		o.repoToFilterMap = repoToFilterMap
	}
}

func WithSearch(filter *string) Option {
	return func(o *OData) {
		o.search = filter
	}
}

func New(opts ...Option) *OData {
	oData := &OData{}
	for _, o := range opts {
		o(oData)
	}
	return oData
}

func (o *OData) GetPagination() repo.Pagination {
	return repo.Pagination{
		Skip:  ptr.GetPtrOrDefault(o.skip, constants.DefaultSkip),
		Top:   ptr.GetPtrOrDefault(o.top, constants.DefaultTop),
		Count: ptr.GetSafeDeref(o.count),
	}
}

func (o *OData) GetFilter() (repo.QueryFilter, error) {
	if o.loadedFilter != nil {
		return o.loadedFilter, nil
	}
	f, err := NewFilter(o.filter, o.repoToFilterMap)
	if err != nil {
		return nil, err
	}
	o.loadedFilter = f
	return f, nil
}
