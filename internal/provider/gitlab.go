package provider

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"

	gitlab "gitlab.com/gitlab-org/api/client-go"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/endpoints"

	"github.com/mkrepo-dev/mkrepo/internal/config"
	"github.com/mkrepo-dev/mkrepo/internal/log"
)

type GitLab struct {
	config   config.Config
	provider config.Provider
}

var _ Provider = &GitLab{}

type GitLabClient struct {
	*gitlab.Client
	gl    *GitLab
	token *oauth2.Token
}

var _ Client = &GitLabClient{}

func NewGitLabFromConfig(cfg config.Config, provider config.Provider) *GitLab {
	gl := &GitLab{
		config:   cfg,
		provider: provider,
	}

	if gl.provider.Name == "" {
		gl.provider.Name = "GitLab"
	}
	if gl.provider.Url == "" {
		gl.provider.Url = "https://gitlab.com"
	}
	if gl.provider.ApiUrl == "" {
		gl.provider.ApiUrl = "https://gitlab.com/api/v4"
	}
	return gl
}

func (gl *GitLab) Key() string {
	return gl.provider.Key
}

func (gl *GitLab) Name() string {
	return gl.provider.Name
}

func (gl *GitLab) Url() string {
	return gl.provider.Url
}

func (*GitLab) Features() ProviderFeatures {
	return ProviderFeatures{
		OAuth2AuthorizationCodeFlowWithPKCE: true,
		Sha256Repo:                          true,
	}
}

func (gl *GitLab) OAuth2Config() *oauth2.Config {
	return &oauth2.Config{
		ClientID:     gl.provider.ClientID,
		ClientSecret: gl.provider.ClientSecret,
		Scopes:       []string{"api"},
		Endpoint:     endpoints.GitLab,
		RedirectURL:  fmt.Sprintf("%s/auth/oauth2/callback/gitlab", gl.config.BaseUrl),
	}
}

func (gl *GitLab) NewClient(ctx context.Context, token *oauth2.Token) Client {
	ts := gl.OAuth2Config().TokenSource(ctx, token)
	tkn, err := ts.Token()
	if err != nil {
		slog.Error("Failed to get token", log.Err(err))
	}
	client, _ := gitlab.NewClient(tkn.AccessToken)
	client.UserAgent = userAgent
	return &GitLabClient{Client: client, token: tkn, gl: gl}
}

func (client *GitLabClient) Token() *oauth2.Token {
	return client.token
}

func (client *GitLabClient) GetUser(ctx context.Context) (User, error) {
	var user User
	res, _, err := client.Users.CurrentUser()
	if err != nil {
		return user, err
	}
	user.Username = res.Username
	user.Email = res.Email
	user.DisplayName = res.Name
	user.AvatarURL = res.AvatarURL
	return user, nil
}

func (client *GitLabClient) GetPosibleRepoOwners(ctx context.Context) ([]RepoOwner, error) {
	var owners []RepoOwner
	user, _, err := client.Users.CurrentUser()
	if err != nil {
		return nil, err
	}
	owners = append(owners, RepoOwner{
		Namespace:   "",
		Path:        user.Username,
		DisplayName: user.Name,
		AvatarUrl:   user.AvatarURL,
	})

	groups, _, err := client.Groups.ListGroups(&gitlab.ListGroupsOptions{
		MinAccessLevel: gitlab.Ptr(gitlab.DeveloperPermissions),
	})
	if err != nil {
		return owners, err
	}
	for _, group := range groups {
		owners = append(owners, RepoOwner{
			Namespace:   strconv.FormatInt(group.ID, 10),
			Path:        group.FullPath,
			DisplayName: group.Name,
			AvatarUrl:   group.AvatarURL,
		})
	}

	return owners, nil
}

func (client *GitLabClient) CreateRemoteRepo(ctx context.Context, repo CreateRepo) (RemoteRepo, error) {
	// TODO: Add support for sha256 once gitlab client supports it
	opt := &gitlab.CreateProjectOptions{
		Name:        &repo.Name,
		Description: repo.Description,
		Visibility:  gitlab.Ptr(gitlab.VisibilityValue(repo.Visibility)),
	}
	if repo.Namespace != "" {
		namespace, err := strconv.ParseInt(repo.Namespace, 10, 64)
		if err != nil {
			return RemoteRepo{}, err
		}
		opt.NamespaceID = &namespace
	}
	r, _, err := client.Projects.CreateProject(opt)
	if err != nil {
		return RemoteRepo{}, err
	}
	return RemoteRepo{
		Id:        int64(r.ID),
		Namespace: r.Namespace.Path, // TODO: Use this? it is probably not used
		Name:      r.Name,
		HtmlUrl:   r.WebURL,
		CloneUrl:  r.HTTPURLToRepo,
	}, nil
}
