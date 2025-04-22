package handler

import (
	"net/http"
	"time"

	database "github.com/mkrepo-dev/mkrepo/internal/db"
	"github.com/mkrepo-dev/mkrepo/internal/middleware"
	"github.com/mkrepo-dev/mkrepo/internal/mkrepo"
	"github.com/mkrepo-dev/mkrepo/internal/provider"
	"github.com/mkrepo-dev/mkrepo/internal/types"
	"github.com/mkrepo-dev/mkrepo/template"
)

func MkrepoForm(db *database.DB, providers provider.Providers, licenses template.Licenses) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		providerKey, username := splitProviderUser(r)
		accounts := middleware.Accounts(r.Context())
		account := database.GetAccount(accounts, providerKey, username) // TODO: Move into middleware

		if account == nil {
			if len(accounts) == 0 || providerKey != "" {
				loginRedirect(w, r, providerKey, r.URL.String())
				return
			}
			account = &accounts[0]
			providerKey = account.Provider
		}

		provider, ok := providers[providerKey]
		if !ok {
			http.Error(w, "unsupported provider", http.StatusBadRequest)
			return
		}

		client := provider.NewClient(r.Context(), account.Token, account.RedirectUri)
		account.Token = client.Token()
		err := db.UpdateAccountToken(r.Context(), middleware.Session(r.Context()), account.Provider, account.Username, account.Token)
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
			Providers:        providers,
			Owners:           owners,
			Name:             r.FormValue("name"),
			SelectedProvider: providerKey,
			Licenses:         licenses,
			CurrentYear:      time.Now().Year(),
		}
		template.Render(w, template.NewRepo, context)
	})
}

func MkrepoCreate(db *database.DB, repomaker *mkrepo.RepoMaker, providers provider.Providers, licenses template.Licenses) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		providerKey, username := splitProviderUser(r)
		account := database.GetAccount(middleware.Accounts(r.Context()), providerKey, username)
		// TODO: Handler if account is nil
		// TODO: Do better validation of input values

		repo, err := types.CreateRepoFromForm(r)
		if err != nil {
			http.Error(w, "invalid form", http.StatusBadRequest)
			return
		}

		// TODO: Do provider validation in middleware
		provider, ok := providers[providerKey]
		if !ok {
			http.Error(w, "unsupported provider", http.StatusBadRequest)
			return
		}
		client := provider.NewClient(r.Context(), account.Token, account.RedirectUri)
		account.Token = client.Token()
		err = db.UpdateAccountToken(r.Context(), account.Session, account.Provider, account.Username, account.Token)
		if err != nil {
			internalServerError(w, "Failed to update token in db", err)
			return
		}

		url, err := repomaker.CreateNewRepo(r.Context(), client, repo)
		if err != nil {
			internalServerError(w, "Failed to create repository", err)
			return
		}

		http.Redirect(w, r, url, http.StatusFound)
	})
}
