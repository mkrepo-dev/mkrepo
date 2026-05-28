package handler

import (
	"log/slog"
	"net/http"

	"github.com/mkrepo-dev/mkrepo/internal/adapter"
	mkrepo "github.com/mkrepo-dev/mkrepo/internal/service"
)

func Templates(logger *slog.Logger, db *adapter.Repository) http.Handler {
	logger = handlerLogger(logger, "Templates")
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		query := r.FormValue("q")
		// TODO: Handle empty query and len(query) == 1

		templates, err := db.SearchTemplates(r.Context(), query)
		if err != nil {
			internalServerError(w)
			return
		}

		encode(ctx, logger, w, templates)
	})
}

func RegisterTemplate(repomaker *mkrepo.MkrepoService) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		url := r.FormValue("url")
		if url == "" {
			http.Error(w, "url is required", http.StatusBadRequest)
			return
		}
		fullName := r.FormValue("full_name")
		if fullName == "" {
			http.Error(w, "full_name is required", http.StatusBadRequest)
			return
		}

		err := repomaker.RegisterTemplate(r.Context(), url, fullName)
		if err != nil {
			internalServerError(w)
			return
		}

		w.WriteHeader(http.StatusCreated)
	})
}
