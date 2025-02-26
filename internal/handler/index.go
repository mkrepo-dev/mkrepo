package handler

import (
	"net/http"

	"github.com/FilipSolich/mkrepo/internal/template"
)

type Index struct{}

func NewIndex() http.Handler {
	return &Index{}
}

func (h *Index) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	template.Render(w, template.Index, nil)
}
