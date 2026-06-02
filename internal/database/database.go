package database

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
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/mkrepo-dev/mkrepo/internal/gen/database"
	"github.com/mkrepo-dev/mkrepo/internal/log"
	"github.com/mkrepo-dev/mkrepo/sql/migrations"
)

type DB struct {
	logger        *slog.Logger
	pool          *pgxpool.Pool
	Queries       *database.Queries
	encryptionKey []byte
}

func New(ctx context.Context, logger *slog.Logger, connectionUri string, encryptionKey string) (*DB, error) {
	logger = log.Component(logger, "database")
	key, err := hex.DecodeString(encryptionKey)
	if err != nil {
		return nil, fmt.Errorf("decode encryption key: %w", err)
	}

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
	logger.InfoContext(ctx, "Migrations applied", slog.Int("applied", len(res.Applied)), slog.String("current", res.Current))

	repo := &DB{
		logger:        logger,
		pool:          pool,
		Queries:       database.New(pool),
		encryptionKey: key,
	}

	err = repo.Cleanup(ctx)
	if err != nil {
		return nil, fmt.Errorf("initial cleanup: %w", err)
	}

	return repo, nil
}

func (r *DB) Close() {
	r.pool.Close()
}

func (r *DB) Cleanup(ctx context.Context) error {
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

func (r *DB) GarbageCollector(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			cleanCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
			err := r.Cleanup(cleanCtx)
			if err != nil {
				r.logger.ErrorContext(cleanCtx, "Failed to cleanup database", log.Err(err))
			}
			cancel()
		case <-ctx.Done():
			return
		}
	}
}

//func (r *DB) SearchTemplates(ctx context.Context, query string) ([]service.Template, error) {
//	rows, err := r.Queries.SearchTemplates(ctx, query)
//	if err != nil {
//		return nil, fmt.Errorf("search templates: %w", err)
//	}
//
//	templates := make([]service.Template, 0, len(rows))
//	for _, row := range rows {
//		template := service.Template{
//			Name:        row.Name,
//			FullName:    row.FullName,
//			BuildIn:     row.BuildIn,
//			Stars:       int(row.Stars),
//			Version:     row.Version,
//			Url:         row.Url,
//			Description: row.Description,
//			Language:    row.Language,
//		}
//		templates = append(templates, template)
//	}
//	return templates, nil
//}
//
//func (r *DB) GetTemplate(ctx context.Context, fullName string) (service.Template, error) {
//	row, err := r.Queries.GetTemplate(ctx, fullName)
//	if err != nil {
//		return service.Template{}, fmt.Errorf("get template: %w", err)
//	}
//
//	template := service.Template{
//		Name:        row.Name,
//		FullName:    row.FullName,
//		BuildIn:     row.BuildIn,
//		Stars:       int(row.Stars),
//		Version:     row.Version,
//		Url:         row.Url,
//		Description: row.Description,
//		Language:    row.Language,
//	}
//	if len(row.Schema) > 0 {
//		var schema map[string]any
//		// Schema is already jsonb, so we can assign it directly
//		// Note: This assumes Schema is stored as a JSON object
//		template.Schema = &schema
//	}
//	return template, nil
//}
//
//func (r *DB) CreateTemplate(ctx context.Context, name string, fullName string, url *string, version string, description *string, language *string, schema []byte, buildIn bool) error {
//	tx, err := r.pool.Begin(ctx)
//	if err != nil {
//		return fmt.Errorf("begin transaction: %w", err)
//	}
//	defer tx.Rollback(ctx) // nolint:errcheck
//	qtx := r.Queries.WithTx(tx)
//
//	err = qtx.InsertTemplateIfNotExists(ctx, database.InsertTemplateIfNotExistsParams{
//		Name:     name,
//		FullName: fullName,
//		Url:      url,
//		BuildIn:  buildIn,
//	})
//	if err != nil {
//		return fmt.Errorf("insert template: %w", err)
//	}
//
//	err = qtx.InsertTemplateVersion(ctx, database.InsertTemplateVersionParams{
//		Description: description,
//		Language:    language,
//		Version:     version,
//		Schema:      schema,
//		FullName:    fullName,
//	})
//	if err != nil {
//		return fmt.Errorf("insert template version: %w", err)
//	}
//
//	return tx.Commit(ctx)
//}

func (r *DB) Encrypt(data []byte) ([]byte, error) {
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

func (r *DB) Decrypt(data []byte) ([]byte, error) {
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
