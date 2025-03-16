package provider

import (
	"context"

	"github.com/google/go-github/v69/github"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/endpoints"

	"github.com/FilipSolich/mkrepo/internal"
	"github.com/FilipSolich/mkrepo/internal/config"
	"github.com/FilipSolich/mkrepo/internal/db"
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
		Events: []string{"push"},
		Config: &github.HookConfig{
			ContentType: github.Ptr("json"),
			InsecureSSL: github.Ptr("0"),
			URL:         github.Ptr("https://example.com/webhook"), // TODO: Change this
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
