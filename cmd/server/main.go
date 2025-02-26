package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/github"

	"github.com/FilipSolich/mkrepo/internal"
	"github.com/FilipSolich/mkrepo/internal/handler"
	"github.com/FilipSolich/mkrepo/internal/log"
	"github.com/FilipSolich/mkrepo/internal/middleware"
)

func main() {
	log.SetupLogger()

	version := internal.ReadVersion()
	slog.Info("Started mkrepo server",
		slog.String("version", version.Version), slog.String("goVersion", version.GoVersion),
		slog.String("revision", version.Revision[:7]), slog.String("buildDatetime", version.BuildDatetime),
	)

	mux := http.NewServeMux()
	mux.Handle("GET /", handler.NewIndex())

	login := handler.NewLogin(map[string]oauth2.Config{
		"github": {
			ClientID:     os.Getenv("GITHUB_CLIENT_ID"),
			ClientSecret: os.Getenv("GITHUB_CLIENT_SECRET"),
			Scopes:       []string{"repo", "read:org"},
			Endpoint:     github.Endpoint,
		},
	})
	mux.HandleFunc("GET /login", login.LoginProvider)
	mux.HandleFunc("GET /oauth2/callback/{provider}", login.Oauth2Callback)

	repo := handler.NewRepo()
	mux.Handle("GET /new", middleware.Authenticated(http.HandlerFunc(repo.Form)))
	mux.Handle("POST /new", middleware.Authenticated(http.HandlerFunc(repo.Create)))

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
