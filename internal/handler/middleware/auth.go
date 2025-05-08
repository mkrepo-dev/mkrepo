package middleware

import (
	"context"
	"net/http"

	"github.com/mkrepo-dev/mkrepo/internal/database"
	"github.com/mkrepo-dev/mkrepo/internal/handler/cookie"
)

type accountContextKey string

const accountKey accountContextKey = "account"

func Account(ctx context.Context) *database.Account {
	account, ok := ctx.Value(accountKey).(database.Account)
	if !ok {
		return nil
	}
	return &account
}

func SetAccount(ctx context.Context, account database.Account) context.Context {
	return context.WithValue(ctx, accountKey, account)
}

// Try to get user based on session and store it in context if authenticated. Context account
// will be nil if not authenticated.
func Authenticate(db *database.DB) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()
			sessionCookie, err := r.Cookie("session")
			if err == nil {
				account, err := db.GetAccountByValidSession(ctx, sessionCookie.Value)
				if err != nil {
					http.SetCookie(w, cookie.NewDeleteCookie("session"))
				} else {
					// TODO: Hydrate session
					ctx = SetAccount(ctx, account)
				}
			}
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// Checks if user is authenticated. If not, redirect to login page. Must be called after
// Authenticate middleware.
func MustAuthenticate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		account := Account(r.Context())
		if account == nil {
			if r.Method == http.MethodGet {
				http.SetCookie(w, cookie.NewCookie("redirect_uri", r.URL.String(), 5*60))
			}
			http.Redirect(w, r, "/auth/login", http.StatusFound)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func MetricsAuth(token string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if token != "" && r.Header.Get("Authorization") != "Bearer "+token {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
