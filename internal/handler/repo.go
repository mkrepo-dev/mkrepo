package handler

import (
	"errors"
	htmltemplate "html/template"
	"log/slog"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/mkrepo-dev/mkrepo/internal/database"
	"github.com/mkrepo-dev/mkrepo/internal/handler/middleware"
	"github.com/mkrepo-dev/mkrepo/internal/mkrepo"
	"github.com/mkrepo-dev/mkrepo/internal/provider"
	"github.com/mkrepo-dev/mkrepo/internal/types"
	"github.com/mkrepo-dev/mkrepo/template"
	"github.com/mkrepo-dev/mkrepo/template/html"
)

func MkrepoForm(db *database.DB, providers provider.Providers, licenses template.Licenses) http.Handler {
	type newRepoFormContext struct {
		baseContext
		Name        string
		Provider    provider.Provider
		Owners      []provider.RepoOwner
		Licenses    template.Licenses
		CurrentYear int
	}
	tmpl := htmltemplate.Must(htmltemplate.ParseFS(html.FS, "base.html", "new.html"))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		account := middleware.Account(r.Context())
		provider := providers[account.Provider]
		client := provider.NewClient(r.Context(), account.Token)

		// TODO: Assume that token is valid during whole request. Maybe assure this in middleware.
		account.Token = client.Token()
		err := db.UpdateAccountToken(r.Context(), account.Provider, account.Username, account.Token)
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
			Licenses:    licenses,
			CurrentYear: time.Now().Year(),
		}
		render(w, tmpl, context)
	})
}

func MkrepoCreate(db *database.DB, repomaker *mkrepo.RepoMaker, providers provider.Providers, licenses template.Licenses) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		account := middleware.Account(r.Context())
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
		client := provider.NewClient(r.Context(), account.Token)
		account.Token = client.Token()
		err = db.UpdateAccountToken(r.Context(), account.Provider, account.Username, account.Token)
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

func CreateRepoFromForm(r *http.Request) (*types.CreateRepo, error) {
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
	var visibility *types.CreateRepoVisibility
	formVisibility := types.CreateRepoVisibility(r.FormValue("visibility"))
	if !slices.Contains([]types.CreateRepoVisibility{types.Private, types.Public}, formVisibility) {
		return nil, errors.New("invalid visibility")
	}
	visibility = &formVisibility

	var sha256 *bool
	if r.Form.Has("sha256") {
		sha256 = ptr(true)
	}

	account := middleware.Account(r.Context())
	var tag *string
	if r.Form.Has("tag") {
		tag = ptr("v0.0.0")
	}
	var template *types.CreateRepoTemplate
	templateStr := r.FormValue("template")
	if templateStr != "" {
		nameVersion := strings.Split(templateStr, "@")
		if len(nameVersion) != 2 {
			return nil, errors.New("invalid template name version")
		}
		template = &types.CreateRepoTemplate{
			FullName: nameVersion[0],
			Version:  &nameVersion[1],
		}
	}

	var readme *bool
	if r.Form.Has("readme") {
		readme = ptr(true)
	}
	var gitignore *string
	gitignoreStr := r.FormValue("gitignore")
	if gitignoreStr != "" {
		gitignore = &gitignoreStr
	}
	var dockerfile *string
	dockerfileStr := r.FormValue("dockerfile")
	if dockerfileStr != "" {
		dockerfile = &dockerfileStr
	}
	var dockerignore *bool
	if r.Form.Has("dockerignore") {
		dockerignore = ptr(true)
	}
	var license *types.CreateRepoInitializeLicense
	licenseStr := r.FormValue("license")
	if licenseStr != "" {
		var licenseFullName *string
		licenseFullNameStr := r.FormValue("license-fullname")
		if licenseFullNameStr != "" {
			licenseFullName = &licenseFullNameStr
		}
		var licenseProject *string
		licenseProjectStr := r.FormValue("license-project")
		if licenseProjectStr != "" {
			licenseProject = &licenseProjectStr
		}
		var year *int
		yearStr := r.FormValue("license-year")
		if yearStr != "" {
			yearInt, err := strconv.Atoi(r.FormValue("license-year"))
			if err != nil {
				return nil, errors.New("invalid license year")
			}
			year = &yearInt
		}
		license = &types.CreateRepoInitializeLicense{
			Key:      licenseStr,
			Fullname: licenseFullName,
			Project:  licenseProject,
			Year:     year,
		}
	}

	repo := &types.CreateRepo{
		Name:        name,
		Namespace:   namespace,
		Description: description,
		Visibility:  visibility,
		Sha256:      sha256,
		Initialize: &types.CreateRepoInitialize{
			Author: types.CreateRepoInitializeAuthor{
				Name:  account.DisplayName,
				Email: account.Email,
			},
			Template:     template,
			Tag:          tag,
			Readme:       readme,
			Gitignore:    gitignore,
			Dockerfile:   dockerfile,
			Dockerignore: dockerignore,
			License:      license,
		},
	}

	return repo, nil
}

func ptr[T any](v T) *T {
	return &v
}
