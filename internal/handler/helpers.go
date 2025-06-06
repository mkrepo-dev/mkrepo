package handler

import (
	"encoding/json"
	"html/template"
	"log/slog"
	"net/http"

	"github.com/mkrepo-dev/mkrepo/internal/database"
	"github.com/mkrepo-dev/mkrepo/internal/handler/middleware"
	"github.com/mkrepo-dev/mkrepo/internal/log"
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
