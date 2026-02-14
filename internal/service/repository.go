package service

import (
	"context"

	"github.com/mkrepo-dev/mkrepo/internal/types"
)

type Repository interface {
	GetTemplate(ctx context.Context, fullName string) (types.GetTemplateVersion, error)
	CreateTemplate(ctx context.Context, name string, fullName string, url *string, version string, description *string, language *string, schema []byte, buildIn bool) error
	SearchTemplates(ctx context.Context, query string) ([]types.GetTemplateVersion, error)
	UpdateTemplateStars(ctx context.Context, fullName string, stars int) error
	IncreaseTemplateUses(ctx context.Context, fullName string) error
}
