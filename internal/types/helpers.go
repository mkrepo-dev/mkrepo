package types

import (
	"errors"
	"net/http"
	"slices"
	"strconv"
)

func ptr[T any](v T) *T {
	return &v
}

func CreateRepoFromForm(r *http.Request) (*CreateRepo, error) {
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
	if namespace == "" {
		return nil, errors.New("owner is required")
	}
	var description *string
	descriptionStr := r.FormValue("description")
	if descriptionStr != "" {
		description = &descriptionStr
	}
	var visibility *CreateRepoVisibility
	var formVisibility CreateRepoVisibility = CreateRepoVisibility(r.FormValue("visibility"))
	if slices.Contains([]CreateRepoVisibility{Private, Public}, formVisibility) {
		visibility = &formVisibility
	} else {
		return nil, errors.New("invalid visibility")
	}

	var sha256 *bool
	if r.Form.Has("sha256") {
		sha256 = ptr(true)
	}

	// TODO: Handle nil and move from db find better place
	//providerUsername := strings.Split(r.FormValue("provider"), ":")
	//provider, username := providerUsername[0], providerUsername[1]
	//account := database.GetAccount(middleware.Accounts(r.Context()), provider, username)
	var tag *string
	if r.Form.Has("tag") {
		tag = ptr("v0.0.0")
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
	var license *CreateRepoInitializeLicense
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
		license = &CreateRepoInitializeLicense{
			Key:      licenseStr,
			Fullname: licenseFullName,
			Project:  licenseProject,
			Year:     year,
		}
	}

	repo := &CreateRepo{
		Name:        name,
		Namespace:   namespace,
		Description: description,
		Visibility:  visibility,
		Sha256:      sha256,
		Initialize: &CreateRepoInitialize{
			Author: CreateRepoInitializeAuthor{
				//Name:  account.DisplayName,
				//Email: account.Email,
			},
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

// TODO: Validate

// TODO: Defaulting

// Returns true if created repo is created from template
func CreateRepoUsesTemplate(repo *CreateRepo) bool {
	return repo.Template != nil
}

// Returns true if created repo need initialization
func CreateRepoNeedsInitialization(repo *CreateRepo) bool {
	return repo.Initialize != nil &&
		(repo.Initialize.Readme != nil ||
			repo.Initialize.Gitignore != nil ||
			repo.Initialize.Dockerfile != nil ||
			repo.Initialize.License != nil)
}

// Return true if created repo is template for mkrepo and uses buildin template for templates
func CreateRepoIsTemplate(repo *CreateRepo) bool {
	return repo.Template != nil && repo.Template.Name == "template"
}
