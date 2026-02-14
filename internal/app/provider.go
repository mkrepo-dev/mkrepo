package app

import (
	"context"

	"golang.org/x/oauth2"
)

// TODO: This all should be moved to adapters and app should use local interface
// well all accept provider key and provders map

type ProviderKey string

type Providers map[ProviderKey]Provider

type Provider interface {
	Features() ProviderFeatures
	OAuth2Config() *oauth2.Config
	Client(token *oauth2.Token) ProviderClient
}

type ProviderClient interface {
	GetUser(ctx context.Context) (ProviderUser, error)
}

type ProviderFeatures struct {
	OAuth2AuthorizationCodeFlowWithPKCE bool
	Sha256Repo                          bool
}

type ProviderUser struct {
	Username    string
	Email       string
	DisplayName string
	AvatarURL   string
}
