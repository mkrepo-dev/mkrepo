package service

// CreateRepoVisibility represents the visibility level of a repository
type CreateRepoVisibility string

const (
	Private  CreateRepoVisibility = "private"
	Public   CreateRepoVisibility = "public"
	Internal CreateRepoVisibility = "internal"
)

// CreateRepoInitializeAuthor contains information about the commit author
type CreateRepoInitializeAuthor struct {
	Name  string
	Email string
}

// CreateRepoTemplate contains template information for repository initialization
type CreateRepoTemplate struct {
	FullName string
}

// CreateRepoInitialize contains all initialization options for a new repository
type CreateRepoInitialize struct {
	Author    CreateRepoInitializeAuthor
	Template  *CreateRepoTemplate
	Tag       *string
	Readme    *bool
	Gitignore *string
	License   *LicenseKey
	Values    map[string]any
}

// CreateRepo represents a request to create a new repository
type CreateRepo struct {
	Name        string
	Namespace   string
	Description *string
	Visibility  *CreateRepoVisibility
	Sha256      *bool
	Initialize  *CreateRepoInitialize
}

// Template represents template information with version details
type Template struct {
	Name        string
	FullName    string
	BuildIn     bool
	Stars       int
	Version     string
	Url         *string
	Description *string
	Language    *string
	Schema      *map[string]any
}

// CreateRepoNeedsInitialization checks if the repository needs initialization
func CreateRepoNeedsInitialization(repo *CreateRepo) bool {
	if repo.Initialize == nil {
		return false
	}
	init := repo.Initialize
	return init.Template != nil ||
		(init.Readme != nil && *init.Readme) ||
		init.Gitignore != nil ||
		init.License != nil
}
