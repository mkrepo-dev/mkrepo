package middleware

import (
	"context"
	"net/http"
	"net/url"
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
			q := url.Values{}
			q.Set("provider", r.FormValue("provider"))
			q.Set("redirect_uri", r.RequestURI)
			u := url.URL{Path: "/login", RawQuery: q.Encode()}
			http.Redirect(w, r, u.String(), http.StatusFound)
			return
		}
		ctx := SetSession(r.Context(), token.Value)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
