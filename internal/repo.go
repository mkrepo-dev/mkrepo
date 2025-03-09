package internal

import (
	"github.com/FilipSolich/mkrepo/internal/db"
)

type Repo struct {
	Account     db.Account
	Owner       string
	Name        string
	Description string
	Visibility  string

	Readme       bool
	Gitignore    string
	License      string
	Dockerfile   string
	Dockerignore bool

	Tag string

	IsTemplate bool
	Sha256     bool
}

func (r *Repo) NeedInitialization() bool {
	return r.Readme || r.Gitignore != "" || r.License != "" || r.Dockerfile != "" || r.Dockerignore || r.IsTemplate
}
