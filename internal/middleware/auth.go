package middleware

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/mkrepo-dev/mkrepo/internal/db"
	"github.com/mkrepo-dev/mkrepo/internal/log"
)

type tokenContextKey string

const tokenKey tokenContextKey = "token"

func SetSession(ctx context.Context, token string) context.Context {
	return context.WithValue(ctx, tokenKey, token)
}

func Session(ctx context.Context) string {
	session, _ := ctx.Value(tokenKey).(string)
	return session
}

type accountsContextKey string

const accountsKey accountsContextKey = "accounts"

func Accounts(ctx context.Context) []db.Account {
	accounts, _ := ctx.Value(accountsKey).([]db.Account)
	return accounts
}

func SetAccounts(ctx context.Context, accounts []db.Account) context.Context {
	return context.WithValue(ctx, accountsKey, accounts)
}

func NewAuthenticate(db *db.DB) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var session string
			ctx := r.Context()
			token, err := r.Cookie("session")
			if err != nil {
				ctx = SetAccounts(ctx, nil)
			} else {
				session = token.Value
				accounts, err := db.GetSessionAccounts(ctx, session)
				if err != nil {
					slog.Error("Failed to get accounts", log.Err(err))
					w.WriteHeader(http.StatusInternalServerError)
					return
				}
				ctx = SetAccounts(ctx, accounts)
			}
			ctx = SetSession(ctx, session)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
