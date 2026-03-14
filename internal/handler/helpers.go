package handler

import (
	"encoding/json"
	"html/template"
	"log/slog"
	"net/http"

	"github.com/mkrepo-dev/mkrepo/internal/app"
	"github.com/mkrepo-dev/mkrepo/internal/log"
)

type baseContext struct {
	Account *app.Account
}

func getBaseContext(r *http.Request) baseContext {
	return baseContext{Account: app.GetAccountFromContext(r.Context())}
}

func render(w http.ResponseWriter, t *template.Template, context any) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	err := t.Execute(w, context)
	if err != nil {
		slog.Error("Failed to render template", log.Err(err))
		w.WriteHeader(http.StatusInternalServerError)
	}
}

func encode[T any](w http.ResponseWriter, v T) {
	w.Header().Set("Content-Type", "application/json")
	err := json.NewEncoder(w).Encode(v)
	if err != nil {
		internalServerError(w, "Failed to encode response", err)
		slog.Error("Failed to encode response", log.Err(err))
	}
}

func internalServerError(w http.ResponseWriter, msg string, err error) {
	slog.Error(msg, log.Err(err))
	http.Error(w, "Internal Server Error", http.StatusInternalServerError)
}
