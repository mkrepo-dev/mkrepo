package provider

import (
	"context"
	"net/http"
	"strconv"
	"time"

	"code.gitea.io/sdk/gitea"
	"golang.org/x/oauth2"

	"github.com/mkrepo-dev/mkrepo/internal/config"
)

type Gitea struct {
	config   config.Config
	provider config.Provider
}

var _ Provider = &Gitea{}

type GiteaClient struct {
	*gitea.Client
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
	return gt
}

func (gt *Gitea) Key() string {
	return gt.provider.Key
}

func (gt *Gitea) Name() string {
	return gt.provider.Name
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

func (gt *Gitea) NewClient(token *oauth2.Token) (Client, error) {
	url := "https://gitea.com"
	if gt.provider.Url != "" {
		url = gt.provider.Url
	}
	client, err := gitea.NewClient(url,
		gitea.SetToken(token.AccessToken),
		gitea.SetUserAgent(userAgent),
		gitea.SetHTTPClient(&http.Client{Timeout: 30 * time.Second}),
	)
	if err != nil {
		return nil, err
	}
	return &GiteaClient{Client: client, token: token}, nil
}

func (client *GiteaClient) Token() *oauth2.Token {
	return client.token
}

func (client *GiteaClient) GetUser(ctx context.Context) (User, error) {
	res, _, err := client.GetMyUserInfo()
	if err != nil {
		return User{}, err
	}
	return User{
		ID:          strconv.FormatInt(res.ID, 10),
		Username:    res.UserName,
		Email:       res.Email,
		DisplayName: res.FullName,
		AvatarURL:   res.AvatarURL,
	}, nil
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
		perm, _, err := client.GetOrgPermissions(org.Name, user.Username)
		if err != nil {
			return owners, err
		}
		if perm.CanCreateRepository {
			owners = append(owners, RepoOwner{
				Namespace:   org.Name,
				Path:        org.Name,
				DisplayName: org.FullName,
				AvatarUrl:   org.AvatarURL,
			})
		}
	}

	return owners, nil
}

func (client *GiteaClient) CreateRemoteRepo(ctx context.Context, repo CreateRepo) (RemoteRepo, error) {
	var description string
	if repo.Description != nil {
		description = *repo.Description
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
		Private:          repo.Visibility == RepoVisibilityPrivate,
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
