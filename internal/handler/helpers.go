package handler

import (
	"net/http"
	"strings"

	"github.com/FilipSolich/mkrepo/internal/middleware"
	"github.com/FilipSolich/mkrepo/internal/template"
)

func getBaseContext(r *http.Request) template.BaseContext {
	accounts := middleware.Accounts(r.Context())
	return template.BaseContext{Accounts: accounts}
}

func splitProviderUser(r *http.Request) (string, string) {
	provider := r.FormValue("provider")
	parts := strings.Split(provider, ":")
	if len(parts) == 2 {
		return parts[0], parts[1]
	}

	return parts[0], ""
}
