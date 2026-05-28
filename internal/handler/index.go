package handler

import (
	"html/template"
	"log/slog"
	"net/http"

	"github.com/mkrepo-dev/mkrepo/internal/provider"
	"github.com/mkrepo-dev/mkrepo/template/html"
)

func Index(logger *slog.Logger, providers provider.Providers) http.Handler {
	logger = handlerLogger(logger, "Index")
	type indexContext struct {
		baseContext
	}
	tmpl := template.Must(template.ParseFS(html.FS, "base.html", "index.html"))

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		render(ctx, logger, w, tmpl, indexContext{
			baseContext: getBaseContext(ctx),
		})
	})
}
