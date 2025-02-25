package provider

import (
	"context"

	"github.com/FilipSolich/mkrepo/internal"
)

type ProviderClient interface {
	// Create new repo and return user accessible url and http clone url
	CreateRemoteRepo(ctx context.Context, repo internal.Repo) (string, string, error)

	// Get possible repo owners
	GetPossibleRepoOwners(ctx context.Context) ([]string, error)

	// Get git author name and email
	GetGitAuthor(ctx context.Context) (string, string, error)
}
