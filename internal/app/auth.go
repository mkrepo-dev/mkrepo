package app

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/mkrepo-dev/mkrepo/internal/provider"
	"golang.org/x/oauth2"
)

type OAuth2State struct {
	State       string
	Verifier    *string
	ExpiresAt   time.Time
	RedirectURL string // TODO: Use thi field
}

func (s OAuth2State) Valid() bool {
	return time.Now().Before(s.ExpiresAt)
}

type Account struct {
	ID          uuid.UUID
	Provider    provider.ProviderKey
	Email       string
	Username    string
	DisplayName string
	AvatarURL   string
	Session     Session
}

type Session struct {
	ID        string
	Token     *oauth2.Token
	ExpiresAt time.Time
}

type contextKey int

const accountContextKey contextKey = iota

func GetAccountFromContext(ctx context.Context) *Account {
	account, ok := ctx.Value(accountContextKey).(*Account)
	if !ok {
		return nil
	}
	return account
}

func ContextWithAccount(ctx context.Context, account *Account) context.Context {
	return context.WithValue(ctx, accountContextKey, account)
}

type authRepo interface {
	GetAndDeleteOAuth2State(ctx context.Context, state string) (OAuth2State, error)
	GetAccountBySessionID(ctx context.Context, sessionID string) (Account, error)
}

type AuthService struct {
	logger    *slog.Logger
	repo      authRepo
	providers provider.Providers
}

func NewAuthService(logger *slog.Logger, repo authRepo, providers provider.Providers) *AuthService {
	return &AuthService{
		logger:    logger,
		repo:      repo,
		providers: providers,
	}
}

func (s *AuthService) Authenticate(ctx context.Context, sessionID string) (*Account, error) {
	account, err := s.repo.GetAccountBySessionID(ctx, sessionID)
	if err != nil {
		return nil, fmt.Errorf("get account by session ID: %w", err)
	}
	if time.Now().After(account.Session.ExpiresAt) {
		return nil, errors.New("session expired")
	}

	provider, ok := s.providers[account.Provider]
	if !ok {
		return nil, fmt.Errorf("unknown provider: %s", account.Provider)
	}
	token, err := provider.OAuth2Config().TokenSource(ctx, account.Session.Token).Token()
	if err != nil {
		return nil, fmt.Errorf("refresh access token: %w", err)
	}
	if token.AccessToken != account.Session.Token.AccessToken {
		user, err := provider.NewClient(ctx, token).GetUser(ctx)
		if err != nil {
			return nil, fmt.Errorf("get user info: %w", err)
		}
		account.Username = user.Username
		account.DisplayName = user.DisplayName
		account.AvatarURL = user.AvatarURL
		account.Session.Token = token
		err = s.repo.UpdateAccountWithSession(ctx, account)
		if err != nil {
			return nil, fmt.Errorf("update account with session: %w", err)
		}
	}

	return &account, nil
}
