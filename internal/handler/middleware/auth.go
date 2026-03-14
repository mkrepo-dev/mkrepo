package middleware

import (
	"crypto/subtle"
	"log/slog"
	"net/http"
	"net/url"
	"strings"

	"github.com/mkrepo-dev/mkrepo/internal/app"
	"github.com/mkrepo-dev/mkrepo/internal/handler"
)

func Authenticate(logger *slog.Logger, authService *app.AuthService) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()
			sessionCookie, err := r.Cookie(handler.SessionCookieName)
			if err == nil {
				account, err := authService.Authenticate(ctx, sessionCookie.Value)
				if err != nil {
					handler.Logout(logger, authService)(w, r)
					return
				}
				ctx = app.ContextWithAccount(ctx, account)
			}
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

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

func MetricsAuth(token string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if token != "" {
				auth, _ := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
				if subtle.ConstantTimeCompare([]byte(auth), []byte(token)) == 0 {
					w.WriteHeader(http.StatusUnauthorized)
					return
				}
			}
			next.ServeHTTP(w, r)
		})
	}
}
