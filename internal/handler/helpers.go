package handler

import (
	"context"
	"encoding/json"
	"html/template"
	"log/slog"
	"net/http"

	"github.com/mkrepo-dev/mkrepo/internal/log"
)

type baseContext struct {
	Account *Account
}

func getBaseContext(ctx context.Context) baseContext {
	return baseContext{Account: getAccountFromContext(ctx)}
}

func handlerLogger(logger *slog.Logger, handlerName string) *slog.Logger {
	return logger.With("handler", handlerName)
}

func internalServerError(w http.ResponseWriter) {
	http.Error(w, "Internal Server Error", http.StatusInternalServerError)
}

func render(ctx context.Context, logger *slog.Logger, w http.ResponseWriter, t *template.Template, context any) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	err := t.Execute(w, context)
	if err != nil {
		logger.ErrorContext(ctx, "Failed to render template", log.Err(err))
		internalServerError(w)
	}
}

func encode[T any](ctx context.Context, logger *slog.Logger, w http.ResponseWriter, v T) {
	w.Header().Set("Content-Type", "application/json")
	err := json.NewEncoder(w).Encode(v)
	if err != nil {
		logger.ErrorContext(ctx, "Failed to encode response", log.Err(err))
		internalServerError(w)
	}
}
