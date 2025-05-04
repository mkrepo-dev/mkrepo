package database

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"time"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/pgx"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	"github.com/jackc/pgconn"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/oauth2"

	"github.com/mkrepo-dev/mkrepo/internal/log"
	"github.com/mkrepo-dev/mkrepo/internal/provider"
	"github.com/mkrepo-dev/mkrepo/internal/types"
	"github.com/mkrepo-dev/mkrepo/migration"
)

var ErrAlreadyExists = errors.New("already exists")

type DB struct {
	*pgxpool.Pool
}

func New(ctx context.Context, datasource string) (*DB, error) {
	pool, err := pgxpool.New(ctx, datasource)
	if err != nil {
		return nil, err
	}
	db := &DB{Pool: pool}

	err = db.Ping(ctx)
	if err != nil {
		return nil, err
	}

	driver, err := iofs.New(migration.FS, ".")
	if err != nil {
		return nil, err
	}
	m, err := migrate.NewWithSourceInstance("iofs", driver, strings.ReplaceAll(datasource, "postgres://", "pgx://"))
	if err != nil {
		return nil, err
	}
	err = m.Up()
	if err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return nil, err
	}

	err = db.Cleanup(ctx)
	if err != nil {
		return nil, err
	}

	return db, nil
}

func (db *DB) GarbageCollector(ctx context.Context, interval time.Duration) {
	ticker := time.Tick(interval)
	for {
		select {
		case <-ticker:
			cleanCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
			err := db.Cleanup(cleanCtx)
			if err != nil {
				slog.Error("Failed to cleanup DB", log.Err(err))
			}
			cancel()
		case <-ctx.Done():
			return
		}
	}
}

func (db *DB) Cleanup(ctx context.Context) error {
	_, err := db.Exec(ctx, `DELETE FROM "oauth2_state" WHERE "expires_at" < 'now'::timestamp;`)
	if err != nil {
		return err
	}
	_, err = db.Exec(ctx, `DELETE FROM "session" WHERE "expires_at" < 'now'::timestamp;`)
	return err
}

func (db *DB) CreateOAuth2State(ctx context.Context, state string, expires_at time.Time) error {
	_, err := db.Exec(ctx,
		`INSERT INTO "oauth2_state" ("state", "expires_at")
		 VALUES ($1, $2);`,
		state, expires_at,
	)
	return err
}

func (db *DB) GetAndDeleteOAuth2State(ctx context.Context, state string) (string, time.Time, error) {
	var expiresAt time.Time
	err := db.QueryRow(ctx,
		`DELETE FROM "oauth2_state"
		 WHERE "state" = $1
		 RETURNING "expires_at";`,
		state,
	).Scan(&expiresAt)
	if err != nil {
		return "", time.Time{}, err
	}
	return state, expiresAt, nil
}

type Account struct {
	Id          int
	Provider    string
	Token       *oauth2.Token
	RedirectUri string
	Email       string
	Username    string
	DisplayName string
	AvatarURL   string
}

func (db *DB) GetAccountBySession(ctx context.Context, session string) (Account, error) {
	// TODO: Token should not be returned by this function
	// Token can be rotated when used ad token source. In case of gitlab tokens old token stops working
	// immediately after new token is generated. This mean that new token has to be store in db. Returning
	// token here can cause problems in parallel requests. Create functino GetAndRefreshToken which in transaction
	// locks account row, returns token and refreshes it and stores it if necesary. This way token can be safely used in parallel requests.
	// This function should be called each time before use.
	account := Account{Token: &oauth2.Token{}}
	err := db.QueryRow(ctx,
		`SELECT a."id", a."provider", a."access_token", a."refresh_token",
		 a."expires_at", a."redirect_uri", a."email", a."username", a."display_name", a."avatar_url"
		 FROM "account" a JOIN "session" s ON a."id" = s."account_id"
		 WHERE "session" = $1;`,
		session,
	).Scan(
		&account.Id, &account.Provider, &account.Token.AccessToken,
		&account.Token.RefreshToken, &account.Token.Expiry, &account.RedirectUri, &account.Email,
		&account.Username, &account.DisplayName, &account.AvatarURL,
	)
	return account, err
}

// TODO: Find better place how to handle structs
type UserInfo struct {
	Username    string
	Email       string
	DisplayName string
	AvatarURL   string
}

func (db *DB) CreateAccountSession(ctx context.Context, session string, sessionExpiresAt time.Time, provider string, token *oauth2.Token, redirectUri string, userInfo provider.User) error {
	tx, err := db.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return err
	}
	defer rollback(ctx, tx)

	var id int
	err = tx.QueryRow(ctx,
		`UPDATE "account"
		 SET "access_token" = $1, "refresh_token" = $2, "expires_at" = $3
		 WHERE "provider" = $4 AND "username" = $5
		 RETURNING "id";`,
		token.AccessToken, token.RefreshToken, token.Expiry, provider, userInfo.Username,
	).Scan(&id)
	if err != nil {
		if !errors.Is(err, pgx.ErrNoRows) {
			return err
		}
		err = tx.QueryRow(ctx,
			`INSERT INTO "account" ("provider", "access_token", "refresh_token", "expires_at", "redirect_uri", "email", "username", "display_name", "avatar_url")
			 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
			 RETURNING "id";`,
			provider, token.AccessToken, token.RefreshToken, token.Expiry, redirectUri,
			userInfo.Email, userInfo.Username, userInfo.DisplayName, userInfo.AvatarUrl,
		).Scan(&id)
		if err != nil {
			return err
		}
	}

	_, err = tx.Exec(ctx,
		`INSERT INTO "session" ("session", "expires_at", "account_id")
		 VALUES ($1, $2, $3);`,
		session, sessionExpiresAt, id,
	)
	if err != nil {
		return err
	}

	return tx.Commit(ctx)
}

