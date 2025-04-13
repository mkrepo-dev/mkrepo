package provider

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/endpoints"

	"github.com/mkrepo-dev/mkrepo/internal"
	"github.com/mkrepo-dev/mkrepo/internal/config"
	"github.com/mkrepo-dev/mkrepo/internal/db"
	"github.com/mkrepo-dev/mkrepo/internal/log"
	gitlab "gitlab.com/gitlab-org/api/client-go"
)

type GitLab struct {
	config        config.Provider
	baseUrl       string
	webhookSecret string
}

var _ Provider = &GitLab{}

func NewGitLabFromConfig(cfg config.Provider, baseUrl string, secret string) *GitLab {
	gl := &GitLab{
		config:        cfg,
		baseUrl:       baseUrl,
		webhookSecret: secret,
	}

	if gl.config.Name == "" {
		gl.config.Name = "GitLab"
	}
	if gl.config.Url == "" {
		gl.config.Url = "https://gitlab.com"
	}
	if gl.config.ApiUrl == "" {
		gl.config.ApiUrl = "https://gitlab.com/api/v4"
	}
	return gl
}

func (provider *GitLab) Name() string {
	return provider.config.Name
}

func (provider *GitLab) Url() string {
	return provider.config.Url
}

func (provider *GitLab) OAuth2Config(redirectUri string) *oauth2.Config {
	cfg := &oauth2.Config{
		ClientID:     provider.config.ClientID,
		ClientSecret: provider.config.ClientSecret,
		Scopes:       []string{"api"},
		Endpoint:     endpoints.GitLab,
		RedirectURL:  buildAuthCallbackUrl(provider.baseUrl, provider.config.Key),
	}
	return oauth2WithRedirectUri(cfg, redirectUri)
}

func (provider *GitLab) ParseWebhookEvent(r *http.Request) (WebhookEvent, error) {
	token := r.Header.Get("X-Gitlab-Token")
	if token != provider.webhookSecret {
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

func (provider *GitLab) NewClient(ctx context.Context, token *oauth2.Token, redirectUri string) (ProviderClient, *oauth2.Token) {
	ts := provider.OAuth2Config(redirectUri).TokenSource(ctx, token)
	tkn, err := ts.Token()
	if err != nil {
		slog.Error("Failed to get token", log.Err(err))
	}
	client, _ := gitlab.NewOAuthClient(tkn.AccessToken)
	client.UserAgent = internal.UserAgent
	return &GitLabClient{
		Client:        client,
		providerKey:   provider.config.Key,
		baseUrl:       provider.baseUrl,
		webhookSecret: provider.webhookSecret,
	}, tkn
}

type GitLabClient struct {
	*gitlab.Client
	providerKey   string
	baseUrl       string
	webhookSecret string
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

func (client *GitLabClient) CreateRemoteRepo(ctx context.Context, repo CreateRepo) (int64, string, string, string, error) {
	opt := &gitlab.CreateProjectOptions{
		Name:        &repo.Name,
		Description: &repo.Description,
		Visibility:  gitlab.Ptr(gitlab.VisibilityValue(repo.Visibility)),
	}
	if repo.Namespace != "" {
		namespace, err := strconv.Atoi(repo.Namespace)
		if err != nil {
			return 0, "", "", "", err
		}
		opt.NamespaceID = &namespace
	}
	r, _, err := client.Projects.CreateProject(opt)
	if err != nil {
		// TODO: Report if repo already exists
		return 0, "", "", "", err
	}
	return int64(r.ID), r.Owner.Username, r.WebURL, r.HTTPURLToRepo, nil
}

func (client *GitLabClient) CreateWebhook(ctx context.Context, webhook CreateWebhook) error {
	_, _, err := client.Projects.AddProjectHook(webhook.ID, &gitlab.AddProjectHookOptions{
		Name:                  gitlab.Ptr("mkrepo"),
		Description:           gitlab.Ptr("mkrepo webhook"),
		URL:                   gitlab.Ptr(buildWebhookUrl(client.baseUrl, client.providerKey)),
		TagPushEvents:         gitlab.Ptr(true),
		Token:                 gitlab.Ptr(client.webhookSecret),
		EnableSSLVerification: gitlab.Ptr(true), // TODO: Make this configurable
	})
	return err
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
