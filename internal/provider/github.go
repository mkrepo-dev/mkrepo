package provider

import (
	"context"

	"github.com/google/go-github/v69/github"
	"golang.org/x/oauth2"
	oauth2Github "golang.org/x/oauth2/github"

	"github.com/FilipSolich/mkrepo/internal"
	"github.com/FilipSolich/mkrepo/internal/config"
)

type GitHub struct {
	Name         string
	ClientId     string
	ClientSecret string
	Url          string
}

var _ Provider = &GitHub{}

func NewGitHubFromConfig(cfg config.Provider) *GitHub {
	return &GitHub{
		Name:         cfg.Name,
		ClientId:     cfg.ClientID,
		ClientSecret: cfg.ClientSecret,
		Url:          cfg.Url,
	}
}

func (provider *GitHub) OAuth2Config() *oauth2.Config {
	// TODO: Fill with custom url if there is any
	return &oauth2.Config{
		ClientID:     provider.ClientId,
		ClientSecret: provider.ClientSecret,
		Scopes:       []string{"repo", "read:org"},
		Endpoint:     oauth2Github.Endpoint,
	}
}

func (provider *GitHub) NewClient(token string) ProviderClient {
	client := github.NewClient(nil).WithAuthToken(token)
	client.UserAgent = internal.UserAgent
	return &GitHubClient{Client: client}
}

type GitHubClient struct {
	*github.Client
}

var _ ProviderClient = &GitHubClient{}

func NewGitHubClient(token string) *GitHubClient {
	client := github.NewClient(nil).WithAuthToken(token)
	client.UserAgent = internal.UserAgent
	return &GitHubClient{Client: client}
}

func (client *GitHubClient) CreateRemoteRepo(ctx context.Context, repo internal.Repo) (string, string, error) {
	var org string
	// TODO: Fix this. AuthorName is diffrent then user login. Find a way to diffrentiate between user and org.
	if repo.Owner != repo.AuthorName {
		org = repo.Owner
	}
	r, _, err := client.Repositories.Create(ctx, org, &github.Repository{
		Name:        &repo.Name,
		Description: &repo.Description,
		Visibility:  &repo.Visibility,
	})
	if err != nil {
		return "", "", err
	}
	return r.GetHTMLURL(), r.GetCloneURL(), nil
}

func (client *GitHubClient) CreateWebhook(ctx context.Context, repo internal.Repo) error {
	_, _, err := client.Repositories.CreateHook(ctx, repo.Owner, repo.Name, &github.Hook{ // TODO: Make sure repo name is correct here
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

func (client *GitHubClient) GetGitAuthor(ctx context.Context) (string, string, error) {
	user, _, err := client.Users.Get(ctx, "")
	if err != nil {
		return "", "", err
	}
	return user.GetName(), user.GetEmail(), nil
}

func (client *GitHubClient) GetPossibleRepoOwners(ctx context.Context) ([]string, error) {
	var owners []string
	user, _, err := client.Users.Get(ctx, "")
	if err != nil {
		return nil, err
	}
	owners = append(owners, user.GetLogin())

	orgs, _, err := client.Organizations.List(ctx, "", nil)
	if err != nil {
		return owners, err
	}
	for _, org := range orgs {
		org, _, err := client.Organizations.Get(ctx, org.GetLogin())
		if err != nil {
			return owners, err
		}
		if org.GetMembersCanCreateRepos() {
			owners = append(owners, org.GetLogin())
			continue
		}
		membership, _, err := client.Organizations.GetOrgMembership(ctx, "", org.GetLogin())
		if err != nil {
			return owners, err
		}
		if membership.GetRole() == "admin" {
			owners = append(owners, org.GetLogin())
		}
	}

	return owners, nil
}
