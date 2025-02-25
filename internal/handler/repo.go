package handler

import (
	"html/template"
	"log/slog"
	"net/http"

	"github.com/FilipSolich/mkrepo/internal"
	"github.com/FilipSolich/mkrepo/internal/log"
	"github.com/FilipSolich/mkrepo/internal/middleware"
	"github.com/FilipSolich/mkrepo/internal/provider"
	"github.com/FilipSolich/mkrepo/internal/repo"
	"github.com/FilipSolich/mkrepo/internal/template/html"
)

type Repo struct {
	t *template.Template
}

func NewRepo() *Repo {
	return &Repo{
		t: template.Must(template.New("base.html").ParseFS(html.HtmlFs, "base.html", "new.html")),
	}
}

func (h *Repo) Form(w http.ResponseWriter, r *http.Request) {
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
	html.Render(w, h.t, context)
}

func (h *Repo) Create(w http.ResponseWriter, r *http.Request) {
	session := middleware.Session(r.Context())

	repository := internal.Repo{
		Provider:     r.FormValue("provider"),
		Owner:        r.FormValue("owner"),
		Name:         r.FormValue("name"),
		Description:  r.FormValue("description"),
		Visibility:   r.FormValue("visibility"),
		Readme:       r.FormValue("readme") == "checked",
		Gitignore:    r.FormValue("gitignore"),
		Dockerfile:   r.FormValue("dockerfile"),
		Dockerignore: r.FormValue("dockerignore") == "checked",
		License:      r.FormValue("license"),
		Tag:          r.FormValue("tag"),
		//IsTemplate:   r.FormValue("is_template") == "checked",
		//Sha256:       r.FormValue("sha256") == "checked",
		AuthToken: session,
	}

	url, err := repo.CreateNewRepo(r.Context(), repository, provider.NewGitHub(session))
	if err != nil {
		slog.Error("Failed to create repository", log.Err(err))
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, url, http.StatusFound)
}
