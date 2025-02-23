package provider

import "context"

type ProviderClient interface {
	CreateRepo(ctx context.Context, repo NewRepo) error
}

type NewRepo struct {
	Provider string
	Name     string
	Owner    string
}
