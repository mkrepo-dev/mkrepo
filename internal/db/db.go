package db

import (
	"context"
	"errors"
	"log/slog"

	"github.com/jackc/pgx/v5"
	"golang.org/x/oauth2"

	"github.com/FilipSolich/mkrepo/internal/log"
)

type DB struct {
	*pgx.Conn
}

func NewDB(ctx context.Context, datasource string) (*DB, error) {
	conn, err := pgx.Connect(ctx, datasource)
	if err != nil {
		return nil, err
	}
	db := &DB{Conn: conn}

	_, err = db.Exec(ctx, `CREATE TABLE IF NOT EXISTS "template" (
		"id" SERIAL PRIMARY KEY,
		"name" TEXT NOT NULL,
		"url" TEXT NOT NULL UNIQUE,
		"version" TEXT NOT NULL DEFAULT 'v0.0.0',
		"stars" INTEGER NOT NULL DEFAULT 0,
		"created_at" TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
	);`)
	if err != nil {
		return nil, err
	}

	_, err = db.Exec(ctx, `CREATE TABLE IF NOT EXISTS "account" (
		"id" SERIAL PRIMARY KEY,
		"session" TEXT NOT NULL,
		"provider" TEXT NOT NULL,
		"access_token" TEXT NOT NULL,
		"refresh_token" TEXT NOT NULL,
		"expiry" TIMESTAMP NOT NULL DEFAULT 'epoch',
		"redirect_uri" TEXT NOT NULL,
		"email" TEXT NOT NULL,
		"username" TEXT NOT NULL,
		"display_name" TEXT NOT NULL,
		"avatar_url" TEXT NOT NULL,
		UNIQUE("session", "provider", "username")
	);`)
	if err != nil {
		return nil, err
	}

	return db, nil
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
	rows, err := db.Query(ctx,
		`SELECT "id", "session", "provider", "access_token", "refresh_token",
		 "expiry", "redirect_uri", "email", "username", "display_name", "avatar_url"
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
	defer func() {
		err := tx.Rollback(ctx)
		if err != nil && !errors.Is(err, pgx.ErrTxClosed) {
			slog.Error("Failed to rollback transaction", log.Err(err))
		}
	}()

	_, err = tx.Exec(ctx, `DELETE FROM "account" WHERE "session" = $1 AND "provider" = $2 AND "username" = $3;`, session, provider, userInfo.Username)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx,
		`INSERT INTO "account" ("session", "provider", "access_token", "refresh_token", "expiry", "redirect_uri", "email", "username", "display_name", "avatar_url")
		 VALUES ($1, $2, $3, $4, TO_TIMESTAMP($5), $6, $7, $8, $9, $10);`,
		session, provider, token.AccessToken, token.RefreshToken, token.Expiry.Unix(), redirectUri,
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
		 SET "access_token" = $1, "refresh_token" = $2, "expiry" = TO_TIMESTAMP($3)
		 WHERE "session" = $4 AND "provider" = $5 AND "username" = $6;`,
		token.AccessToken, token.RefreshToken, token.Expiry.Unix(), session, provider, username,
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
