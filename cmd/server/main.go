package main

import (
	"context"
	"database/sql"
	"flag"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	_ "github.com/mattn/go-sqlite3"

	"github.com/FilipSolich/mkrepo/internal"
	"github.com/FilipSolich/mkrepo/internal/config"
	"github.com/FilipSolich/mkrepo/internal/handler"
	"github.com/FilipSolich/mkrepo/internal/log"
	"github.com/FilipSolich/mkrepo/internal/middleware"
	"github.com/FilipSolich/mkrepo/internal/provider"
)

func main() {
	log.SetupLogger()

	configFile := flag.String("config", "config.yaml", "Path to the configuration file")
	flag.Parse()
	cfg, err := config.LoadConfig(*configFile)
	if err != nil {
		log.Fatal("Cannot load config", err)
	}

	version := internal.ReadVersion()
	slog.Info("Started mkrepo server",
		slog.String("version", version.Version), slog.String("goVersion", version.GoVersion),
		slog.String("revision", version.Revision[:7]), slog.String("buildDatetime", version.BuildDatetime),
	)

	providers := provider.NewProvidersFromConfig(cfg.Providers)

	mux := http.NewServeMux()
	mux.Handle("GET /", handler.NewIndex(providers))

	login := handler.NewLogin(providers)
	mux.HandleFunc("GET /login", login.LoginProvider)
	mux.HandleFunc("GET /oauth2/callback/{provider}", login.Oauth2Callback)

	repo := handler.NewRepo(providers)
	mux.Handle("GET /new", middleware.Authenticated(http.HandlerFunc(repo.Form)))
	mux.Handle("POST /new", middleware.Authenticated(http.HandlerFunc(repo.Create)))

	db, err := sql.Open("sqlite3", "./db.sqlite")
	if err != nil {
		log.Fatal("Cannot open database", err)
	}
	defer db.Close()
	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS "template" (
		"id" INTEGER NOT NULL UNIQUE,
		"name" TEXT NOT NULL,
		"url" TEXT NOT NULL UNIQUE,
		"version" TEXT NOT NULL DEFAULT 'v0.0.0',
		"stars"	INTEGER NOT NULL DEFAULT 0,
		"created_at" INTEGER NOT NULL DEFAULT CURRENT_TIMESTAMP,
		PRIMARY KEY("id" AUTOINCREMENT)
	) STRICT;`)
	if err != nil {
		log.Fatal("Cannot create table", err)
	}
	webhook := handler.NewWebhook(db)
	mux.Handle("POST /webhook/handler", http.HandlerFunc(webhook.Handle))

	server := &http.Server{
		Addr:         ":8000",
		Handler:      mux,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 60 * time.Second,
		IdleTimeout:  90 * time.Second,
	}

	errCh := make(chan error)
	go func() {
		slog.Info("Starting listening", slog.String("addr", server.Addr))
		errCh <- server.ListenAndServe()
	}()

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	select {
	case err := <-errCh:
		if err != nil && err != http.ErrServerClosed {
			slog.Error("Cannot run server", log.Err(err))
			os.Exit(1)
		}
	case <-ctx.Done():
		timeout := 15 * time.Second
		slog.Info("Shutting down server", slog.Duration("timeout", timeout))
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()
		err := server.Shutdown(ctx)
		if err != nil {
			slog.Error("Cannot gracefully shutdown server", log.Err(err))
			os.Exit(1)
		}
	}
}
