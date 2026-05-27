package handler

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"html/template"
	"io/fs"
	"log/slog"
	"net/http"
	"time"

	"github.com/google/uuid"

	"github.com/mkrepo-dev/mkrepo/internal/adapter"
	"github.com/mkrepo-dev/mkrepo/internal/app"
	"github.com/mkrepo-dev/mkrepo/internal/gen/database"
	"github.com/mkrepo-dev/mkrepo/internal/log"
	"github.com/mkrepo-dev/mkrepo/internal/provider"
	"golang.org/x/oauth2"
)

type Account struct {
	ID                uuid.UUID
	ProviderKey       provider.ProviderKey
	ProviderAccountID string
	Email             string
	Username          string
	DisplayName       string
	AvatarURL         string
	Token             *oauth2.Token
}

const sessionCookieName = "session"

func sessionCookie(value string, maxAge int) *http.Cookie {
	return &http.Cookie{
		Name:     sessionCookieName,
		Value:    value,
		Path:     "/",
		MaxAge:   maxAge,
		Secure:   true,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	}
}

func AuthenticateMiddleware(logger *slog.Logger, db *adapter.Repository, providers provider.Providers) func(http.Handler) http.Handler {
	logger = logger.With("middleware", "Authenticate")
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()
			reqSession, err := r.Cookie(sessionCookieName)
			if err != nil {
				next.ServeHTTP(w, r.WithContext(ctx))
				return
			}

			account, err := authenticateSession(ctx, db, reqSession.Value, providers)
			if err != nil {
				http.SetCookie(w, sessionCookie("", -1))
				next.ServeHTTP(w, r.WithContext(ctx))
				return
			}

			next.ServeHTTP(w, r.WithContext(app.ContextWithAccount(ctx, account)))
		})
	}
}

func Login(logger *slog.Logger, templatesFS fs.FS, db *adapter.Repository, providers provider.Providers) http.HandlerFunc {
	logger = logger.With("handler", "Login")
	type loginContext struct {
		baseContext
		Providers provider.Providers
	}
	tmpl := template.Must(template.ParseFS(templatesFS, "base.html", "login.html"))

	return func(w http.ResponseWriter, r *http.Request) {
		providerKey := r.FormValue("provider")
		if providerKey == "" {
			render(w, tmpl, loginContext{
				baseContext: getBaseContext(r),
				Providers:   providers,
			})
		}

		provider, ok := providers[provider.ProviderKey(providerKey)]
		if !ok {
			http.Error(w, "Unknown provider", http.StatusBadRequest)
			return
		}

		var verifier *string
		opts := []oauth2.AuthCodeOption{}
		if provider.Features().OAuth2AuthorizationCodeFlowWithPKCE {
			verifier = new(oauth2.GenerateVerifier())
			opts = append(opts, oauth2.S256ChallengeOption(*verifier))
		}

		state := rand.Text()
		err := db.Queries.CreateOAuth2State(r.Context(), database.CreateOAuth2StateParams{
			State:     state,
			Verifier:  verifier,
			ExpiresAt: time.Now().Add(15 * time.Minute),
		})
		if err != nil {
			logger.Error("Failed to create OAuth2 state", log.Err(err))
			http.Error(w, "Failed to create OAuth2 state", http.StatusInternalServerError)
			return
		}

		authURL := provider.OAuth2Config().AuthCodeURL(state, opts...)
		http.Redirect(w, r, authURL, http.StatusFound)
	}
}

func Logout(logger *slog.Logger, db *adapter.Repository) http.HandlerFunc {
	logger = logger.With("handler", "Logout")
	return func(w http.ResponseWriter, r *http.Request) {
		reqSession, err := r.Cookie(sessionCookieName)
		if err != nil {
			http.Redirect(w, r, "/", http.StatusFound)
			return
		}

		err = db.Queries.DeleteSession(r.Context(), reqSession.Value)
		if err != nil {
			logger.Error("Failed to delete session from database.", log.Err(err))
		}

		// TODO: Invalidate token if possible

		http.SetCookie(w, sessionCookie("", -1))
		http.Redirect(w, r, "/", http.StatusFound)
	}
}

