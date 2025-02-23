package provider

import (
	"context"

	"github.com/google/go-github/v69/github"
)

type GitHub struct {
	client *github.Client
}

var _ ProviderClient = &GitHub{}

func NewGitHub(token string) *GitHub {
	return &GitHub{
		client: github.NewClient(nil).WithAuthToken(token),
	}
}

func (gh *GitHub) CreateRepo(ctx context.Context, repo NewRepo) error {
	_, _, err := gh.client.Repositories.Create(ctx, repo.Owner, &github.Repository{Name: &repo.Name})
	return err
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
		// TODO: Find if user is admin and owner if it is
		if org.GetMembersCanCreateRepos() {
			owners = append(owners, org.GetLogin())
		}
	}

	return owners, nil
}
