package handler

import (
	"net/http"
	"time"

	"github.com/mkrepo-dev/mkrepo/internal/db"
	"github.com/mkrepo-dev/mkrepo/internal/middleware"
	"github.com/mkrepo-dev/mkrepo/internal/mkrepo"
	"github.com/mkrepo-dev/mkrepo/internal/provider"
	"github.com/mkrepo-dev/mkrepo/internal/types"
	"github.com/mkrepo-dev/mkrepo/template"
)

type Repo struct {
	db        *db.DB
	repomaker *mkrepo.RepoMaker
	providers provider.Providers
	licenses  template.Licenses
}

// TODO: Rewrite handlers as standalone functions
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

	client := provider.NewClient(r.Context(), account.Token, account.RedirectUri)
	account.Token = client.Token()
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

	repo, err := types.CreateRepoFromForm(r)
	if err != nil {
		http.Error(w, "invalid form", http.StatusBadRequest)
		return
	}

	// TODO: Do provider validation in middleware
	provider, ok := h.providers[providerKey]
	if !ok {
		http.Error(w, "unsupported provider", http.StatusBadRequest)
		return
	}
	client := provider.NewClient(r.Context(), account.Token, account.RedirectUri)
	account.Token = client.Token()
	err = h.db.UpdateAccountToken(r.Context(), account.Session, account.Provider, account.Username, account.Token)
	if err != nil {
		internalServerError(w, "Failed to update token in db", err)
		return
	}

	url, err := h.repomaker.CreateNewRepo(r.Context(), client, repo)
	if err != nil {
		internalServerError(w, "Failed to create repository", err)
		return
	}

	http.Redirect(w, r, url, http.StatusFound)
}
