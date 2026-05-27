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
	"time"

	"ariga.io/atlas/atlasexec"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/oauth2"

	"github.com/mkrepo-dev/mkrepo/internal/app"
	"github.com/mkrepo-dev/mkrepo/internal/gen/database"
	"github.com/mkrepo-dev/mkrepo/internal/log"
	"github.com/mkrepo-dev/mkrepo/internal/provider"
	"github.com/mkrepo-dev/mkrepo/internal/service"
	"github.com/mkrepo-dev/mkrepo/sql/migrations"
)

type Repository struct {
	pool          *pgxpool.Pool
	Queries       *database.Queries
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
	defer func() {
		if err != nil {
			pool.Close()
		}
	}()

	workDir, err := atlasexec.NewWorkingDir(atlasexec.WithMigrations(migrations.FS))
	if err != nil {
		return nil, fmt.Errorf("create working directory: %w", err)
	}
	defer workDir.Close()
	migrationClient, err := atlasexec.NewClient(workDir.Path(), "atlas")
	if err != nil {
		return nil, fmt.Errorf("create atlas client: %w", err)
	}
	res, err := migrationClient.MigrateApply(ctx, &atlasexec.MigrateApplyParams{
		URL: connectionUri,
	})
	if err != nil {
		return nil, fmt.Errorf("apply migrations: %w", err)
	}
	slog.Info("Migrations applied", "applied", len(res.Applied), "current", res.Current)

	// Create repository
	repo := &Repository{
		pool:          pool,
		Queries:       database.New(pool),
		encryptionKey: key,
	}

	// Run initial cleanup
	err = repo.Cleanup(ctx)
	if err != nil {
		return nil, fmt.Errorf("initial cleanup: %w", err)
	}

	return repo, nil
}

func (r *Repository) Close() {
	r.pool.Close()
}

func (r *Repository) Cleanup(ctx context.Context) error {
	err := r.Queries.DeleteExpiredOAuth2States(ctx)
	if err != nil {
		return fmt.Errorf("delete expired oauth2 states: %w", err)
	}
	err = r.Queries.DeleteExpiredSessions(ctx)
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

func (r *Repository) GetAndDeleteOAuth2State(ctx context.Context, state string) (app.OAuth2State, error) {
	dbState, err := r.Queries.GetAndDeleteOAuth2State(ctx, state)
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
	dbAccount, err := r.Queries.GetAccountBySession(ctx, sessionID)
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

func (r *Repository) UpdateAccountWithSession(ctx context.Context, account app.Account) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("create transaction: %w", err)
	}
	defer tx.Rollback(ctx) // nolint:errcheck
	qtx := r.Queries.WithTx(tx)

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

func (r *Repository) SearchTemplates(ctx context.Context, query string) ([]service.Template, error) {
	rows, err := r.Queries.SearchTemplates(ctx, query)
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
	row, err := r.Queries.GetTemplate(ctx, fullName)
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
	qtx := r.Queries.WithTx(tx)

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
	return r.Queries.UpdateTemplateStars(ctx, database.UpdateTemplateStarsParams{
		FullName: fullName,
		Stars:    int32(stars),
	})
}

func (r *Repository) IncreaseTemplateUses(ctx context.Context, fullName string) error {
	return r.Queries.IncreaseTemplateUses(ctx, fullName)
}

func (r *Repository) Encrypt(data []byte) ([]byte, error) {
	block, err := aes.NewCipher(r.encryptionKey)
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

func (r *Repository) Decrypt(data []byte) ([]byte, error) {
	block, err := aes.NewCipher(r.encryptionKey)
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
