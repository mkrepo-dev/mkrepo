package provider

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/google/go-github/v70/github"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/endpoints"

	"github.com/mkrepo-dev/mkrepo/internal"
	"github.com/mkrepo-dev/mkrepo/internal/config"
	"github.com/mkrepo-dev/mkrepo/internal/db"
)

type GitHub struct {
	config        config.Provider
	baseUrl       string
	webhookSecret string
}

var _ Provider = &GitHub{}

func NewGitHubFromConfig(cfg config.Provider, baseUrl string, secret string) *GitHub {
	gh := &GitHub{
		config:        cfg,
		baseUrl:       baseUrl,
		webhookSecret: secret,
	}
	if gh.config.Name == "" {
		gh.config.Name = "GitHub"
	}
	if gh.config.Url == "" {
		gh.config.Url = "https://github.com"
	}
	if gh.config.ApiUrl == "" { // TODO: Use api url
		gh.config.ApiUrl = "https://api.github.com"
	}
	return gh
}

func (provider *GitHub) Name() string {
	return provider.config.Name
}

func (provider *GitHub) Url() string {
	return provider.config.Url
}

func (provider *GitHub) OAuth2Config(redirectUri string) *oauth2.Config {
	// TODO: Validate if redirect url is parsable but not here
	cfg := &oauth2.Config{
		ClientID:     provider.config.ClientID,
		ClientSecret: provider.config.ClientSecret,
		Scopes:       []string{"repo", "read:org"},
		RedirectURL:  buildAuthCallbackUrl(provider.baseUrl, provider.config.Key),
		Endpoint:     endpoints.GitHub,
	}
	return oauth2WithRedirectUri(cfg, redirectUri)
}

func (provider *GitHub) ParseWebhookEvent(r *http.Request) (WebhookEvent, error) {
	payload, err := github.ValidatePayload(r, []byte(provider.webhookSecret))
	if err != nil {
		return WebhookEvent{}, err
	}
	event, err := github.ParseWebHook(github.WebHookType(r), payload)
	if err != nil {
		return WebhookEvent{}, err

	}
	switch event := event.(type) {
	case *github.CreateEvent:
		if event.GetRefType() != "tag" {
			return WebhookEvent{}, ErrIgnoreEvent
		}
		fmt.Println(event.GetRef()) // TODO: Remove
		return WebhookEvent{
			Tag:      strings.TrimPrefix(strings.TrimPrefix(event.GetRef(), "refs/tags/"), "v"), // TODO: Is ref tag or does it contain refs/tags/?
			Url:      event.GetRepo().GetHTMLURL(),
			CloneUrl: event.GetRepo().GetCloneURL(),
		}, nil
	default:
		return WebhookEvent{}, ErrIgnoreEvent
	}
}

func (provider *GitHub) NewClient(ctx context.Context, token *oauth2.Token, _ string) (ProviderClient, *oauth2.Token) {
	client := github.NewClient(nil).WithAuthToken(token.AccessToken)
	client.UserAgent = internal.UserAgent
	return &GitHubClient{
		Client:        client,
		providerKey:   provider.config.Key,
		baseUrl:       provider.baseUrl,
		webhookSecret: provider.webhookSecret,
	}, token
}

type GitHubClient struct {
	*github.Client
	providerKey   string
	baseUrl       string
	webhookSecret string
}

var _ ProviderClient = &GitHubClient{}

func (client *GitHubClient) GetUserInfo(ctx context.Context) (db.UserInfo, error) {
	var info db.UserInfo
	user, _, err := client.Users.Get(ctx, "")
	if err != nil {
		return info, err
	}
	info.Username = user.GetLogin()
	info.Email = user.GetEmail()
	info.DisplayName = user.GetName()
	info.AvatarURL = user.GetAvatarURL()
	return info, nil
}

func (client *GitHubClient) CreateRemoteRepo(ctx context.Context, repo CreateRepo) (int64, string, string, string, error) {
	r, _, err := client.Repositories.Create(ctx, repo.Namespace, &github.Repository{
		Name:        &repo.Name,
		Description: &repo.Description,
		Visibility:  github.Ptr(string(repo.Visibility)),
	})
	if err != nil {
		if strings.Contains(err.Error(), "already exists") {
			return 0, "", "", "", ErrRepoAlreadyExists
		}
		return 0, "", "", "", err
	}
	return r.GetID(), r.GetOwner().GetLogin(), r.GetHTMLURL(), r.GetCloneURL(), nil
}

func (client *GitHubClient) CreateWebhook(ctx context.Context, webhook CreateWebhook) error {
	_, _, err := client.Repositories.CreateHook(ctx, webhook.Owner, webhook.Name, &github.Hook{ // TODO: Make sure repo name is correct here
		Active: github.Ptr(true),
		Events: []string{"create"},
		Config: &github.HookConfig{
			ContentType: github.Ptr("json"),
			InsecureSSL: github.Ptr("0"), // TODO: Make this configurable
			URL:         github.Ptr(buildWebhookUrl(client.baseUrl, client.providerKey)),
			Secret:      github.Ptr(client.webhookSecret),
		},
	})
	return err
}

func (client *GitHubClient) GetRepoOwners(ctx context.Context) ([]RepoOwner, error) {
	var owners []RepoOwner
	user, _, err := client.Users.Get(ctx, "")
	if err != nil {
		return nil, err
	}
	owners = append(owners, RepoOwner{
		Namespace:   "",
		Path:        user.GetLogin(),
		DisplayName: user.GetName(),
		AvatarUrl:   user.GetAvatarURL(),
	})

	orgs, _, err := client.Organizations.List(ctx, "", nil)
	if err != nil {
		return owners, err
	}
	for _, org := range orgs {
		org, _, err := client.Organizations.Get(ctx, org.GetLogin())
		if err != nil {
			return owners, err
		}

		orgOwner := RepoOwner{
			Namespace:   org.GetLogin(),
			Path:        org.GetLogin(),
			DisplayName: org.GetName(),
			AvatarUrl:   org.GetAvatarURL(),
		}
		if org.GetMembersCanCreateRepos() {
			owners = append(owners, orgOwner)
			continue
		}
		membership, _, err := client.Organizations.GetOrgMembership(ctx, "", org.GetLogin())
		if err != nil {
			return owners, err
		}
		if membership.GetRole() == "admin" {
			owners = append(owners, orgOwner)
		}
	}

	return owners, nil
}
