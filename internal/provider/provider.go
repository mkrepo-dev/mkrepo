package provider

import (
	"context"

	"github.com/FilipSolich/mkrepo/internal"
	"github.com/FilipSolich/mkrepo/internal/config"
	"golang.org/x/oauth2"
)

type Provider interface {
	OAuth2Config() *oauth2.Config
	NewClient(token string) ProviderClient
}

type ProviderClient interface {
	// Create new repo and return user accessible url and http clone url
	CreateRemoteRepo(ctx context.Context, repo internal.Repo) (string, string, error)

	// Create webhook for the repo
	CreateWebhook(ctx context.Context, repo internal.Repo) error

	// Get possible repo owners
	GetPossibleRepoOwners(ctx context.Context) ([]string, error)

	// Get git author name and email
	GetGitAuthor(ctx context.Context) (string, string, error)
}

type Providers map[string]Provider

func NewProvidersFromConfig(cfg []config.Provider) Providers {
	providers := make(Providers)
	for _, providerConfig := range cfg {
		switch providerConfig.Type {
		case config.GitHubProvider:
			providers[providerConfig.Key] = NewGitHubFromConfig(providerConfig)
		case config.GitLabProvider:
			providers[providerConfig.Key] = NewGitLabFromConfig(providerConfig)
		}
	}
	return providers
}
