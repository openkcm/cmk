package daemon

import (
	"net/http"
	"strings"

	"github.tools.sap/kms/cmk/internal/authz"
)

type ServeMux struct {
	httpServeMux http.ServeMux
	BaseURL      string
}

func NewServeMux(baseURL string) *ServeMux {
	return &ServeMux{
		httpServeMux: http.ServeMux{},
		BaseURL:      baseURL,
	}
}

func (m *ServeMux) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	m.httpServeMux.ServeHTTP(w, r)
}

func (m *ServeMux) HandleFunc(
	pattern string,
	handler func(http.ResponseWriter, *http.Request),
) {
	p := strings.Replace(pattern, m.BaseURL, "", 1)

	_, restricted := authz.RestrictionsByAPI[p]
	_, allowed := authz.AllowListByAPI[p]

	if !restricted && !allowed {
		panic("pattern not registered in restrictions or allow list: " + p)
	}

	m.httpServeMux.HandleFunc(pattern, handler)
}
