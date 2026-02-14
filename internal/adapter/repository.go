package adapter

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"time"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/pgx/v5"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/oauth2"

	"github.com/mkrepo-dev/mkrepo/internal/app"
	"github.com/mkrepo-dev/mkrepo/internal/generated/database"
	"github.com/mkrepo-dev/mkrepo/internal/log"
	"github.com/mkrepo-dev/mkrepo/internal/provider"
	"github.com/mkrepo-dev/mkrepo/internal/service"
	"github.com/mkrepo-dev/mkrepo/sql/migrations"
)

type Repository struct {
	pool          *pgxpool.Pool
	queries       *database.Queries
	encryptionKey []byte
}

func New(ctx context.Context, connectionUri string, encryptionKey string) (*Repository, error) {
	// Decode encryption key from hex
	key, err := hex.DecodeString(encryptionKey)
	if err != nil {
		return nil, fmt.Errorf("decode encryption key: %w", err)
	}

	// Create connection pool
	pool, err := pgxpool.New(ctx, connectionUri)
	if err != nil {
		return nil, fmt.Errorf("create pool: %w", err)
	}

	// Ping database
	err = pool.Ping(ctx)
	if err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping database: %w", err)
	}

	// Run migrations
	driver, err := iofs.New(migrations.FS, ".")
	if err != nil {
		pool.Close()
		return nil, fmt.Errorf("create migration source: %w", err)
	}

	migrationUri := strings.ReplaceAll(connectionUri, "postgres://", "pgx5://")
	m, err := migrate.NewWithSourceInstance("iofs", driver, migrationUri)
	if err != nil {
		pool.Close()
		return nil, fmt.Errorf("create migrator: %w", err)
	}

	err = m.Up()
	if err != nil && !errors.Is(err, migrate.ErrNoChange) {
		pool.Close()
		return nil, fmt.Errorf("run migrations: %w", err)
	}

	// Create repository
	repo := &Repository{
		pool:          pool,
		queries:       database.New(pool),
		encryptionKey: key,
	}

	// Run initial cleanup
	err = repo.Cleanup(ctx)
	if err != nil {
		pool.Close()
		return nil, fmt.Errorf("initial cleanup: %w", err)
	}

	return repo, nil
}

func (r *Repository) Close() {
	r.pool.Close()
}

func (r *Repository) Cleanup(ctx context.Context) error {
	err := r.queries.DeleteExpiredOAuth2States(ctx)
	if err != nil {
		return fmt.Errorf("delete expired oauth2 states: %w", err)
	}
	err = r.queries.DeleteExpiredSessions(ctx)
	if err != nil {
		return fmt.Errorf("delete expired sessions: %w", err)
	}
	return nil
}

func (r *Repository) GarbageCollector(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			cleanCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
			err := r.Cleanup(cleanCtx)
			if err != nil {
				slog.Error("Failed to cleanup repository", log.Err(err))
			}
			cancel()
		case <-ctx.Done():
			return
		}
	}
}

func (r *Repository) Ping(ctx context.Context) error {
	return r.pool.Ping(ctx)
}

func (r *Repository) CreateOAuth2State(ctx context.Context, state app.OAuth2State) error {
	verifier := pgtype.Text{}
	if state.Verifier != nil {
		verifier = pgtype.Text{String: *state.Verifier, Valid: true}
	}
	return r.queries.CreateOAuth2State(ctx, database.CreateOAuth2StateParams{
		State:     state.State,
		Verifier:  verifier,
		ExpiresAt: pgtype.Timestamptz{Time: state.ExpiresAt, Valid: true},
	})
}

func (r *Repository) GetAndDeleteOAuth2State(ctx context.Context, state string) (app.OAuth2State, error) {
	dbState, err := r.queries.GetAndDeleteOAuth2State(ctx, state)
	if err != nil {
		return app.OAuth2State{}, err
	}
	oauth2State := app.OAuth2State{
		State:     dbState.State,
		ExpiresAt: dbState.ExpiresAt.Time,
	}
	if dbState.Verifier.Valid {
		oauth2State.Verifier = &dbState.Verifier.String
	}
	return oauth2State, nil
}

