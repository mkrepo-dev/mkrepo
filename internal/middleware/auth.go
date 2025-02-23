package middleware

import (
	"context"
	"net/http"
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

func Authenticated(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token, err := r.Cookie("session")
		if err != nil {
			http.Error(w, "unauthorized", http.StatusUnauthorized) // TODO: Find better way to redirect on provider login and back
			return
		}
		ctx := SetSession(r.Context(), token.Value)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
