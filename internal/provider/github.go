package provider

import (
	"context"

	"github.com/FilipSolich/mkrepo/internal"
	"github.com/google/go-github/v69/github"
)

type GitHub struct {
	client *github.Client
}

var _ ProviderClient = &GitHub{}

func NewGitHub(token string) *GitHub {
	client := github.NewClient(nil).WithAuthToken(token)
	client.UserAgent = internal.UserAgent
	return &GitHub{client: client}
}

func (gh *GitHub) CreateRemoteRepo(ctx context.Context, repo internal.Repo) (string, string, error) {
	var org string
	// TODO: Fix this. AuthorName is diffrent then user login. Find a way to diffrentiate between user and org.
	if repo.Owner != repo.AuthorName {
		org = repo.Owner
	}
	r, _, err := gh.client.Repositories.Create(ctx, org, &github.Repository{
		Name:        &repo.Name,
		Description: &repo.Description,
		Visibility:  &repo.Visibility,
	})
	if err != nil {
		return "", "", err
	}
	return r.GetHTMLURL(), r.GetCloneURL(), nil
}

func (gh *GitHub) CreateWebhook(ctx context.Context, repo internal.Repo) error {
	_, _, err := gh.client.Repositories.CreateHook(ctx, repo.Owner, repo.Name, &github.Hook{ // TODO: Make sure repo name is correct here
		Active: github.Ptr(true),
		Events: []string{"push"},
		Config: &github.HookConfig{
			ContentType: github.Ptr("json"),
			InsecureSSL: github.Ptr("0"),
			URL:         github.Ptr("https://example.com/webhook"), // TODO: Change this
		},
	})
	return err
}

func (gh *GitHub) GetGitAuthor(ctx context.Context) (string, string, error) {
	user, _, err := gh.client.Users.Get(ctx, "")
	if err != nil {
		return "", "", err
	}
	return user.GetName(), user.GetEmail(), nil
}

func (gh *GitHub) GetPossibleRepoOwners(ctx context.Context) ([]string, error) {
	var owners []string
	user, _, err := gh.client.Users.Get(ctx, "")
	if err != nil {
		return nil, err
	}
	owners = append(owners, user.GetLogin())

	orgs, _, err := gh.client.Organizations.List(ctx, "", nil)
	if err != nil {
		return owners, err
	}
	for _, org := range orgs {
		org, _, err := gh.client.Organizations.Get(ctx, org.GetLogin())
		if err != nil {
			return owners, err
		}
		if org.GetMembersCanCreateRepos() {
			owners = append(owners, org.GetLogin())
			continue
		}
		membership, _, err := gh.client.Organizations.GetOrgMembership(ctx, "", org.GetLogin())
		if err != nil {
			return owners, err
		}
		if membership.GetRole() == "admin" {
			owners = append(owners, org.GetLogin())
		}
	}

	return owners, nil
}
