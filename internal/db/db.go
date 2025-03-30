package db

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"time"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/pgx"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/oauth2"

	"github.com/FilipSolich/mkrepo/internal/log"
	"github.com/FilipSolich/mkrepo/migration"
)

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

	err = db.Clenup(ctx)
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
			err := db.Clenup(cleanCtx)
			if err != nil {
				slog.Error("Failed to cleanup DB", log.Err(err))
			}
			cancel()
		case <-ctx.Done():
			return
		}
	}
}

func (db *DB) Clenup(ctx context.Context) error {
	_, err := db.Exec(ctx, `DELETE FROM "oauth2_state" WHERE "expires_at" < 'now'::timestamp;`)
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
	Session     string
	Provider    string
	Token       *oauth2.Token
	RedirectUri string
	Email       string
	Username    string
	DisplayName string
	AvatarURL   string
}

func GetAccount(accounts []Account, provider string, username string) *Account {
	for _, account := range accounts {
		if account.Provider == provider && (account.Username == username || username == "") {
			return &account
		}
	}
	return nil
}

func (db *DB) GetSessionAccounts(ctx context.Context, session string) ([]Account, error) {
	// TODO: Token should not be returned by this function
	// Token can be rotated when used ad token source. In case of gitlab tokens old token stops working
	// immediately after new token is generated. This mean that new token has to be store in db. Returning
	// token here can cause problems in parallel requests. Create functino GetAndRefreshToken which in transaction
	// locks account row, returns token and refreshes it and stores it if necesary. This way token can be safely used in parallel requests.
	// This function should be called each time before use.
	rows, err := db.Query(ctx,
		`SELECT "id", "session", "provider", "access_token", "refresh_token",
		 "expires_at", "redirect_uri", "email", "username", "display_name", "avatar_url"
		 FROM "account"
		 WHERE "session" = $1;`,
		session,
	)
	if err != nil {
		return nil, err
	}
	var accounts []Account
	for rows.Next() {
		account := Account{Token: &oauth2.Token{}}
		err = rows.Scan(
			&account.Id, &account.Session, &account.Provider, &account.Token.AccessToken,
			&account.Token.RefreshToken, &account.Token.Expiry, &account.RedirectUri, &account.Email,
			&account.Username, &account.DisplayName, &account.AvatarURL,
		)
		if err != nil {
			return nil, err
		}
		accounts = append(accounts, account)
	}
	return accounts, rows.Err()
}

// TODO: Find better place how to handle structs
type UserInfo struct {
	Username    string
	Email       string
	DisplayName string
	AvatarURL   string
}

func (db *DB) CreateOrOverwriteAccount(ctx context.Context, session string, provider string, token *oauth2.Token, redirectUri string, userInfo UserInfo) error {
	tx, err := db.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return err
	}
	defer rollback(tx)

	_, err = tx.Exec(ctx,
		`DELETE FROM "account"
		 WHERE "session" = $1 AND "provider" = $2 AND "username" = $3;`,
		session, provider, userInfo.Username,
	)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx,
		`INSERT INTO "account" ("session", "provider", "access_token", "refresh_token", "expires_at", "redirect_uri", "email", "username", "display_name", "avatar_url")
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10);`,
		session, provider, token.AccessToken, token.RefreshToken, token.Expiry, redirectUri,
		userInfo.Email, userInfo.Username, userInfo.DisplayName, userInfo.AvatarURL,
	)
	if err != nil {
		return err
	}

	return tx.Commit(ctx)
}

func (db *DB) UpdateAccountToken(ctx context.Context, session string, provider string, username string, token *oauth2.Token) error {
	_, err := db.Exec(ctx,
		`UPDATE "account"
		 SET "access_token" = $1, "refresh_token" = $2, "expires_at" = $3
		 WHERE "session" = $4 AND "provider" = $5 AND "username" = $6;`,
		token.AccessToken, token.RefreshToken, token.Expiry, session, provider, username,
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

func (db *DB) CreateTemplate(ctx context.Context, name string, description string, url string, version string) error {
	tx, err := db.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return err
	}
	defer rollback(tx)

	var id int64
	err = tx.QueryRow(ctx,
		`INSERT INTO "template" ("name", "url")
		 VALUES ($1, $2)
		 RETURNING "id";`,
		name, url,
	).Scan(&id)
	if err != nil {
		return err
	}

	_, err = tx.Exec(ctx,
		`INSERT INTO "template_name" ("description", "version", "template_id")
		 VALUES ($1, $2, $3);`,
		description, version, id,
	)
	if err != nil {
		return err
	}

	return tx.Commit(ctx)
}

func (db *DB) UpdateTemplateStars(ctx context.Context, url string, stars int) error {
	_, err := db.Exec(ctx,
		`UPDATE "template"
		 SET "stars" = $1
		 WHERE "url" = $2;`,
		stars, url,
	)
	return err
}

func (db *DB) CreateTemplateVersion(ctx context.Context, url string, description string, language string, version string) error {
	_, err := db.Exec(ctx,
		`INSERT INTO "template_name" ("description", "language", "version", "template_id")
		 VALUES ($1, $2, $3, (SELECT "id" FROM "template" WHERE "url" = $4 LIMIT 1));`,
		description, language, version, url,
	)
	return err
}

func rollback(tx pgx.Tx) {
	err := tx.Rollback(context.Background())
	if err != nil && !errors.Is(err, pgx.ErrTxClosed) {
		slog.Error("Failed to rollback transaction", log.Err(err))
	}
}
