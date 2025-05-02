package types

// TODO: Validate

// TODO: Defaulting

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
	return repo.Initialize.Template != nil && repo.Initialize.Template.FullName == "template"
}
