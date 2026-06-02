package provider

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"time"

	gitlab "gitlab.com/gitlab-org/api/client-go/v2"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/endpoints"

	"github.com/mkrepo-dev/mkrepo/internal/config"
)

type GitLab struct {
	config   config.Config
	provider config.Provider
}

var _ Provider = &GitLab{}

type GitLabClient struct {
	*gitlab.Client
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
	return gl
}

func (gl *GitLab) Key() string {
	return gl.provider.Key
}

func (gl *GitLab) Name() string {
	return gl.provider.Name
}

func (*GitLab) Features() ProviderFeatures {
	return ProviderFeatures{
		OAuth2AuthorizationCodeFlowWithPKCE: true,
		Sha256Repo:                          false,
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

func (gl *GitLab) NewClient(token *oauth2.Token) (Client, error) {
	opts := []gitlab.ClientOptionFunc{
		gitlab.WithUserAgent(userAgent),
		gitlab.WithHTTPClient(&http.Client{Timeout: 30 * time.Second}),
	}
	if gl.provider.Url != "" {
		opts = append(opts, gitlab.WithBaseURL(gl.provider.Url))
	}
	client, err := gitlab.NewClient(token.AccessToken, opts...)
	if err != nil {
		return nil, err
	}
	return &GitLabClient{Client: client, token: token}, nil
}

func (client *GitLabClient) Token() *oauth2.Token {
	return client.token
}

func (client *GitLabClient) GetUser(ctx context.Context) (User, error) {
	res, _, err := client.Users.CurrentUser()
	if err != nil {
		return User{}, err
	}
	return User{
		ID:          strconv.FormatInt(res.ID, 10),
		Username:    res.Username,
		Email:       res.Email,
		DisplayName: res.Name,
		AvatarURL:   res.AvatarURL,
	}, nil
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
		MinAccessLevel: new(gitlab.DeveloperPermissions),
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
		Visibility:  new(gitlab.VisibilityValue(repo.Visibility)),
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
