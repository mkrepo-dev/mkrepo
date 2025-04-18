package provider

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	gitlab "gitlab.com/gitlab-org/api/client-go"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/endpoints"

	"github.com/mkrepo-dev/mkrepo/internal"
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
	gl *GitLab
}

var _ ProviderClient = &GitLabClient{}

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

func (gl *GitLab) Name() string {
	return gl.provider.Name
}

func (gl *GitLab) Url() string {
	return gl.provider.Url
}

func (gl *GitLab) OAuth2Config(redirectUri string) *oauth2.Config {
	cfg := &oauth2.Config{
		ClientID:     gl.provider.ClientID,
		ClientSecret: gl.provider.ClientSecret,
		Scopes:       []string{"api"},
		Endpoint:     endpoints.GitLab,
		RedirectURL:  buildAuthCallbackUrl(gl.config.BaseUrl, gl.provider.Key),
	}
	return oauth2WithRedirectUri(cfg, redirectUri)
}

func (gl *GitLab) ParseWebhookEvent(r *http.Request) (WebhookEvent, error) {
	token := r.Header.Get("X-Gitlab-Token")
	if token != gl.config.Secret {
		return WebhookEvent{}, errors.New("invalid request")
	}
	payload, err := io.ReadAll(r.Body)
	if err != nil {
		return WebhookEvent{}, err
	}
	event, err := gitlab.ParseWebhook(gitlab.HookEventType(r), payload)
	if err != nil {
		return WebhookEvent{}, err
	}
	switch event := event.(type) {
	case *gitlab.TagEvent:
		return WebhookEvent{
			Tag:      strings.TrimPrefix(strings.TrimPrefix(event.Ref, "refs/tags/"), "v"),
			Url:      event.Repository.WebURL,
			CloneUrl: event.Repository.HTTPURL,
		}, nil
	default:
		return WebhookEvent{}, ErrIgnoreEvent
	}
}

func (gl *GitLab) NewClient(ctx context.Context, token *oauth2.Token, redirectUri string) (ProviderClient, *oauth2.Token) {
	ts := gl.OAuth2Config(redirectUri).TokenSource(ctx, token)
	tkn, err := ts.Token()
	if err != nil {
		slog.Error("Failed to get token", log.Err(err))
	}
	client, _ := gitlab.NewOAuthClient(tkn.AccessToken)
	client.UserAgent = internal.UserAgent
	return &GitLabClient{Client: client, gl: gl}, tkn
}

func (gl *GitLab) webhookConfig() *gitlab.AddProjectHookOptions {
	return &gitlab.AddProjectHookOptions{
		Name:                  gitlab.Ptr("mkrepo"),
		Description:           gitlab.Ptr("mkrepo webhook"),
		URL:                   gitlab.Ptr(buildWebhookUrl(gl.config.BaseUrl, gl.provider.Key)),
		TagPushEvents:         gitlab.Ptr(true),
		Token:                 gitlab.Ptr(gl.config.Secret),
		EnableSSLVerification: gitlab.Ptr(!gl.config.WebhookInsecure),
	}
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
	user.AvatarUrl = res.AvatarURL
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
			Namespace:   strconv.Itoa(group.ID),
			Path:        group.FullPath,
			DisplayName: group.Name,
			AvatarUrl:   group.AvatarURL,
		})
	}

	return owners, nil
}

func (client *GitLabClient) CreateRemoteRepo(ctx context.Context, repo CreateRepo) (RemoteRepo, error) {
	opt := &gitlab.CreateProjectOptions{
		Name:        &repo.Name,
		Description: &repo.Description,
		Visibility:  gitlab.Ptr(gitlab.VisibilityValue(repo.Visibility)),
	}
	if repo.Namespace != "" {
		namespace, err := strconv.Atoi(repo.Namespace)
		if err != nil {
			return RemoteRepo{}, err
		}
		opt.NamespaceID = &namespace
	}
	r, _, err := client.Projects.CreateProject(opt)
	if err != nil {
		// TODO: Report if repo already exists
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

func (client *GitLabClient) CreateWebhook(ctx context.Context, repo RemoteRepo) error {
	_, _, err := client.Projects.AddProjectHook(repo.Id, client.gl.webhookConfig())
	return err
}
