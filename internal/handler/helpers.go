package handler

import (
	"encoding/json"
	"html/template"
	"log/slog"
	"net/http"

	"github.com/mkrepo-dev/mkrepo/internal/database"
	"github.com/mkrepo-dev/mkrepo/internal/log"
	"github.com/mkrepo-dev/mkrepo/internal/middleware"
)

type baseContext struct {
	Account *database.Account
}

func getBaseContext(r *http.Request) baseContext {
	account := middleware.Account(r.Context())
	return baseContext{Account: account}
}

func render(w http.ResponseWriter, t *template.Template, context any) {
	err := t.Execute(w, context)
	if err != nil {
		slog.Error("Failed to render template", log.Err(err))
		w.WriteHeader(http.StatusInternalServerError)
	}
}

func internalServerError(w http.ResponseWriter, msg string, err error) {
	slog.Error(msg, log.Err(err))
	http.Error(w, "Internal Server Error", http.StatusInternalServerError)
}

// TODO: Unused?
//func loginRedirect(w http.ResponseWriter, r *http.Request, providerKey string, redirectUri string) {
//	query := url.Values{}
//	query.Set("provider", providerKey)
//	query.Set("redirect_uri", redirectUri)
//	redirect := url.URL{
//		Path:     "/auth/login",
//		RawQuery: query.Encode(),
//	}
//
//	http.Redirect(w, r, redirect.String(), http.StatusFound)
//}

func encode[T any](w http.ResponseWriter, v T) {
	w.Header().Set("Content-Type", "application/json")
	err := json.NewEncoder(w).Encode(v)
	if err != nil {
		internalServerError(w, "Failed to encode response", err)
		slog.Error("Failed to encode response", log.Err(err))
	}
}
