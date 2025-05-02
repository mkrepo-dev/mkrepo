package provider

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"

	"golang.org/x/oauth2"

	"github.com/mkrepo-dev/mkrepo/internal/config"
	"github.com/mkrepo-dev/mkrepo/internal/log"
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

type User struct {
	Username    string
	Email       string
	DisplayName string
	AvatarUrl   string
}

type RepoOwner struct {
	Namespace   string
	Path        string
	DisplayName string
	AvatarUrl   string
}

type CreateRepo struct {
	Namespace   string
	Name        string
	Description *string
	Visibility  RepoVisibility
}

type RemoteRepo struct {
	Id        int64
	Namespace string
	Name      string
	HtmlUrl   string
	CloneUrl  string
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

	// Parse webhook from provider and return event. Only relevant event is tag creation.
	ParseWebhookEvent(r *http.Request) (WebhookEvent, error)

	// Create provider client based on oauth2 token. Refreshes token if needed. Created
	// client have same token during its lifetime and one client should be short lived
	// and request scoped. If token is refreshed during client creation it is up to caller
	// to update token in persistent storage. Token refreshed or not is accessible from
	// returned client using [ProviderClient.Token] method.
	NewClient(ctx context.Context, token *oauth2.Token, redirectUri string) Client
}

type Client interface {
	// Return oauth2 token
	Token() *oauth2.Token

	// Get user info
	GetUser(ctx context.Context) (User, error)
	// Get possible repo owners
	GetPosibleRepoOwners(ctx context.Context) ([]RepoOwner, error)

	// Create new repo and return user accessible url and http clone url
	CreateRemoteRepo(ctx context.Context, repo CreateRepo) (RemoteRepo, error)
	// Create webhook for the repo
	CreateWebhook(ctx context.Context, repo RemoteRepo) error
}

type Providers map[string]Provider

func NewProvidersFromConfig(cfg config.Config) Providers {
	providers := make(Providers)
	for _, providerConfig := range cfg.Providers {
		switch providerConfig.Type {
		case config.GitHubProvider:
			providers[providerConfig.Key] = NewGitHubFromConfig(cfg, providerConfig)
		case config.GitLabProvider:
			providers[providerConfig.Key] = NewGitLabFromConfig(cfg, providerConfig)
		default:
			slog.Warn("Unknown provider type", slog.String("type", string(providerConfig.Type)))
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