func (r *Repository) GetAccountBySessionID(ctx context.Context, sessionID string) (app.Account, error) {
	dbAccount, err := r.queries.GetAccountBySession(ctx, sessionID)
	if err != nil {
		return app.Account{}, fmt.Errorf("get account by session id: %w", err)
	}

	accessToken, err := decrypt(r.encryptionKey, dbAccount.AccessToken)
	if err != nil {
		return app.Account{}, fmt.Errorf("decrypt access token: %w", err)
	}
	refreshToken, err := decrypt(r.encryptionKey, dbAccount.RefreshToken)
	if err != nil {
		return app.Account{}, fmt.Errorf("decrypt refresh token: %w", err)
	}
	return app.Account{
		ID:          dbAccount.ID,
		Provider:    provider.ProviderKey(dbAccount.Provider),
		Email:       dbAccount.Email,
		Username:    dbAccount.Username,
		DisplayName: dbAccount.DisplayName,
		AvatarURL:   dbAccount.AvatarUrl,
		Session: app.Session{
			ID: sessionID,
			Token: &oauth2.Token{
				AccessToken:  string(accessToken),
				RefreshToken: string(refreshToken),
				Expiry:       dbAccount.AccessTokenExpiresAt.Time,
			},
			ExpiresAt: dbAccount.ExpiresAt.Time,
		},
	}, nil
}

func (r *Repository) CreateOrUpdateAccountWithSession(ctx context.Context, account app.Account) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("create transaction: %w", err)
	}
	defer tx.Rollback(ctx) // nolint:errcheck
	qtx := r.queries.WithTx(tx)

	var accountID uuid.UUID
	dbAccount, err := qtx.GetAccountByProviderAndEmail(ctx, database.GetAccountByProviderAndEmailParams{
		Provider: string(account.Provider),
		Email:    account.Email,
	})
	if err != nil {
		accountID = uuid.Must(uuid.NewV7())
		err = qtx.CreateAccount(ctx, database.CreateAccountParams{
			ID:          accountID,
			Provider:    string(account.Provider),
			Email:       account.Email,
			Username:    account.Username,
			DisplayName: account.DisplayName,
			AvatarUrl:   account.AvatarURL,
		})
		if err != nil {
			return fmt.Errorf("create account: %w", err)
		}
	} else {
		accountID = dbAccount.ID
		err = qtx.UpdateAccount(ctx, database.UpdateAccountParams{
			ID:          accountID,
			Username:    account.Username,
			DisplayName: account.DisplayName,
			AvatarUrl:   account.AvatarURL,
		})
		if err != nil {
			return fmt.Errorf("update account: %w", err)
		}
	}

	accessToken, err := encrypt(r.encryptionKey, []byte(account.Session.Token.AccessToken))
	if err != nil {
		return fmt.Errorf("encrypt access token: %w", err)
	}
	refreshToken, err := encrypt(r.encryptionKey, []byte(account.Session.Token.RefreshToken))
	if err != nil {
		return fmt.Errorf("encrypt refresh token: %w", err)
	}
	var accessTokenExpiresAt pgtype.Timestamptz
	if !account.Session.Token.Expiry.IsZero() {
		accessTokenExpiresAt = pgtype.Timestamptz{
			Time:  account.Session.Token.Expiry,
			Valid: true,
		}
	}

	err = qtx.CreateSession(ctx, database.CreateSessionParams{
		ID:                   account.Session.ID,
		AccessToken:          accessToken,
		RefreshToken:         refreshToken,
		AccessTokenExpiresAt: accessTokenExpiresAt,
		ExpiresAt:            pgtype.Timestamptz{Time: account.Session.ExpiresAt, Valid: true},
		AccountID:            accountID,
	})
	if err != nil {
		return fmt.Errorf("create session: %w", err)
	}

	err = tx.Commit(ctx)
	if err != nil {
		return fmt.Errorf("commit transaction: %w", err)
	}
	return nil
}

func (r *Repository) UpdateAccountWithSession(ctx context.Context, account app.Account) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("create transaction: %w", err)
	}
	defer tx.Rollback(ctx) // nolint:errcheck
	qtx := r.queries.WithTx(tx)

	accessToken, err := encrypt(r.encryptionKey, []byte(account.Session.Token.AccessToken))
	if err != nil {
		return fmt.Errorf("encrypt access token: %w", err)
	}
	refreshToken, err := encrypt(r.encryptionKey, []byte(account.Session.Token.RefreshToken))
	if err != nil {
		return fmt.Errorf("encrypt refresh token: %w", err)
	}
	var accessTokenExpiresAt pgtype.Timestamptz
	if !account.Session.Token.Expiry.IsZero() {
		accessTokenExpiresAt = pgtype.Timestamptz{
			Time:  account.Session.Token.Expiry,
			Valid: true,
		}
	}
	err = qtx.UpdateSession(ctx, database.UpdateSessionParams{
		ID:                   account.Session.ID,
		AccessToken:          accessToken,
		RefreshToken:         refreshToken,
		AccessTokenExpiresAt: accessTokenExpiresAt,
		ExpiresAt:            pgtype.Timestamptz{Time: account.Session.ExpiresAt, Valid: true},
	})
	if err != nil {
		return fmt.Errorf("update session: %w", err)
	}
	err = qtx.UpdateAccount(ctx, database.UpdateAccountParams{
		ID:          account.ID,
		Username:    account.Username,
		DisplayName: account.DisplayName,
		AvatarUrl:   account.AvatarURL,
	})
	if err != nil {
		return fmt.Errorf("update account: %w", err)
	}

	err = tx.Commit(ctx)
	if err != nil {
		return fmt.Errorf("commit transaction: %w", err)
	}
	return nil
}

