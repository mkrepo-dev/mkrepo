package handler

import (
	"log/slog"
	"net/http"

	"golang.org/x/oauth2"

	"github.com/FilipSolich/mkrepo/internal"
	"github.com/FilipSolich/mkrepo/internal/config"
	"github.com/FilipSolich/mkrepo/internal/log"
	"github.com/FilipSolich/mkrepo/internal/middleware"
	"github.com/FilipSolich/mkrepo/internal/provider"
	"github.com/FilipSolich/mkrepo/internal/repo"
	"github.com/FilipSolich/mkrepo/internal/template"
)

type Repo struct {
	cfg       config.Config
	providers provider.Providers
}

func NewRepo(cfg config.Config, providers provider.Providers) *Repo {
	return &Repo{cfg: cfg, providers: providers}
}

func (h *Repo) Form(w http.ResponseWriter, r *http.Request) {
	accounts := middleware.Accounts(r.Context())

	providerKey := r.FormValue("provider")
	if providerKey == "" {
		providerKey = h.cfg.DefaultProviderKey
	}
	provider, ok := h.providers[providerKey]
	if !ok {
		http.Error(w, "unsupported provider", http.StatusBadRequest)
		return
	}

	owners, err := provider.NewClient(r.Context(), accounts[0].Token).GetPossibleRepoOwners(r.Context()) // TODO: Dont use [0]
	if err != nil {
		slog.Error("Failed to get possible repo owners", log.Err(err))
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	context := template.NewRepoFormContext{
		BaseContext:      getBaseContext(r),
		Providers:        h.providers,
		Owners:           owners,
		Name:             r.FormValue("name"),
		SelectedProvider: providerKey,
	}
	template.Render(w, template.NewRepoForm, context)
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
		IsTemplate:   r.FormValue("template") == "checked",
		//IsTemplate:   r.FormValue("is_template") == "checked",
		//Sha256:       r.FormValue("sha256") == "checked",
		AuthToken: session,
	}

	providerKey := r.FormValue("provider")
	provider, ok := h.providers[providerKey]
	if !ok {
		http.Error(w, "unsupported provider", http.StatusBadRequest)
		return
	}

	url, err := repo.CreateNewRepo(r.Context(), repository, provider.NewClient(r.Context(), &oauth2.Token{AccessToken: session}))
	if err != nil {
		slog.Error("Failed to create repository", log.Err(err))
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, url, http.StatusFound)
}
