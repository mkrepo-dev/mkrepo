package handler

import (
	"log/slog"
	"net/http"
	"net/url"
	"strings"

	"github.com/FilipSolich/mkrepo/internal/log"
	"github.com/FilipSolich/mkrepo/internal/middleware"
	"github.com/FilipSolich/mkrepo/template"
)

func getBaseContext(r *http.Request) template.BaseContext {
	accounts := middleware.Accounts(r.Context())
	return template.BaseContext{Accounts: accounts}
}

func internalServerError(w http.ResponseWriter, msg string, err error) {
	slog.Error(msg, log.Err(err))
	http.Error(w, "Internal Server Error", http.StatusInternalServerError)
}

func splitProviderUser(r *http.Request) (string, string) {
	provider := r.FormValue("provider")
	parts := strings.Split(provider, ":")
	if len(parts) == 2 {
		return parts[0], parts[1]
	}

	return parts[0], ""
}

func loginRedirect(w http.ResponseWriter, r *http.Request, providerKey string, redirectUri string) {
	query := url.Values{}
	query.Set("provider", providerKey)
	query.Set("redirect_uri", redirectUri)
	redirect := url.URL{
		Path:     "/auth/login",
		RawQuery: query.Encode(),
	}

	http.Redirect(w, r, redirect.String(), http.StatusFound)
}
