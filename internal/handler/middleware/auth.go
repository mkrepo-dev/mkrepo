package middleware

import (
	"net/http"
	"net/url"

	"github.com/mkrepo-dev/mkrepo/internal/app"
)

// Checks if user is authenticated. If not, redirect to login page. Must be called after
// Authenticate middleware.
func MustAuthenticate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		account := app.GetAccountFromContext(r.Context())
		if account == nil {
			redirect := "/auth/login"
			if r.Method == http.MethodGet {
				redirect += "?redirect=" + url.QueryEscape(r.URL.String())
			}
			http.Redirect(w, r, redirect, http.StatusFound)
			return
		}
		next.ServeHTTP(w, r)
	})
}
