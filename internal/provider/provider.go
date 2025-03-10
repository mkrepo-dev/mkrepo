package provider

import (
	"context"
	"log/slog"
	"net/url"

	"github.com/FilipSolich/mkrepo/internal"
	"github.com/FilipSolich/mkrepo/internal/config"
	"github.com/FilipSolich/mkrepo/internal/db"
	"github.com/FilipSolich/mkrepo/internal/log"
	"golang.org/x/oauth2"
)

type Provider interface {
	Name() string
	Url() string
	OAuth2Config(redirectUri string) *oauth2.Config
	NewClient(ctx context.Context, token *oauth2.Token, redirectUri string) (ProviderClient, *oauth2.Token)
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

func oauth2WithRedirectUri(config *oauth2.Config, redirectUri string) *oauth2.Config {
	if redirectUri == "" {
		return config
	}
	u, err := url.Parse(config.RedirectURL)
	if err != nil {
		slog.Error("Failed to parse redirect url", log.Err(err))
		return config
	}
	q := u.Query()
	q.Set("redirect_uri", redirectUri)
	u.RawQuery = q.Encode()
	config.RedirectURL = u.String()
	return config
}
