package provider

import (
	"context"

	"github.com/FilipSolich/mkrepo/internal"
	"github.com/FilipSolich/mkrepo/internal/config"
	"github.com/FilipSolich/mkrepo/internal/db"
	"golang.org/x/oauth2"
)

type Provider interface {
	Name() string
	Url() string
	OAuth2Config() *oauth2.Config
	NewClient(ctx context.Context, token *oauth2.Token) ProviderClient // TODO: use custom http client
}

type ProviderClient interface {
	// Create new repo and return user accessible url and http clone url
	CreateRemoteRepo(ctx context.Context, repo internal.Repo) (string, string, error)

	// Create webhook for the repo
	CreateWebhook(ctx context.Context, repo internal.Repo) error

	// Get possible repo owners
	GetRepoOwners(ctx context.Context) ([]RepoOwner, error)

	// Get user info
	GetUserInfo(ctx context.Context) (db.UserInfo, error)
}

type RepoOwner struct {
	Name        string
	DisplayName string
	AvatarUrl   string
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
