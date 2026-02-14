package provider

import (
	"context"
	"log/slog"

	"code.gitea.io/sdk/gitea"
	"github.com/mkrepo-dev/mkrepo/internal/config"
	"github.com/mkrepo-dev/mkrepo/internal/log"
	"golang.org/x/oauth2"
)

type Gitea struct {
	config   config.Config
	provider config.Provider
}

var _ Provider = &Gitea{}

type GiteaClient struct {
	*gitea.Client
	gt    *Gitea
	token *oauth2.Token
}

var _ Client = &GiteaClient{}

func NewGiteaFromConfig(cfg config.Config, provider config.Provider) *Gitea {
	gt := &Gitea{
		config:   cfg,
		provider: provider,
	}
	if gt.provider.Name == "" {
		gt.provider.Name = "Gitea"
	}
	if gt.provider.Url == "" {
		gt.provider.Url = "https://gitea.com"
	}
	if gt.provider.ApiUrl == "" {
		gt.provider.ApiUrl = "https://gitea.com"
	}
	return gt
}

func (gt *Gitea) Key() string {
	return gt.provider.Key
}

func (gt *Gitea) Name() string {
	return gt.provider.Name
}

func (gt *Gitea) Url() string {
	return gt.provider.Url
}

func (*Gitea) Features() ProviderFeatures {
	return ProviderFeatures{
		OAuth2AuthorizationCodeFlowWithPKCE: true,
		Sha256Repo:                          true,
	}
}

func (gt *Gitea) OAuth2Config() *oauth2.Config {
	return &oauth2.Config{
		ClientID: gt.provider.ClientID,
		//ClientSecret: gt.provider.ClientSecret,
		Scopes: []string{"repository", "read:user", "read:organization"},
		Endpoint: oauth2.Endpoint{
			AuthURL:   gt.provider.Url + "/login/oauth/authorize",
			TokenURL:  gt.provider.Url + "/login/oauth/access_token",
			AuthStyle: oauth2.AuthStyleInParams,
		},
		RedirectURL: gt.config.BaseUrl + "/auth/oauth2/callback/gitea",
	}
}

func (gt *Gitea) NewClient(ctx context.Context, token *oauth2.Token) Client {
	ts := gt.OAuth2Config().TokenSource(ctx, token)
	tkn, err := ts.Token()
	if err != nil {
		slog.Error("Failed to get token", log.Err(err))
	}
	client, err := gitea.NewClient(gt.provider.ApiUrl,
		gitea.SetToken(tkn.AccessToken),
		gitea.SetUserAgent(userAgent),
	)
	if err != nil {
		slog.Error("Failed to create Gitea client.", log.Err(err))
	}
	return &GiteaClient{Client: client, token: tkn, gt: gt}
}

func (client *GiteaClient) Token() *oauth2.Token {
	return client.token
}

func (client *GiteaClient) GetUser(ctx context.Context) (User, error) {
	var user User
	res, _, err := client.GetMyUserInfo()
	if err != nil {
		return user, err
	}
	user.Username = res.UserName
	user.Email = res.Email
	user.DisplayName = res.FullName
	user.AvatarURL = res.AvatarURL
	return user, nil
}

func (client *GiteaClient) GetPosibleRepoOwners(ctx context.Context) ([]RepoOwner, error) {
	var owners []RepoOwner
	user, err := client.GetUser(ctx)
	if err != nil {
		return nil, err
	}
	owners = append(owners, RepoOwner{
		Namespace:   "",
		Path:        user.Username,
		DisplayName: user.DisplayName,
		AvatarUrl:   user.AvatarURL,
	})

	orgs, _, err := client.ListMyOrgs(gitea.ListOrgsOptions{})
	if err != nil {
		return owners, err
	}
	for _, org := range orgs {
		perm, _, err := client.GetOrgPermissions(org.UserName, user.Username)
		if err != nil {
			return owners, err
		}

		orgOwner := RepoOwner{
			Namespace:   org.UserName,
			Path:        org.UserName,
			DisplayName: org.FullName,
			AvatarUrl:   org.AvatarURL,
		}
		if perm.CanCreateRepository {
			owners = append(owners, orgOwner)
		}
	}

	return owners, nil
}

func (client *GiteaClient) CreateRemoteRepo(ctx context.Context, repo CreateRepo) (RemoteRepo, error) {
	var description string
	if repo.Description != nil {
		description = *repo.Description
	}
	var private bool
	if repo.Visibility == RepoVisibilityPrivate {
		private = true
	}
	objectFormat := "sha1"
	if repo.Sha256 != nil && *repo.Sha256 {
		objectFormat = "sha256"
	}

	var err error
	var r *gitea.Repository
	opt := gitea.CreateRepoOption{
		Name:             repo.Name,
		Description:      description,
		Private:          private,
		ObjectFormatName: objectFormat,
	}
	if repo.Namespace == "" {
		r, _, err = client.CreateRepo(opt)
	} else {
		r, _, err = client.CreateOrgRepo(repo.Namespace, opt)
	}
	if err != nil {
		return RemoteRepo{}, err
	}

	return RemoteRepo{
		Id:        r.ID,
		Namespace: r.Owner.UserName, // TODO: Is this correct?
		Name:      r.Name,
		HtmlUrl:   r.HTMLURL,
		CloneUrl:  r.CloneURL,
	}, nil
}
