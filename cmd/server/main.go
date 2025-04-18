package main

import (
	"context"
	"flag"
	"log/slog"
	"net/http"
	"os/signal"
	"syscall"
	"time"

	"github.com/mkrepo-dev/mkrepo/internal"
	"github.com/mkrepo-dev/mkrepo/internal/config"
	"github.com/mkrepo-dev/mkrepo/internal/db"
	"github.com/mkrepo-dev/mkrepo/internal/handler"
	"github.com/mkrepo-dev/mkrepo/internal/log"
	"github.com/mkrepo-dev/mkrepo/internal/middleware"
	"github.com/mkrepo-dev/mkrepo/internal/mkrepo"
	"github.com/mkrepo-dev/mkrepo/internal/provider"
	"github.com/mkrepo-dev/mkrepo/static"
	"github.com/mkrepo-dev/mkrepo/template"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

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

	providers := provider.NewProvidersFromConfig(cfg)

	licenses, err := template.PrepareLicenses(template.LicenseFS)
	if err != nil {
		log.Fatal("Cannot prepare licenses", err)
	}

	db, err := db.New(ctx, "postgres://mkrepo:mkrepo@localhost:5432/mkrepo?sslmode=disable") // TODO: Use this from env or config
	if err != nil {
		log.Fatal("Cannot open database", err)
	}
	defer db.Close()
	go db.GarbageCollector(ctx, 12*time.Hour)

	repomaker := mkrepo.New(db, providers, licenses)

	mux := http.NewServeMux()
	mux.Handle("GET /", handler.NewIndex(providers))
	mux.Handle("GET /static/", http.StripPrefix("/static/", http.FileServer(http.FS(static.FS))))

	auth := handler.NewAuth(db, providers)
	mux.HandleFunc("GET /auth/login", auth.Login)
	mux.HandleFunc("GET /auth/logout", auth.Logout)
	mux.HandleFunc("GET /auth/oauth2/callback/{provider}", auth.OAuth2Callback)

	repo := handler.NewRepo(db, repomaker, providers, licenses)
	mux.Handle("GET /new", http.HandlerFunc(repo.Form))
	mux.Handle("POST /new", http.HandlerFunc(repo.Create))

	mux.Handle("POST /webhook/{provider}", handler.Webhook(db, providers))

	wrapped := middleware.NewAuthenticate(db)(mux)

	server := &http.Server{
		Addr:         ":8080",
		Handler:      wrapped,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 60 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	errCh := make(chan error)
	go func() {
		slog.Info("Starting listening", slog.String("addr", server.Addr))
		errCh <- server.ListenAndServe() // TODO: Use TLS
	}()

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
