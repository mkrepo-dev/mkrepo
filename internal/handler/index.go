package handler

import (
	"html/template"
	"net/http"

	tmpl "github.com/FilipSolich/mkrepo/internal/template"
)

type Index struct {
	t *template.Template
}

func NewIndex() http.Handler {
	return &Index{
		t: template.Must(template.New("base.html").ParseFS(tmpl.TemplatesFS, "base.html", "index.html")),
	}
}

func (h *Index) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	tmpl.Render(w, h.t, nil)
}
