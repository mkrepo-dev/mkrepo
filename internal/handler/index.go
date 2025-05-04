package handler

import (
	"html/template"
	"net/http"

	"github.com/mkrepo-dev/mkrepo/internal/provider"
	"github.com/mkrepo-dev/mkrepo/template/html"
)

func Index(providers provider.Providers) http.Handler {
	type indexContext struct {
		baseContext
	}
	tmpl := template.Must(template.ParseFS(html.FS, "base.html", "index.html"))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		render(w, tmpl, indexContext{
			baseContext: getBaseContext(r),
		})
	})
}
