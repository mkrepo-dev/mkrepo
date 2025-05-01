package handler

import (
	"net/http"

	"github.com/mkrepo-dev/mkrepo/internal/database"
)

func Templates(db *database.DB) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		query := r.FormValue("q")
		// TODO: Handle empty query and len(query) == 1

		templates, err := db.SearchTemplates(r.Context(), query)
		if err != nil {
			internalServerError(w, "Failed to search templates", err)
			return
		}

		encode(w, templates)
	})
}
