package handler

import (
	"net/http"
	"strconv"
	"time"

	"github.com/mkrepo-dev/mkrepo/internal/db"
	"github.com/mkrepo-dev/mkrepo/internal/middleware"
	"github.com/mkrepo-dev/mkrepo/internal/mkrepo"
	"github.com/mkrepo-dev/mkrepo/internal/provider"
	"github.com/mkrepo-dev/mkrepo/template"
)

type Repo struct {
	db        *db.DB
	repomaker *mkrepo.RepoMaker
	providers provider.Providers
	licenses  template.Licenses
}

func NewRepo(db *db.DB, repomaker *mkrepo.RepoMaker, providers provider.Providers, licenses template.Licenses) *Repo {
	return &Repo{db: db, repomaker: repomaker, providers: providers, licenses: licenses}
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
		internalServerError(w, "Failed to update account token", err)
		return
	}

	owners, err := client.GetPosibleRepoOwners(r.Context())
	if err != nil {
		internalServerError(w, "Failed to get possible repo owners", err)
		return
	}

	context := template.NewRepoFormContext{
		BaseContext:      getBaseContext(r),
		Providers:        h.providers,
		Owners:           owners,
		Name:             r.FormValue("name"),
		SelectedProvider: providerKey,
		Licenses:         h.licenses,
		CurrentYear:      time.Now().Year(),
	}
	template.Render(w, template.NewRepo, context)
}

func (h *Repo) Create(w http.ResponseWriter, r *http.Request) {
	providerKey, username := splitProviderUser(r)
	account := db.GetAccount(middleware.Accounts(r.Context()), providerKey, username)
	// TODO: Handler if account is nil
	// TODO: Do better validation of input values

	err := r.ParseForm()
	if err != nil {
		http.Error(w, "failed to parse form", http.StatusBadRequest)
		return
	}

	var tag string
	if r.Form.Has("tag") {
		tag = "v0.0.0"
	}
	var license string
	if r.Form.Has("license") {
		license = r.FormValue("license")
	}
	licenseYear, err := strconv.Atoi(r.FormValue("license-year"))
	if err != nil {
		http.Error(w, "invalid license year", http.StatusBadRequest)
		return
	}

	repository := mkrepo.CreateRepo{
		Account:      *account,
		Namespace:    r.FormValue("owner"),
		Name:         r.FormValue("name"),
		Description:  r.FormValue("description"),
		Visibility:   provider.RepoVisibility(r.FormValue("visibility")),
		Readme:       r.Form.Has("readme"),
		Gitignore:    r.FormValue("gitignore"),
		Dockerfile:   r.FormValue("dockerfile"),
		Dockerignore: r.Form.Has("dockerignore"),
		LicenseKey:   license,
		LicenseContext: template.LicenseContext{
			Year:     licenseYear,
			Fullname: r.FormValue("license-fullname"),
			Project:  r.FormValue("license-project"),
		},
		Tag:        tag,
		Sha256:     r.Form.Has("sha256"),
		IsTemplate: r.Form.Has("template"),
	}

	provider, ok := h.providers[providerKey]
	if !ok {
		http.Error(w, "unsupported provider", http.StatusBadRequest)
		return
	}

	// Spawn goroutine to create new repo

	url, err := h.repomaker.CreateNewRepo(r.Context(), repository, provider)
	if err != nil {
		internalServerError(w, "Failed to create repository", err)
		return
	}

	http.Redirect(w, r, url, http.StatusFound)
}
