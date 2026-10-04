package provider

import (
	"context"
	"fmt"
	"log/slog"

	"golang.org/x/oauth2"

	"github.com/mkrepo-dev/mkrepo/internal"
	"github.com/mkrepo-dev/mkrepo/internal/config"
)

var userAgent = fmt.Sprintf("mkrepo/%s", internal.Build.Version)

type RepoVisibility string

const (
	RepoVisibilityPrivate RepoVisibility = "private"
	RepoVisibilityPublic  RepoVisibility = "public"
	// TODO: Add internal repos
)

type ProviderFeatures struct {
	OAuth2AuthorizationCodeFlowWithPKCE bool
	Sha256Repo                          bool
}

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

type Provider interface {
	Key() string
	Name() string
	OAuth2Config() *oauth2.Config
	Features() ProviderFeatures
	NewClient(token *oauth2.Token) (Client, error)
}

type Client interface {
	Token() *oauth2.Token
	GetUser(ctx context.Context) (User, error)
	GetPosibleRepoOwners(ctx context.Context) ([]RepoOwner, error)
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
