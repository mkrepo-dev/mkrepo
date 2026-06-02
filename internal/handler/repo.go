package handler

import (
	"errors"
	"html/template"
	"log/slog"
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/mkrepo-dev/mkrepo/internal/database"
	"github.com/mkrepo-dev/mkrepo/internal/provider"
	mkrepo "github.com/mkrepo-dev/mkrepo/internal/service"
	"github.com/mkrepo-dev/mkrepo/template/html"
)

func MkrepoForm(logger *slog.Logger, db *database.DB, providers provider.Providers, gitignores []string, licenses mkrepo.Licenses) http.Handler {
	logger = handlerLogger(logger, "MkrepoForm")
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
		ctx := r.Context()
		account := getAccountFromContext(ctx)
		provider := providers[account.ProviderKey]
		client := provider.NewClient(ctx, account.Token)

		// TODO: Assume that token is valid during whole request. Maybe assure this in middleware.
		//account.Session.Token = client.Token()
		//err := db.UpdateAccountWithSession(ctx, *account)
		//if err != nil {
		//	internalServerError(w)
		//	return
		//}

		owners, err := client.GetPosibleRepoOwners(ctx)
		if err != nil {
			internalServerError(w)
			return
		}

		context := newRepoFormContext{
			baseContext: getBaseContext(ctx),
			Provider:    provider,
			Owners:      owners,
			Name:        r.FormValue("name"),
			Gitignores:  gitignores,
			Licenses:    licenses,
			CurrentYear: time.Now().Year(),
		}
		render(ctx, logger, w, tmpl, context)
	})
}

func MkrepoCreate(logger *slog.Logger, db *database.DB, repomaker *mkrepo.MkrepoService, providers provider.Providers) http.Handler {
	logger = handlerLogger(logger, "MkrepoCreate")

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		account := getAccountFromContext(ctx)
		// TODO: Do better validation of input values

		repo, err := CreateRepoFromForm(r)
		if err != nil {
			logger.WarnContext(ctx, "Failed to parse form", "error", err)
			http.Error(w, "invalid form", http.StatusBadRequest)
			return
		}

		provider, ok := providers[account.ProviderKey]
		if !ok {
			http.Error(w, "unsupported provider", http.StatusBadRequest)
			return
		}
		client := provider.NewClient(ctx, account.Token)
		//account.Session.Token = client.Token()
		//err = db.UpdateAccountWithSession(ctx, *account)
		//if err != nil {
		//	internalServerError(w)
		//	return
		//}

		url, err := repomaker.CreateNewRepo(ctx, client, repo)
		if err != nil {
			internalServerError(w)
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
		encode(r.Context(), slog.Default(), w, response) // TODO: Don't use default logger
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

	account := getAccountFromContext(r.Context())
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
