package main

import (
	"context"
	"flag"
	"log/slog"
	"net/http"
	"os/signal"
	"syscall"
	"time"

	"github.com/FilipSolich/mkrepo/internal"
	"github.com/FilipSolich/mkrepo/internal/config"
	"github.com/FilipSolich/mkrepo/internal/db"
	"github.com/FilipSolich/mkrepo/internal/handler"
	"github.com/FilipSolich/mkrepo/internal/log"
	"github.com/FilipSolich/mkrepo/internal/middleware"
	"github.com/FilipSolich/mkrepo/internal/provider"
)

func main() {
	log.SetupLogger()

	version := internal.ReadVersion()
	slog.Info("Started mkrepo server",
		slog.String("version", version.Version), slog.String("goVersion", version.GoVersion),
		slog.String("revision", version.Revision[:7]), slog.Time("buildDatetime", version.BuildDatetime),
	)

	configFile := flag.String("config", "config.yaml", "Path to the configuration file")
	flag.Parse()

	cfg, err := config.LoadConfig(*configFile)
	if err != nil {
		log.Fatal("Cannot load config", err)
	}

	providers := provider.NewProvidersFromConfig(cfg.Providers)

	db, err := db.NewDB(context.Background(), "./db.sqlite")
	if err != nil {
		log.Fatal("Cannot open database", err)
	}
	defer db.Close()

	mux := http.NewServeMux()
	mux.Handle("GET /", handler.NewIndex(providers))

	auth := handler.NewAuth(cfg, db, providers)
	mux.HandleFunc("GET /auth/login", auth.Login)
	mux.HandleFunc("GET /auth/logout", auth.Logout)
	mux.HandleFunc("GET /auth/oauth2/callback/{provider}", auth.OAuth2Callback)

	repo := handler.NewRepo(cfg, providers)
	mux.Handle("GET /new", http.HandlerFunc(repo.Form))
	mux.Handle("POST /new", http.HandlerFunc(repo.Create))

	webhook := handler.NewWebhook(db)
	mux.Handle("POST /webhook/handler", http.HandlerFunc(webhook.Handle))

	wrapped := middleware.NewAuthenticate(db)(mux)

	server := &http.Server{
		Addr:         ":8000",
		Handler:      wrapped,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 60 * time.Second,
		IdleTimeout:  120 * time.Second,
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
			log.Fatal("Cannot run server", err)
		}
	case <-ctx.Done():
		timeout := 15 * time.Second
		slog.Info("Shutting down server", slog.Duration("timeout", timeout))
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()
		err := server.Shutdown(ctx)
		if err != nil {
			log.Fatal("Cannot gracefully shutdown server", err)
		}
	}
}
