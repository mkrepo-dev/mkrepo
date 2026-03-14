package app

import (
	"context"
	"crypto/rand"
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
	CreateOAuth2State(ctx context.Context, state OAuth2State) error
	GetAccountBySessionID(ctx context.Context, sessionID string) (Account, error)
	CreateOrUpdateAccountWithSession(ctx context.Context, account Account) error
	UpdateAccountWithSession(ctx context.Context, account Account) error
	DeleteSession(ctx context.Context, sessionID string) error
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

func (s *AuthService) GetAuthURL(ctx context.Context, providerKey provider.ProviderKey, redirectURL string) (string, error) {
	provider, ok := s.providers[providerKey]
	if !ok {
		return "", fmt.Errorf("unknown provider: %s", providerKey)
	}
	pkce := provider.Features().OAuth2AuthorizationCodeFlowWithPKCE

	var verifier *string
	opts := []oauth2.AuthCodeOption{}
	if pkce {
		verifier = new(oauth2.GenerateVerifier())
		opts = append(opts, oauth2.S256ChallengeOption(*verifier))
	}

	state := rand.Text()
	err := s.repo.CreateOAuth2State(ctx, OAuth2State{
		State:       state,
		Verifier:    verifier,
		ExpiresAt:   time.Now().Add(15 * time.Minute),
		RedirectURL: redirectURL,
	})
	if err != nil {
		return "", fmt.Errorf("create oauth2 state: %w", err)
	}

	return provider.OAuth2Config().AuthCodeURL(state, opts...), nil
}

func (s *AuthService) LoginWithOAuth2Callback(ctx context.Context, providerKey provider.ProviderKey, state string, code string) (Session, error) {
	provider, ok := s.providers[providerKey]
	if !ok {
		return Session{}, fmt.Errorf("unknown provider: %s", providerKey)
	}
	pkce := provider.Features().OAuth2AuthorizationCodeFlowWithPKCE

	oauth2State, err := s.repo.GetAndDeleteOAuth2State(ctx, state)
	if err != nil {
		return Session{}, fmt.Errorf("get and delete oauth2 state: %w", err)
	}
	if !oauth2State.Valid() {
		return Session{}, fmt.Errorf("oauth2 state is not valid")
	}

	var authCodeOptions []oauth2.AuthCodeOption
	if pkce && oauth2State.Verifier != nil {
		authCodeOptions = append(authCodeOptions, oauth2.VerifierOption(*oauth2State.Verifier))
	}
	token, err := provider.OAuth2Config().Exchange(ctx, code, authCodeOptions...)
	if err != nil {
		return Session{}, fmt.Errorf("exchange code for token: %w", err)
	}

	user, err := provider.NewClient(ctx, token).GetUser(ctx)
	if err != nil {
		return Session{}, fmt.Errorf("get user info: %w", err)
	}

	account := Account{
		Provider:    providerKey,
		Email:       user.Email,
		Username:    user.Username,
		DisplayName: user.DisplayName,
		AvatarURL:   user.AvatarURL,
		Session: Session{
			ID:        rand.Text(),
			Token:     token,
			ExpiresAt: time.Now().Add(14 * 24 * time.Hour),
		},
	}

	err = s.repo.CreateOrUpdateAccountWithSession(ctx, account)
	if err != nil {
		return Session{}, fmt.Errorf("create or update account with session: %w", err)
	}

	return account.Session, nil
}

func (s *AuthService) Logout(ctx context.Context, sessionID string) error {
	err := s.repo.DeleteSession(ctx, sessionID)
	if err != nil {
		return fmt.Errorf("delete session: %w", err)
	}
	return nil
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