func (r *Repository) DeleteSession(ctx context.Context, sessionID string) error {
	return r.queries.DeleteSession(ctx, sessionID)
}

func (r *Repository) SearchTemplates(ctx context.Context, query string) ([]service.Template, error) {
	rows, err := r.queries.SearchTemplates(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("search templates: %w", err)
	}

	templates := make([]service.Template, 0, len(rows))
	for _, row := range rows {
		template := service.Template{
			Name:     row.Name,
			FullName: row.FullName,
			BuildIn:  row.BuildIn,
			Stars:    int(row.Stars),
			Version:  row.Version,
		}
		if row.Url.Valid {
			template.Url = &row.Url.String
		}
		if row.Description.Valid {
			template.Description = &row.Description.String
		}
		if row.Language.Valid {
			template.Language = &row.Language.String
		}
		templates = append(templates, template)
	}
	return templates, nil
}

func (r *Repository) GetTemplate(ctx context.Context, fullName string) (service.Template, error) {
	row, err := r.queries.GetTemplate(ctx, fullName)
	if err != nil {
		return service.Template{}, fmt.Errorf("get template: %w", err)
	}

	template := service.Template{
		Name:     row.Name,
		FullName: row.FullName,
		BuildIn:  row.BuildIn,
		Stars:    int(row.Stars),
		Version:  row.Version,
	}
	if row.Url.Valid {
		template.Url = &row.Url.String
	}
	if row.Description.Valid {
		template.Description = &row.Description.String
	}
	if row.Language.Valid {
		template.Language = &row.Language.String
	}
	if len(row.Schema) > 0 {
		var schema map[string]any
		// Schema is already jsonb, so we can assign it directly
		// Note: This assumes Schema is stored as a JSON object
		template.Schema = &schema
	}
	return template, nil
}

func (r *Repository) CreateTemplate(ctx context.Context, name string, fullName string, url *string, version string, description *string, language *string, schema []byte, buildIn bool) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback(ctx) // nolint:errcheck
	qtx := r.queries.WithTx(tx)

	var urlText pgtype.Text
	if url != nil {
		urlText = pgtype.Text{String: *url, Valid: true}
	}

	err = qtx.InsertTemplateIfNotExists(ctx, database.InsertTemplateIfNotExistsParams{
		Name:     name,
		FullName: fullName,
		Url:      urlText,
		BuildIn:  buildIn,
	})
	if err != nil {
		return fmt.Errorf("insert template: %w", err)
	}

	var descText pgtype.Text
	if description != nil {
		descText = pgtype.Text{String: *description, Valid: true}
	}
	var langText pgtype.Text
	if language != nil {
		langText = pgtype.Text{String: *language, Valid: true}
	}

	err = qtx.InsertTemplateVersion(ctx, database.InsertTemplateVersionParams{
		Description: descText,
		Language:    langText,
		Version:     version,
		Schema:      schema,
		FullName:    fullName,
	})
	if err != nil {
		return fmt.Errorf("insert template version: %w", err)
	}

	return tx.Commit(ctx)
}

func (r *Repository) UpdateTemplateStars(ctx context.Context, fullName string, stars int) error {
	return r.queries.UpdateTemplateStars(ctx, database.UpdateTemplateStarsParams{
		FullName: fullName,
		Stars:    int32(stars),
	})
}

func (r *Repository) IncreaseTemplateUses(ctx context.Context, fullName string) error {
	return r.queries.IncreaseTemplateUses(ctx, fullName)
}

func encrypt(key []byte, data []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	aesGCM, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, aesGCM.NonceSize())
	_, err = io.ReadFull(rand.Reader, nonce)
	if err != nil {
		return nil, err
	}

	return aesGCM.Seal(nonce, nonce, []byte(data), nil), nil
}

func decrypt(key []byte, data []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	aesGCM, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	if len(data) < aesGCM.NonceSize() {
		return nil, errors.New("ciphertext too short")
	}
	nonce, ciphertext := data[:aesGCM.NonceSize()], data[aesGCM.NonceSize():]
	plaintext, err := aesGCM.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, err
	}
	return plaintext, nil
}
