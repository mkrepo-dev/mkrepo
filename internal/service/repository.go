package service

import "context"

type Repository interface {
	GetTemplate(ctx context.Context, fullName string) (Template, error)
	CreateTemplate(ctx context.Context, name string, fullName string, url *string, version string, description *string, language *string, schema []byte, buildIn bool) error
	SearchTemplates(ctx context.Context, query string) ([]Template, error)
	UpdateTemplateStars(ctx context.Context, fullName string, stars int) error
	IncreaseTemplateUses(ctx context.Context, fullName string) error
}
