package internal

type Repo struct {
	Provider    string
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

	AuthorName  string
	AuthorEmail string
	AuthToken   string
}

func (r *Repo) NeedInitialization() bool {
	return r.Readme || r.Gitignore != "" || r.License != "" || r.Dockerfile != "" || r.Dockerignore
}
