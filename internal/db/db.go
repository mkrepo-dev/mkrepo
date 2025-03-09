package db

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"time"

	"github.com/FilipSolich/mkrepo/internal/log"
	_ "github.com/mattn/go-sqlite3"
	"golang.org/x/oauth2"
)

type DB struct {
	*sql.DB
}

func NewDB(ctx context.Context, datasource string) (*DB, error) {
	db, err := sql.Open("sqlite3", datasource)
	if err != nil {
		return nil, err
	}

	_, err = db.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS "template" (
		"id" INTEGER NOT NULL UNIQUE,
		"name" TEXT NOT NULL,
		"url" TEXT NOT NULL UNIQUE,
		"version" TEXT NOT NULL DEFAULT 'v0.0.0',
		"stars" INTEGER NOT NULL DEFAULT 0,
		"created_at" INTEGER NOT NULL DEFAULT CURRENT_TIMESTAMP,
		PRIMARY KEY("id" AUTOINCREMENT)
	) STRICT;`)
	if err != nil {
		return nil, err
	}
	_, err = db.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS "account" (
		"id" INTEGER NOT NULL UNIQUE,
		"session" TEXT NOT NULL,
		"provider" TEXT NOT NULL,
		"access_token" TEXT NOT NULL,
		"refresh_token" TEXT NOT NULL,
		"expiry" INTEGER NOT NULL DEFAULT 0,
		"email" TEXT NOT NULL,
		"username" TEXT NOT NULL,
		"display_name" TEXT NOT NULL,
		"avatar_url" TEXT NOT NULL,
		PRIMARY KEY("id" AUTOINCREMENT),
		UNIQUE("session", "provider", "username")
	) STRICT;`)
	if err != nil {
		return nil, err
	}

	return &DB{DB: db}, nil
}

type Account struct {
	Id          int
	Session     string
	Provider    string
	Token       *oauth2.Token
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
	rows, err := db.QueryContext(ctx,
		`SELECT "id", "session", "provider", "access_token", "refresh_token", "expiry", "email", "username", "display_name", "avatar_url"
		 FROM "account"
		 WHERE "session" = ?;`,
		session,
	)
	if err != nil {
		return nil, err
	}
	var accounts []Account
	for rows.Next() {
		var account Account
		var accessToken, refreshToken string
		var expiry int64
		err = rows.Scan(&account.Id, &account.Session, &account.Provider, &accessToken, &refreshToken, &expiry, &account.Email, &account.Username, &account.DisplayName, &account.AvatarURL)
		if err != nil {
			return nil, err
		}
		account.Token = &oauth2.Token{AccessToken: accessToken, RefreshToken: refreshToken, Expiry: time.Unix(expiry, 0)}
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

func (db *DB) CreateOrOverwriteAccount(ctx context.Context, session string, provider string, token *oauth2.Token, userInfo UserInfo) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() {
		err := tx.Rollback()
		if err != nil && !errors.Is(err, sql.ErrTxDone) {
			slog.Error("Failed to rollback transaction", log.Err(err))
		}
	}()

	// TODO: Use prepare statement for this and use it for delete account also.
	_, err = tx.ExecContext(ctx, `DELETE FROM "account" WHERE "session" = ? AND "provider" = ? AND "username" = ?;`, session, provider, userInfo.Username)
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx,
		`INSERT INTO "account" ("session", "provider", "access_token", "refresh_token", "expiry", "email", "username", "display_name", "avatar_url")
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?);`,
		session, provider, token.AccessToken, token.RefreshToken, token.Expiry.Unix(),
		userInfo.Email, userInfo.Username, userInfo.DisplayName, userInfo.AvatarURL,
	)
	if err != nil {
		return err
	}

	return tx.Commit()
}

func (db *DB) DeleteAccount(ctx context.Context, session string, provider string, username string) error {
	_, err := db.ExecContext(ctx,
		`DELETE FROM "account"
		 WHERE "session" = ? AND "provider" = ? AND "username" = ?;`,
		session, provider, username,
	)
	return err
}
