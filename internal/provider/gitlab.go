package provider

import (
	"context"
	"log/slog"
	"strconv"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/endpoints"

	"github.com/FilipSolich/mkrepo/internal"
	"github.com/FilipSolich/mkrepo/internal/config"
	"github.com/FilipSolich/mkrepo/internal/db"
	"github.com/FilipSolich/mkrepo/internal/log"
	gitlab "gitlab.com/gitlab-org/api/client-go"
)

type GitLab struct {
	name         string
	clientId     string
	clientSecret string
	url          string
	apiUrl       string
}

var _ Provider = &GitLab{}

func NewGitLabFromConfig(cfg config.Provider) *GitLab {
	name := "GitLab"
	if cfg.Name != "" {
		name = cfg.Name
	}
	url := "https://gitlab.com"
	if cfg.Url != "" {
		url = cfg.Url
	}
	apiUrl := "https://gitlab.com/api/v4"
	if cfg.ApiUrl != "" {
		apiUrl = cfg.ApiUrl
	}
	return &GitLab{
		name:         name,
		clientId:     cfg.ClientID,
		clientSecret: cfg.ClientSecret,
		url:          url,
		apiUrl:       apiUrl,
	}
}

func (provider *GitLab) Name() string {
	return provider.name
}

func (provider *GitLab) Url() string {
	return provider.url
}

func (provider *GitLab) OAuth2Config(redirectUri string) *oauth2.Config {
	cfg := &oauth2.Config{
		ClientID:     provider.clientId,
		ClientSecret: provider.clientSecret,
		Scopes:       []string{"api"},
		Endpoint:     endpoints.GitLab,
		RedirectURL:  "http://localhost:8000/auth/oauth2/callback/gitlab", // TODO: Put this into config
	}
	return oauth2WithRedirectUri(cfg, redirectUri)
}

func (provider *GitLab) NewClient(ctx context.Context, token *oauth2.Token, redirectUri string) (ProviderClient, *oauth2.Token) {
	ts := provider.OAuth2Config(redirectUri).TokenSource(ctx, token)
	tkn, err := ts.Token()
	if err != nil {
		slog.Error("Failed to get token", log.Err(err))
	}
	client, _ := gitlab.NewOAuthClient(tkn.AccessToken)
	client.UserAgent = internal.UserAgent
	return &GitLabClient{Client: client}, tkn
}

type GitLabClient struct {
	*gitlab.Client
}

var _ ProviderClient = &GitLabClient{}

func (client *GitLabClient) GetUserInfo(ctx context.Context) (db.UserInfo, error) {
	var info db.UserInfo
	user, _, err := client.Users.CurrentUser()
	if err != nil {
		return info, err
	}
	info.Username = user.Username
	info.Email = user.Email
	info.DisplayName = user.Name
	info.AvatarURL = user.AvatarURL
	return info, nil
}

func (client *GitLabClient) CreateRemoteRepo(ctx context.Context, repo CreateRepo) (string, string, error) {
	opt := &gitlab.CreateProjectOptions{
		Name:        &repo.Name,
		Description: &repo.Description,
		Visibility:  gitlab.Ptr(gitlab.VisibilityValue(repo.Visibility)),
	}
	if repo.Namespace != "" {
		namespace, err := strconv.Atoi(repo.Namespace)
		if err != nil {
			return "", "", err
		}
		opt.NamespaceID = &namespace
	}
	r, _, err := client.Projects.CreateProject(opt)
	if err != nil {
		return "", "", err
	}
	return r.WebURL, r.HTTPURLToRepo, nil
}

func (client *GitLabClient) CreateWebhook(ctx context.Context, repo CreateRepo) error {
	return nil
}

func (client *GitLabClient) GetRepoOwners(ctx context.Context) ([]RepoOwner, error) {
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
			Namespace:   strconv.Itoa(group.ID),
			Path:        group.FullPath,
			DisplayName: group.Name,
			AvatarUrl:   group.AvatarURL,
		})
	}

	return owners, nil
}
