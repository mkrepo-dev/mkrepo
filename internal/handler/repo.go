package handler

import (
	"log/slog"
	"net/http"

	"github.com/FilipSolich/mkrepo/internal"
	"github.com/FilipSolich/mkrepo/internal/config"
	"github.com/FilipSolich/mkrepo/internal/db"
	"github.com/FilipSolich/mkrepo/internal/log"
	"github.com/FilipSolich/mkrepo/internal/middleware"
	"github.com/FilipSolich/mkrepo/internal/provider"
	"github.com/FilipSolich/mkrepo/internal/repo"
	"github.com/FilipSolich/mkrepo/internal/template"
)

type Repo struct {
	cfg       config.Config
	db        *db.DB
	providers provider.Providers
}

func NewRepo(cfg config.Config, db *db.DB, providers provider.Providers) *Repo {
	return &Repo{cfg: cfg, db: db, providers: providers}
}

func (h *Repo) Form(w http.ResponseWriter, r *http.Request) {
	providerKey, username := splitProviderUser(r)
	accounts := middleware.Accounts(r.Context())
	account := db.GetAccount(accounts, providerKey, username)

	if account == nil {
		if len(accounts) == 0 || providerKey != "" {
			loginRedirect(w, r, providerKey, r.URL.String())
			return
		}
		account = &accounts[0]
		providerKey = account.Provider
	}

	provider, ok := h.providers[providerKey]
	if !ok {
		http.Error(w, "unsupported provider", http.StatusBadRequest)
		return
	}

	client, token := provider.NewClient(r.Context(), account.Token, account.RedirectUri)
	account.Token = token
	err := h.db.UpdateAccountToken(r.Context(), middleware.Session(r.Context()), account.Provider, account.Username, account.Token)
	if err != nil {
		slog.Error("Failed to update account token", log.Err(err))
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	owners, err := client.GetRepoOwners(r.Context())
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
	providerKey, username := splitProviderUser(r)
	account := db.GetAccount(middleware.Accounts(r.Context()), providerKey, username)
	// TODO: Handler if account is nil

	repository := internal.Repo{
		Account:      *account,
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
	}

	provider, ok := h.providers[providerKey]
	if !ok {
		http.Error(w, "unsupported provider", http.StatusBadRequest)
		return
	}

	url, err := repo.CreateNewRepo(r.Context(), h.db, repository, provider)
	if err != nil {
		slog.Error("Failed to create repository", log.Err(err))
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, url, http.StatusFound)
}
