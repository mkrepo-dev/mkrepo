package handler

import (
	"maps"
	"net/http"

	"github.com/mkrepo-dev/mkrepo/internal/middleware"
	"github.com/mkrepo-dev/mkrepo/internal/provider"
	"github.com/mkrepo-dev/mkrepo/template"
)

func Index(providers provider.Providers) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		unauthenticatedProviders := maps.Clone(providers)
		for _, account := range middleware.Accounts(r.Context()) {
			delete(unauthenticatedProviders, account.Provider)
		}
		template.Render(w, template.Index, template.IndexContext{
			BaseContext:              getBaseContext(r),
			Providers:                providers,
			UnauthenticatedProviders: unauthenticatedProviders,
		})
	})
}
