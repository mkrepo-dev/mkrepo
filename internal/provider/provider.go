package provider

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"

	"github.com/mkrepo-dev/mkrepo/internal/config"
	"github.com/mkrepo-dev/mkrepo/internal/db"
	"github.com/mkrepo-dev/mkrepo/internal/log"
	"golang.org/x/oauth2"
)

var (
	ErrRepoAlreadyExists = errors.New("repository already exists") // TODO: Use this error in handler
	ErrIgnoreEvent       = errors.New("ignore event")
)

type RepoVisibility string

const (
	RepoVisibilityPrivate RepoVisibility = "private"
	RepoVisibilityPublic  RepoVisibility = "public"
	// TODO: Add internal repos
)

type CreateRepo struct {
	Namespace   string
	Name        string
	Description string
	Visibility  RepoVisibility
}

type CreateWebhook struct {
	ID    int64
	Owner string
	Name  string
}

type RepoOwner struct {
	Namespace   string
	Path        string
	DisplayName string
	AvatarUrl   string
}

type WebhookEvent struct {
	Tag      string
	Url      string
	CloneUrl string
}

type Provider interface {
	Name() string
	Url() string
	OAuth2Config(redirectUri string) *oauth2.Config
	ParseWebhookEvent(r *http.Request) (WebhookEvent, error)
	NewClient(ctx context.Context, token *oauth2.Token, redirectUri string) (ProviderClient, *oauth2.Token)
}

type ProviderClient interface {
	// Create new repo and return user accessible url and http clone url
	CreateRemoteRepo(ctx context.Context, repo CreateRepo) (int64, string, string, string, error)

	// Create webhook for the repo
	CreateWebhook(ctx context.Context, webhook CreateWebhook) error

	// Get possible repo owners
	GetRepoOwners(ctx context.Context) ([]RepoOwner, error)

	// Get user info
	GetUserInfo(ctx context.Context) (db.UserInfo, error)
}

type Providers map[string]Provider

func NewProvidersFromConfig(cfg config.Config) Providers {
	providers := make(Providers)
	for _, providerConfig := range cfg.Providers {
		switch providerConfig.Type {
		case config.GitHubProvider:
			providers[providerConfig.Key] = NewGitHubFromConfig(providerConfig, cfg.BaseUrl, cfg.Secret)
		case config.GitLabProvider:
			providers[providerConfig.Key] = NewGitLabFromConfig(providerConfig, cfg.BaseUrl, cfg.Secret)
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

func buildAuthCallbackUrl(baseUrl string, providerKey string) string {
	return fmt.Sprintf("%s/auth/oauth2/callback/%s", baseUrl, providerKey)
}

func buildWebhookUrl(baseUrl string, providerKey string) string {
	return fmt.Sprintf("%s/webhook/%s", baseUrl, providerKey)
}
