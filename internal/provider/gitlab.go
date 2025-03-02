package provider

import (
	"context"
	"log/slog"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/endpoints"

	"github.com/FilipSolich/mkrepo/internal"
	"github.com/FilipSolich/mkrepo/internal/config"
	"github.com/FilipSolich/mkrepo/internal/db"
	"github.com/FilipSolich/mkrepo/internal/log"
	gitlab "gitlab.com/gitlab-org/api/client-go"
)

type GitLab struct {
	Name         string
	ClientId     string
	ClientSecret string
	Url          string
}

var _ Provider = &GitLab{}

func NewGitLabFromConfig(cfg config.Provider) *GitLab {
	return &GitLab{
		Name:         cfg.Name,
		ClientId:     cfg.ClientID,
		ClientSecret: cfg.ClientSecret,
		Url:          cfg.Url,
	}
}

func (provider *GitLab) OAuth2Config() *oauth2.Config {
	return &oauth2.Config{
		ClientID:     provider.ClientId,
		ClientSecret: provider.ClientSecret,
		Scopes:       []string{"api"},
		Endpoint:     endpoints.GitLab,
		RedirectURL:  "http://localhost:8000/auth/oauth2/callback/gitlab", // TODO: Put this into config
	}
}

func (provider *GitLab) NewClient(ctx context.Context, token *oauth2.Token) ProviderClient {
	httpClient := provider.OAuth2Config().Client(ctx, token)
	client, err := gitlab.NewOAuthClient(token.AccessToken, gitlab.WithHTTPClient(httpClient))
	if err != nil {
		slog.Error("Failed to create gitlab client", log.Err(err))
	}
	client.UserAgent = internal.UserAgent
	return &GitLabClient{Client: client}
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

func (client *GitLabClient) CreateRemoteRepo(ctx context.Context, repo internal.Repo) (string, string, error) {
	// TODO: Handler groups
	r, _, err := client.Projects.CreateProject(&gitlab.CreateProjectOptions{
		Name:        &repo.Name,
		Description: &repo.Description,
		Visibility:  gitlab.Ptr(gitlab.VisibilityValue(repo.Visibility)),
	})
	if err != nil {
		return "", "", err
	}
	return r.WebURL, r.HTTPURLToRepo, nil
}

func (client *GitLabClient) CreateWebhook(ctx context.Context, repo internal.Repo) error {
	return nil
}

func (client *GitLabClient) GetPossibleRepoOwners(ctx context.Context) ([]string, error) {
	var owners []string
	user, _, err := client.Users.CurrentUser()
	if err != nil {
		return nil, err
	}
	owners = append(owners, user.Name)

	groups, _, err := client.Groups.ListGroups(&gitlab.ListGroupsOptions{})
	if err != nil {
		return owners, err
	}
	for _, group := range groups {
		// TODO: Can user create project in all of this groups?
		owners = append(owners, group.FullPath)
	}

	return owners, nil
}

func (client *GitLabClient) GetGitAuthor(ctx context.Context) (string, string, error) {
	user, _, err := client.Users.CurrentUser()
	if err != nil {
		return "", "", err
	}
	return user.Name, user.Email, nil
}
