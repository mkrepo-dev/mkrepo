package middleware

import (
	"context"
	"net/http"

	"github.com/mkrepo-dev/mkrepo/internal/database"
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

func Authenticate(db *database.DB) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()
			token, err := r.Cookie("session")
			if err == nil {
				account, err := db.GetAccountBySession(ctx, token.Value)
				if err != nil {
					token.MaxAge = -1
					http.SetCookie(w, token)
				} else {
					ctx = SetAccount(ctx, account)
				}
			}
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func MustAuthenticate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		account := Account(r.Context())
		if account == nil {
			if r.Method == http.MethodGet {
				cookie := &http.Cookie{
					Name:     "redirecturi",
					Value:    r.URL.String(),
					Path:     "/",
					MaxAge:   5 * 60,
					HttpOnly: true,
					Secure:   true,
					SameSite: http.SameSiteLaxMode,
				}
				http.SetCookie(w, cookie)
			}
			http.Redirect(w, r, "/auth/login", http.StatusFound)
			return
		}
		next.ServeHTTP(w, r)
	})
}
