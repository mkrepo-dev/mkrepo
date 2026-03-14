package handler

import (
	"errors"
	"html/template"
	"log/slog"
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/mkrepo-dev/mkrepo/internal/adapter"
	"github.com/mkrepo-dev/mkrepo/internal/app"
	"github.com/mkrepo-dev/mkrepo/internal/provider"
	mkrepo "github.com/mkrepo-dev/mkrepo/internal/service"
	"github.com/mkrepo-dev/mkrepo/template/html"
)

func MkrepoForm(db *adapter.Repository, providers provider.Providers, gitignores []string, licenses mkrepo.Licenses) http.Handler {
	type newRepoFormContext struct {
		baseContext
		Name        string
		Provider    provider.Provider
		Owners      []provider.RepoOwner
		Gitignores  []string
		Licenses    mkrepo.Licenses
		CurrentYear int
	}
	tmpl := template.Must(template.ParseFS(html.FS, "base.html", "new.html"))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		account := app.GetAccountFromContext(r.Context())
		provider := providers[account.Provider]
		client := provider.NewClient(r.Context(), account.Session.Token)

		// TODO: Assume that token is valid during whole request. Maybe assure this in middleware.
		account.Session.Token = client.Token()
		err := db.UpdateAccountWithSession(r.Context(), *account)
		if err != nil {
			internalServerError(w, "Failed to update account token", err)
			return
		}

		owners, err := client.GetPosibleRepoOwners(r.Context())
		if err != nil {
			internalServerError(w, "Failed to get possible repo owners", err)
			return
		}

		context := newRepoFormContext{
			baseContext: getBaseContext(r),
			Provider:    provider,
			Owners:      owners,
			Name:        r.FormValue("name"),
			Gitignores:  gitignores,
			Licenses:    licenses,
			CurrentYear: time.Now().Year(),
		}
		render(w, tmpl, context)
	})
}

func MkrepoCreate(db *adapter.Repository, repomaker *mkrepo.MkrepoService, providers provider.Providers) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		account := app.GetAccountFromContext(r.Context())
		// TODO: Do better validation of input values

		repo, err := CreateRepoFromForm(r)
		if err != nil {
			slog.Warn("Failed to parse form", "error", err)
			http.Error(w, "invalid form", http.StatusBadRequest)
			return
		}

		provider, ok := providers[account.Provider]
		if !ok {
			http.Error(w, "unsupported provider", http.StatusBadRequest)
			return
		}
		client := provider.NewClient(r.Context(), account.Session.Token)
		account.Session.Token = client.Token()
		err = db.UpdateAccountWithSession(r.Context(), *account)
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

func Schemas(licenses mkrepo.Licenses) http.Handler {
	type schemasResponse struct {
		Licenses mkrepo.Licenses `json:"licenses"`
	}
	response := schemasResponse{Licenses: licenses}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		encode(w, response)
	})
}

func CreateRepoFromForm(r *http.Request) (*mkrepo.CreateRepo, error) {
	err := r.ParseForm()
	if err != nil {
		return nil, errors.New("invalid form")
	}

	// TODO: Do validation somewhere else. Just set values here.
	name := r.FormValue("name")
	if name == "" {
		return nil, errors.New("name is required")
	}
	namespace := r.FormValue("owner")
	var description *string
	descriptionStr := r.FormValue("description")
	if descriptionStr != "" {
		description = &descriptionStr
	}
	var visibility *mkrepo.CreateRepoVisibility
	formVisibility := mkrepo.CreateRepoVisibility(r.FormValue("visibility"))
	if !slices.Contains([]mkrepo.CreateRepoVisibility{mkrepo.Private, mkrepo.Public}, formVisibility) {
		return nil, errors.New("invalid visibility")
	}
	visibility = &formVisibility

	var sha256 *bool
	if r.Form.Has("sha256") {
		sha256 = new(true)
	}

	account := app.GetAccountFromContext(r.Context())
	var tag *string
	if r.Form.Has("tag") {
		tag = new("v0.0.0")
	}
	var template *mkrepo.CreateRepoTemplate
	templateStr := r.FormValue("template")
	if templateStr != "" {
		nameVersion := strings.Split(templateStr, "@")
		if len(nameVersion) != 2 {
			return nil, errors.New("invalid template name version")
		}
		template = &mkrepo.CreateRepoTemplate{
			FullName: nameVersion[0],
		}
	}

	var readme *bool
	if r.Form.Has("readme") {
		readme = new(true)
	}
	var gitignore *string
	gitignoreStr := r.FormValue("gitignore")
	if gitignoreStr != "" {
		gitignore = &gitignoreStr
	}
	var license mkrepo.LicenseKey
	var values map[string]any
	licenseStr := r.FormValue("license")
	if licenseStr != "" {
		license = mkrepo.LicenseKey(licenseStr)
		var licenseValues map[string]string
		for key, vals := range r.Form {
			if strings.HasPrefix(key, "license-") && len(vals) > 0 && vals[0] != "" {
				if licenseValues == nil {
					licenseValues = make(map[string]string)
				}
				licenseValues[strings.TrimPrefix(key, "license-")] = vals[0]
			}
		}
		if licenseValues != nil {
			values = map[string]any{"License": licenseValues}
		}
	}

	repo := &mkrepo.CreateRepo{
		Name:        name,
		Namespace:   namespace,
		Description: description,
		Visibility:  visibility,
		Sha256:      sha256,
		Initialize: &mkrepo.CreateRepoInitialize{
			Author: mkrepo.CreateRepoInitializeAuthor{
				Name:  account.DisplayName,
				Email: account.Email,
			},
			Template:  template,
			Tag:       tag,
			Readme:    readme,
			Gitignore: gitignore,
			License:   &license,
			Values:    values,
		},
	}

	return repo, nil
}
