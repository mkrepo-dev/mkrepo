package handler

import (
	"maps"
	"net/http"

	"github.com/mkrepo-dev/mkrepo/internal/middleware"
	"github.com/mkrepo-dev/mkrepo/internal/provider"
	"github.com/mkrepo-dev/mkrepo/template"
)

type Index struct {
	providers provider.Providers
}

func NewIndex(providers provider.Providers) http.Handler {
	return &Index{providers: providers}
}

func (h *Index) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	unauthenticatedProviders := maps.Clone(h.providers)
	for _, account := range middleware.Accounts(r.Context()) {
		delete(unauthenticatedProviders, account.Provider)
	}
	template.Render(w, template.Index, template.IndexContext{
		BaseContext:              getBaseContext(r),
		Providers:                h.providers,
		UnauthenticatedProviders: unauthenticatedProviders,
	})
}
