package provider

import (
	"context"
	"net/http"
	"strconv"
	"time"

	"github.com/google/go-github/v88/github"
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

func (gh *GitHub) NewClient(token *oauth2.Token) (Client, error) {
	opts := []github.ClientOptionsFunc{
		github.WithAuthToken(token.AccessToken),
		github.WithUserAgent(userAgent),
		github.WithHTTPClient(&http.Client{Timeout: 30 * time.Second}),
	}
	if gh.provider.Url != "" {
		opts = append(opts, github.WithURLs(&gh.provider.Url, nil))
	}
	client, err := github.NewClient(opts...)
	if err != nil {
		return nil, err
	}
	return &GitHubClient{Client: client, token: token}, nil
}

func (client *GitHubClient) Token() *oauth2.Token {
	return client.token
}

func (client *GitHubClient) GetUser(ctx context.Context) (User, error) {
	res, _, err := client.Users.Get(ctx, "")
	if err != nil {
		return User{}, err
	}
	return User{
		ID:          strconv.FormatInt(res.GetID(), 10),
		Username:    res.GetLogin(),
		Email:       res.GetEmail(),
		DisplayName: res.GetName(),
		AvatarURL:   res.GetAvatarURL(),
	}, nil
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
	r, _, err := client.Repositories.Create(ctx, repo.Namespace, &github.Repository{
		Name:        &repo.Name,
		Description: repo.Description,
		Private:     new(repo.Visibility == RepoVisibilityPrivate),
		Visibility:  new(string(repo.Visibility)),
	})
	if err != nil {
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
