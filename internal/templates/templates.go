package templates

import (
	"embed"
	"html/template"
	"log/slog"
	"net/http"

	"github.com/FilipSolich/mkrepo/internal/log"
)

//go:embed *.html
var TemplatesFS embed.FS

func Render(w http.ResponseWriter, t *template.Template, context any) {
	err := t.Execute(w, context)
	if err != nil {
		slog.Error("Failed to render template", log.Err(err))
		w.WriteHeader(http.StatusInternalServerError)
	}
}