func (db *DB) UpdateAccountToken(ctx context.Context, provider string, username string, token *oauth2.Token) error {
	_, err := db.Exec(ctx,
		`UPDATE "account"
		 SET "access_token" = $1, "refresh_token" = $2, "expires_at" = $3
		 WHERE "provider" = $4 AND "username" = $5;`,
		token.AccessToken, token.RefreshToken, token.Expiry, provider, username,
	)
	return err
}

func (db *DB) DeleteAccount(ctx context.Context, session string, provider string, username string) error {
	_, err := db.Exec(ctx,
		`DELETE FROM "account"
		 WHERE "session" = $1 AND "provider" = $2 AND "username" = $3;`,
		session, provider, username,
	)
	return err
}

func (db *DB) DeleteSession(ctx context.Context, session string) error {
	_, err := db.Exec(ctx,
		`DELETE FROM "session"
		 WHERE "session" = $1;`,
		session,
	)
	return err
}

func (db *DB) CreateTemplate(ctx context.Context, name string, fullName string, url *string, version string, description *string, language *string, buildIn bool) error {
	tx, err := db.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return err
	}
	defer rollback(ctx, tx)

	// TODO: First get then insert to not generate new id from serial type
	_, err = tx.Exec(ctx,
		`INSERT INTO "template" ("name", "full_name", "url", "build_in")
		 VALUES ($1, $2, $3, $4)
		 ON CONFLICT ("full_name") DO NOTHING;`,
		name, fullName, url, buildIn,
	)
	if err != nil {
		return err
	}

	_, err = tx.Exec(ctx,
		`INSERT INTO "template_version" ("description", "language", "version", "template_id")
		 VALUES ($1, $2, $3, (SELECT "id" FROM "template" WHERE "full_name" = $4));`,
		description, language, version, fullName,
	)
	if err != nil {
		var pgErr *pgconn.PgError
		if !errors.As(err, &pgErr) || pgErr.Code != "23505" {
			return ErrAlreadyExists
		}
		return err
	}

	return tx.Commit(ctx)
}

func (db *DB) SearchTemplates(ctx context.Context, query string) ([]types.GetTemplateVersion, error) {
	rows, err := db.Query(ctx,
		`SELECT t."name", t."full_name", t."url", t."build_in", t."stars", tv."version", tv."description", tv."language"
		 FROM "template" t JOIN "template_version" tv ON t."id" = tv."template_id"
		 WHERE tv."version" = (
		   SELECT "version" FROM "template_version" WHERE "template_id" = t."id" ORDER BY "version" DESC LIMIT 1
		 ) AND t."name" ~ $1
		 ORDER BY t."build_in" DESC, t."stars" DESC
		 LIMIT 10;`, // TODO: Do fulltext search on name, fullname and description. And optional filter by language.
		query,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []types.GetTemplateVersion
	for rows.Next() {
		var template types.GetTemplateVersion
		err = rows.Scan(&template.Name, &template.FullName, &template.Url, &template.BuildIn,
			&template.Stars, &template.Version, &template.Description, &template.Language)
		if err != nil {
			return nil, err
		}
		results = append(results, template)
	}

	return results, nil
}

// Returns template with specified version or the latest version if version is nil
func (db *DB) GetTemplate(ctx context.Context, fullName string, version *string) (types.GetTemplateVersion, error) {
	var template types.GetTemplateVersion
	var row pgx.Row
	if version != nil {
		row = db.QueryRow(ctx,
			`SELECT t."name", t."full_name", t."url", t."build_in", t."stars", tv."version", tv."description", tv."language"
			 FROM "template" t JOIN "template_version" tv ON t."id" = tv."template_id"
			 WHERE t."full_name" = $1
			 ORDER BY tv."version" DESC
			 LIMIT 1;`,
			fullName, version,
		)
	} else {
		row = db.QueryRow(ctx,
			`SELECT t."name", t."full_name", t."url", t."build_in", t."stars", tv."version", tv."description", tv."language"
			 FROM "template" t JOIN "template_version" tv ON t."id" = tv."template_id"
			 WHERE t."full_name" = $1 AND tv."version" = $2;`,
			fullName, version,
		)

	}
	err := row.Scan(&template.Name, &template.FullName, &template.Url, &template.BuildIn,
		&template.Stars, &template.Version, &template.Description, &template.Language)
	if err != nil {
		return types.GetTemplateVersion{}, err
	}
	return template, nil
}

func (db *DB) UpdateTemplateStars(ctx context.Context, fullName string, stars int) error {
	_, err := db.Exec(ctx,
		`UPDATE "template"
		 SET "stars" = $1
		 WHERE "full_name" = $2;`,
		stars, fullName,
	)
	return err
}

func (db *DB) IncreaseTemplateUses(ctx context.Context, fullName string) error {
	_, err := db.Exec(ctx,
		`UPDATE "template"
		 SET "used" = "used" + 1
		 WHERE "full_name" = $2;`,
		fullName,
	)
	return err
}

func rollback(ctx context.Context, tx pgx.Tx) {
	err := tx.Rollback(ctx)
	if err != nil && !errors.Is(err, pgx.ErrTxClosed) {
		slog.Error("Failed to rollback transaction", log.Err(err))
	}
}
