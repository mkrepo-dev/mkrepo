// Package types provides primitives to interact with the openapi HTTP API.
//
// Code generated by github.com/oapi-codegen/oapi-codegen/v2 version v2.4.1 DO NOT EDIT.
package types

// Defines values for CreateRepoVisibility.
const (
	Private CreateRepoVisibility = "private"
	Public  CreateRepoVisibility = "public"
)

// CreateRepo Create a new repository.
type CreateRepo struct {
	// Description A short description of the repository.
	Description *string `json:"description,omitempty"`

	// Initialize The initialization options for the repository.
	Initialize *CreateRepoInitialize `json:"initialize,omitempty"`

	// Name The name of the repository.
	Name string `json:"name"`

	// Namespace The namespace of the repository.
	Namespace string `json:"namespace"`

	// Sha256 Use SHA256 for the repository.
	Sha256 *bool `json:"sha256,omitempty"`

	// Visibility The visibility of the repository.
	Visibility *CreateRepoVisibility `json:"visibility,omitempty"`
}

// CreateRepoVisibility The visibility of the repository.
type CreateRepoVisibility string

// CreateRepoInitialize The initialization options for the repository.
type CreateRepoInitialize struct {
	// Author The author of initialize commit.
	Author CreateRepoInitializeAuthor `json:"author"`

	// Dockerfile Create a Dockerfile.
	Dockerfile *string `json:"dockerfile,omitempty"`

	// Dockerignore Create a .dockerignore file.
	Dockerignore *bool `json:"dockerignore,omitempty"`

	// Gitignore Create a .gitignore file.
	Gitignore *string `json:"gitignore,omitempty"`

	// License The license options for the repository.
	License *CreateRepoInitializeLicense `json:"license,omitempty"`

	// Readme Create a README file.
	Readme *bool `json:"readme,omitempty"`

	// Tag The tag to use for the repository.
	Tag *string `json:"tag,omitempty"`

	// Template The template to use for the repository.
	Template *CreateRepoTemplate `json:"template,omitempty"`
}

// CreateRepoInitializeAuthor The author of initialize commit.
type CreateRepoInitializeAuthor struct {
	// Email The email of the author.
	Email string `json:"email"`

	// Name The name of the author.
	Name string `json:"name"`
}

// CreateRepoInitializeLicense The license options for the repository.
type CreateRepoInitializeLicense struct {
	// Fullname The full name of the license holder.
	Fullname *string `json:"fullname,omitempty"`

	// Key The key of the license.
	Key string `json:"key"`

	// Project The name of the project.
	Project *string `json:"project,omitempty"`

	// Year The year of the license.
	Year *int `json:"year,omitempty"`
}

// CreateRepoTemplate The template to use for the repository.
type CreateRepoTemplate struct {
	// FullName The full name of the template.
	FullName string `json:"fullName"`

	// Values The values to use for the template.
	Values *map[string]interface{} `json:"values,omitempty"`

	// Version The version of the template.
	Version *string `json:"version,omitempty"`
}

// GetTemplateVersion Template with version.
type GetTemplateVersion struct {
	// BuildIn Whether the template is built-in.
	BuildIn bool `json:"buildIn"`

	// Description A short description of the template.
	Description *string `json:"description,omitempty"`

	// FullName The full name of the template.
	FullName string `json:"fullName"`

	// Language The language this template is for.
	Language *string `json:"language,omitempty"`

	// Name The name of the template.
	Name string `json:"name"`

	// Stars The number of stars for the template.
	Stars int `json:"stars"`

	// Url The URL of the template.
	Url *string `json:"url,omitempty"`

	// Version The version of the template.
	Version string `json:"version"`
}
