package handler

import (
	"html/template"
	"log/slog"
	"net/http"

	"github.com/FilipSolich/mkrepo/internal/log"
	"github.com/FilipSolich/mkrepo/internal/middleware"
	tmpl "github.com/FilipSolich/mkrepo/internal/template"
	"github.com/FilipSolich/mkrepo/pkg/provider"
)

type New struct {
	t *template.Template
}

func NewNew() *New {
	return &New{
		t: template.Must(template.New("base.html").ParseFS(tmpl.TemplatesFS, "base.html", "new.html")),
	}
}

func (h *New) NewRepoForm(w http.ResponseWriter, r *http.Request) {
	session := middleware.Session(r.Context())
	owners, err := provider.NewGitHub(session).GetPossibleRepoOwners(r.Context())
	if err != nil {
		slog.Error("Failed to get possible repo owners", log.Err(err))
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	context := struct {
		Name      string
		Providers map[string]struct {
			Name     string
			Selected bool
		}
		Owners []string
	}{
		Name: r.FormValue("name"),
		Providers: map[string]struct {
			Name     string
			Selected bool
		}{
			"github": {Name: "GitHub", Selected: false},
			"gitlab": {Name: "GitLab", Selected: false},
		},
		Owners: owners,
	}
	selectedProvider := r.FormValue("provider")
	if selectedProvider != "" {
		val, ok := context.Providers[selectedProvider]
		if ok {
			val.Selected = true
			context.Providers[selectedProvider] = val
		}
	}
	tmpl.Render(w, h.t, context)
}

func (h *New) CreateNewRepo(w http.ResponseWriter, r *http.Request) {
	session := middleware.Session(r.Context())
	slog.Info("Session token", slog.String("session", session))
	token := r.FormValue("description")
	err := provider.NewGitHub(token).CreateRepo(r.Context(), provider.NewRepo{Name: r.FormValue("name"), Owner: ""})
	if err != nil {
		slog.Error("Failed to create repository", log.Err(err))
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/", http.StatusFound)
}
