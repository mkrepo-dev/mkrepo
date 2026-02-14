package provider

import (
	"context"
	"strings"

	"github.com/google/go-github/v82/github"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/endpoints"

	"github.com/mkrepo-dev/mkrepo/internal/config"
)

type GitHub struct {
	config   config.Config
	provider config.Provider
}

var _ Provider = &GitHub{}

type GitHubClient struct {
	*github.Client
	gh    *GitHub
	token *oauth2.Token
}

var _ Client = &GitHubClient{}

func NewGitHubFromConfig(cfg config.Config, provider config.Provider) *GitHub {
	gh := &GitHub{
		config:   cfg,
		provider: provider,
	}
	if gh.provider.Name == "" {
		gh.provider.Name = "GitHub"
	}
	if gh.provider.Url == "" {
		gh.provider.Url = "https://github.com"
	}
	if gh.provider.ApiUrl == "" { // TODO: Use api url
		gh.provider.ApiUrl = "https://api.github.com"
	}
	return gh
}

func (gh *GitHub) Key() string {
	return gh.provider.Key
}

func (gh *GitHub) Name() string {
	return gh.provider.Name
}

func (gh *GitHub) Url() string {
	return gh.provider.Url
}

func (*GitHub) Features() ProviderFeatures {
	return ProviderFeatures{
		OAuth2AuthorizationCodeFlowWithPKCE: false,
		Sha256Repo:                          false,
	}
}

func (gh *GitHub) OAuth2Config() *oauth2.Config {
	return &oauth2.Config{
		ClientID:     gh.provider.ClientID,
		ClientSecret: gh.provider.ClientSecret,
		Scopes:       []string{"repo", "read:org"},
		Endpoint:     endpoints.GitHub,
	}
}

func (gh *GitHub) NewClient(ctx context.Context, token *oauth2.Token) Client {
	client := github.NewClient(nil).WithAuthToken(token.AccessToken)
	client.UserAgent = userAgent
	return &GitHubClient{Client: client, token: token, gh: gh}
}

func (client *GitHubClient) Token() *oauth2.Token {
	return client.token
}

func (client *GitHubClient) GetUser(ctx context.Context) (User, error) {
	var user User
	res, _, err := client.Users.Get(ctx, "")
	if err != nil {
		return user, err
	}
	user.Username = res.GetLogin()
	user.Email = res.GetEmail()
	user.DisplayName = res.GetName()
	user.AvatarURL = res.GetAvatarURL()
	return user, nil
}

func (client *GitHubClient) GetPosibleRepoOwners(ctx context.Context) ([]RepoOwner, error) {
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

func (client *GitHubClient) CreateRemoteRepo(ctx context.Context, repo CreateRepo) (RemoteRepo, error) {
	var private bool
	if repo.Visibility == RepoVisibilityPrivate {
		private = true
	}
	r, _, err := client.Repositories.Create(ctx, repo.Namespace, &github.Repository{
		Name:        &repo.Name,
		Description: repo.Description,
		Private:     &private,
		Visibility:  github.Ptr(string(repo.Visibility)),
	})
	if err != nil {
		if strings.Contains(err.Error(), "already exists") {
			return RemoteRepo{}, ErrRepoAlreadyExists
		}
		return RemoteRepo{}, err
	}
	return RemoteRepo{
		Id:        r.GetID(),
		Namespace: r.GetOwner().GetLogin(),
		Name:      r.GetName(),
		HtmlUrl:   r.GetHTMLURL(),
		CloneUrl:  r.GetCloneURL(),
	}, nil
}
