package provider

import (
	"context"
	"errors"
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
	name         string
	clientId     string
	clientSecret string
	url          string
	apiUrl       string // TODO: Use api url
}

var _ Provider = &GitHub{}

func NewGitHubFromConfig(cfg config.Provider) *GitHub {
	name := "GitHub"
	if cfg.Name != "" {
		name = cfg.Name
	}
	url := "https://github.com"
	if cfg.Url != "" {
		url = cfg.Url
	}
	apiUrl := "https://api.github.com"
	if cfg.ApiUrl != "" {
		apiUrl = cfg.ApiUrl
	}
	return &GitHub{
		name:         name,
		clientId:     cfg.ClientID,
		clientSecret: cfg.ClientSecret,
		url:          url,
		apiUrl:       apiUrl,
	}
}

func (provider *GitHub) Name() string {
	return provider.name
}

func (provider *GitHub) Url() string {
	return provider.url
}

func (provider *GitHub) OAuth2Config(redirectUri string) *oauth2.Config {
	// TODO: Validate if redirect url is parsable but not here
	cfg := &oauth2.Config{
		ClientID:     provider.clientId,
		ClientSecret: provider.clientSecret,
		Scopes:       []string{"repo", "read:org"},
		RedirectURL:  "http://localhost:8000/auth/oauth2/callback/github", // TODO: Fill this from config. Must match what is set in GitHub.
		Endpoint:     endpoints.GitHub,
	}
	return oauth2WithRedirectUri(cfg, redirectUri)
}

func (provider *GitHub) ParseWebhookEvent(r *http.Request) (WebhookEvent, error) {
	payload, err := github.ValidatePayload(r, []byte("")) // TODO: Fill this with secret
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
			return WebhookEvent{}, errors.New("unsupported event type")
		}
		fmt.Println(event.GetRef())
		return WebhookEvent{
			Tag:      strings.TrimPrefix(strings.TrimPrefix(event.GetRef(), "refs/tags/"), "v"), // TODO: Is ref tag or does it contain refs/tags/?
			Url:      event.GetRepo().GetHTMLURL(),
			CloneUrl: event.GetRepo().GetCloneURL(),
		}, nil
	default:
		return WebhookEvent{}, errors.New("unsupported event type")
	}
}

func (provider *GitHub) NewClient(ctx context.Context, token *oauth2.Token, _ string) (ProviderClient, *oauth2.Token) {
	client := github.NewClient(nil).WithAuthToken(token.AccessToken)
	client.UserAgent = internal.UserAgent
	return &GitHubClient{Client: client}, token
}

type GitHubClient struct {
	*github.Client
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

func (client *GitHubClient) CreateRemoteRepo(ctx context.Context, repo CreateRepo) (string, string, error) {
	r, _, err := client.Repositories.Create(ctx, repo.Namespace, &github.Repository{
		Name:        &repo.Name,
		Description: &repo.Description,
		Visibility:  github.Ptr(string(repo.Visibility)),
	})
	if err != nil {
		return "", "", err
	}
	return r.GetHTMLURL(), r.GetCloneURL(), nil
}

func (client *GitHubClient) CreateWebhook(ctx context.Context, repo CreateRepo) error {
	_, _, err := client.Repositories.CreateHook(ctx, repo.Namespace, repo.Name, &github.Hook{ // TODO: Make sure repo name is correct here
		Active: github.Ptr(true),
		Events: []string{"create"},
		Config: &github.HookConfig{
			ContentType: github.Ptr("json"),
			InsecureSSL: github.Ptr("0"),
			URL:         github.Ptr("https://example.com/webhook"), // TODO: Change this
			Secret:      github.Ptr(""),                            // TODO: Change this
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