func OAuth2Callback(logger *slog.Logger, db *adapter.Repository, providers provider.Providers) http.HandlerFunc {
	logger = logger.With("handler", "OAuth2Callback")
	return func(w http.ResponseWriter, r *http.Request) {
		errMsg := r.FormValue("error")
		if errMsg != "" {
			http.Error(w, "OAuth2 error: "+errMsg, http.StatusBadRequest)
			return
		}

		providerKey := r.PathValue("provider")
		provider, ok := providers[provider.ProviderKey(providerKey)]
		if !ok {
			http.Error(w, "Unknown provider", http.StatusBadRequest)
			return
		}

		oauth2State, err := db.Queries.GetAndDeleteOAuth2State(r.Context(), r.FormValue("state"))
		if err != nil {
			http.Error(w, "Invalid state", http.StatusBadRequest)
			return
		}
		if time.Now().After(oauth2State.ExpiresAt) {
			http.Error(w, "State is expired", http.StatusBadRequest)
			return
		}
		var authCodeOptions []oauth2.AuthCodeOption
		if oauth2State.Verifier != nil {
			authCodeOptions = append(authCodeOptions, oauth2.VerifierOption(*oauth2State.Verifier))
		}

		token, err := provider.OAuth2Config().Exchange(r.Context(), r.FormValue("code"), authCodeOptions...)
		if err != nil {
			http.Error(w, "Failed to exchange code for token", http.StatusInternalServerError)
			return
		}

		user, err := provider.NewClient(context.TODO(), token).GetUser(r.Context()) // TODO: This shouldnt take context becaise token should be rotated in middleware
		if err != nil {
			logger.Error("Failed to get user info", log.Err(err))
			http.Error(w, "Failed to get user info", http.StatusInternalServerError)
			return
		}

		accountID, err := db.Queries.UpsertAccount(r.Context(), database.UpsertAccountParams{
			ID:                uuid.Must(uuid.NewV7()),
			Provider:          providerKey,
			ProviderAccountID: user.ID,
			Email:             user.Email,
			Username:          user.Username,
			DisplayName:       user.DisplayName,
			AvatarUrl:         user.AvatarURL,
		})
		if err != nil {
			logger.Error("Failed to upsert account", log.Err(err))
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}

		sessionID := rand.Text()
		accessToken, err := db.Encrypt([]byte(token.AccessToken))
		if err != nil {
			logger.Error("Failed to encrypt access token", log.Err(err))
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}
		refreshToken, err := db.Encrypt([]byte(token.RefreshToken))
		if err != nil {
			logger.Error("Failed to encrypt refresh token", log.Err(err))
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}
		var accessTokenExpires *time.Time
		if !token.Expiry.IsZero() {
			accessTokenExpires = &token.Expiry
		}
		sessionExpiresAt := time.Now().Add(14 * 24 * time.Hour)

		err = db.Queries.CreateSession(r.Context(), database.CreateSessionParams{
			ID:                   sessionID,
			AccessToken:          accessToken,
			AccessTokenExpiresAt: accessTokenExpires,
			RefreshToken:         refreshToken,
			ExpiresAt:            sessionExpiresAt,
			AccountID:            accountID,
		})
		if err != nil {
			logger.Error("Failed to create session", log.Err(err))
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}

		http.SetCookie(w, sessionCookie(sessionID, int(time.Until(sessionExpiresAt).Seconds())))
		http.Redirect(w, r, "/", http.StatusFound)
	}
}

func authenticateSession(ctx context.Context, db *adapter.Repository, sessionID string, providers provider.Providers) (*Account, error) {
	accountSession, err := db.Queries.GetAccountSession(ctx, sessionID)
	if err != nil {
		return nil, fmt.Errorf("cannot get account by session ID")
	}
	if time.Now().After(accountSession.ExpiresAt) {
		return nil, errors.New("session expired")
	}

	provider, ok := providers[provider.ProviderKey(accountSession.Provider)]
	if !ok {
		return nil, fmt.Errorf("unknown provider: %s", accountSession.Provider)
	}

	accessToken, refreshToken, err := decryptTokens(db, accountSession.AccessToken, accountSession.RefreshToken)
	if err != nil {
		return nil, fmt.Errorf("decrypt tokens: %w", err)
	}
	token := &oauth2.Token{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
	}
	if accountSession.AccessTokenExpiresAt != nil {
		token.Expiry = *accountSession.AccessTokenExpiresAt
	}

	token, err = provider.OAuth2Config().TokenSource(ctx, token).Token()
	if err != nil {
		return nil, fmt.Errorf("refresh access token: %w", err)
	}
	if token.AccessToken != string(accessToken) {
		accessTokenEnc, refreshTokenEnc, err := encryptTokens(db, token.AccessToken, token.RefreshToken)
		if err != nil {
			return nil, fmt.Errorf("encrypt tokens: %w", err)
		}
		user, err := provider.NewClient(ctx, token).GetUser(ctx)
		if err != nil {
			return nil, fmt.Errorf("get user info: %w", err)
		}
		err = db.Queries.UpdateAccount(ctx, database.UpdateAccountParams{
			ID:          accountSession.ID,
			Username:    user.Username,
			Email:       user.Email,
			DisplayName: user.DisplayName,
			AvatarUrl:   user.AvatarURL,
		})
		if err != nil {
			return nil, fmt.Errorf("update account: %w", err)
		}
		err = db.Queries.UpdateSession(ctx, database.UpdateSessionParams{
			ID:                   accountSession.SessionID,
			AccessToken:          accessTokenEnc,
			RefreshToken:         refreshTokenEnc,
			AccessTokenExpiresAt: &token.Expiry,
		})
		if err != nil {
			return nil, fmt.Errorf("update session: %w", err)
		}
	}

	return &Account{
		ID:                accountSession.ID,
		ProviderKey:       provider.ProviderKey(accountSession.Provider),
		ProviderAccountID: accountSession.ProviderAccountID,
		Email:             accountSession.Email,
		Username:          accountSession.Username,
		DisplayName:       accountSession.DisplayName,
		AvatarURL:         accountSession.AvatarUrl,
		Token:             token,
	}, nil
}

func encryptTokens(db *adapter.Repository, accessToken, refreshToken string) ([]byte, []byte, error) {
	accessTokenEnc, err := db.Encrypt([]byte(accessToken))
	if err != nil {
		return nil, nil, fmt.Errorf("encrypt access token: %w", err)
	}
	var refreshTokenEnc []byte
	if refreshToken != "" {
		refreshTokenEnc, err = db.Encrypt([]byte(refreshToken))
		if err != nil {
			return nil, nil, fmt.Errorf("encrypt refresh token: %w", err)
		}
	}
	return accessTokenEnc, refreshTokenEnc, nil
}

func decryptTokens(db *adapter.Repository, accessTokenEnc, refreshTokenEnc []byte) (string, string, error) {
	accessToken, err := db.Decrypt(accessTokenEnc)
	if err != nil {
		return "", "", fmt.Errorf("decrypt access token: %w", err)
	}
	var refreshToken string
	if refreshTokenEnc != nil {
		refreshTokenBytes, err := db.Decrypt(refreshTokenEnc)
		if err != nil {
			return "", "", fmt.Errorf("decrypt refresh token: %w", err)
		}
		refreshToken = string(refreshTokenBytes)
	}
	return string(accessToken), refreshToken, nil
}
