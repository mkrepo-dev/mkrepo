package provider

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"golang.org/x/oauth2"

	"github.com/mkrepo-dev/mkrepo/internal"
	"github.com/mkrepo-dev/mkrepo/internal/config"
)

var (
	ErrRepoAlreadyExists = errors.New("repository already exists") // TODO: Use this error in handler
)

var userAgent = fmt.Sprintf("mkrepo/%s", internal.Build.Version)

type RepoVisibility string

const (
	RepoVisibilityPrivate RepoVisibility = "private"
	RepoVisibilityPublic  RepoVisibility = "public"
	// TODO: Add internal repos
)

type User struct {
	ID          string
	Username    string
	Email       string
	DisplayName string
	AvatarURL   string
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
	Sha256      *bool
}

type RemoteRepo struct {
	Id        int64
	Namespace string
	Name      string
	HtmlUrl   string
	CloneUrl  string
}

type ProviderFeatures struct {
	OAuth2AuthorizationCodeFlowWithPKCE bool
	Sha256Repo                          bool
}

type Provider interface {
	Key() string
	Name() string
	Url() string
	OAuth2Config() *oauth2.Config
	Features() ProviderFeatures

	// Create provider client based on oauth2 token. Refreshes token if needed. Created
	// client have same token during its lifetime and one client should be short lived
	// and request scoped. If token is refreshed during client creation it is up to caller
	// to update token in persistent storage. Token refreshed or not is accessible from
	// returned client using [ProviderClient.Token] method.
	// TODO: Return error
	NewClient(ctx context.Context, token *oauth2.Token) Client
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
}

type ProviderKey string

type Providers map[ProviderKey]Provider

func NewProvidersFromConfig(ctx context.Context, logger *slog.Logger, cfg config.Config) Providers {
	providers := make(Providers)
	for _, providerConfig := range cfg.Providers {
		switch providerConfig.Type {
		case config.GitHubProvider:
			providers[ProviderKey(providerConfig.Key)] = NewGitHubFromConfig(cfg, providerConfig)
		case config.GitLabProvider:
			providers[ProviderKey(providerConfig.Key)] = NewGitLabFromConfig(cfg, providerConfig)
		case config.GiteaProvider:
			providers[ProviderKey(providerConfig.Key)] = NewGiteaFromConfig(cfg, providerConfig)
		default:
			logger.WarnContext(ctx, "Unknown provider type", slog.String("type", string(providerConfig.Type)))
		}
	}
	return providers
}
