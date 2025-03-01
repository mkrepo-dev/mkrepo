package handler

import (
	"net/http"

	"github.com/FilipSolich/mkrepo/internal/provider"
	"github.com/FilipSolich/mkrepo/internal/template"
)

type Index struct {
	providers provider.Providers
}

func NewIndex(providers provider.Providers) http.Handler {
	return &Index{providers: providers}
}

func (h *Index) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	template.Render(w, template.Index, template.IndexContext{Providers: h.providers})
}
