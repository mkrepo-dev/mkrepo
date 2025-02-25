package handler

import (
	"html/template"
	"net/http"

	"github.com/FilipSolich/mkrepo/internal/template/html"
)

type Index struct {
	t *template.Template
}

func NewIndex() http.Handler {
	return &Index{
		t: template.Must(template.New("base.html").ParseFS(html.HtmlFs, "base.html", "index.html")),
	}
}

func (h *Index) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	html.Render(w, h.t, nil)
}
