package mkrepo

import (
	"github.com/FilipSolich/mkrepo/internal/db"
	"github.com/FilipSolich/mkrepo/internal/provider"
	"github.com/FilipSolich/mkrepo/template"
)

type CreateRepo struct {
	Account db.Account
	// Remote repo information
	ProviderKey string
	Namespace   string
	Name        string
	Description string
	Visibility  provider.RepoVisibility

	// Initialization options
	Readme         bool
	Gitignore      string
	LicenseKey     string
	LicenseContext template.LicenseContext
	Dockerfile     string
	Dockerignore   bool

	// Extra git options
	Sha256 bool
	Tag    string

	// Rest
	IsTemplate bool
}

func (r *CreateRepo) NeedInitialization() bool {
	return r.Readme || r.Gitignore != "none" || r.Dockerfile != "none" || r.LicenseKey != "none" || r.IsTemplate
}
