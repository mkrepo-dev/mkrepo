package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"go.uber.org/automaxprocs/maxprocs"

	"github.com/mkrepo-dev/mkrepo/internal"
	"github.com/mkrepo-dev/mkrepo/internal/config"
	"github.com/mkrepo-dev/mkrepo/internal/database"
	"github.com/mkrepo-dev/mkrepo/internal/log"
	"github.com/mkrepo-dev/mkrepo/internal/mkrepo"
	"github.com/mkrepo-dev/mkrepo/internal/provider"
	"github.com/mkrepo-dev/mkrepo/internal/server"
	"github.com/mkrepo-dev/mkrepo/template/docker"
	"github.com/mkrepo-dev/mkrepo/template/gitignore"
	"github.com/mkrepo-dev/mkrepo/template/license"
	"github.com/mkrepo-dev/mkrepo/template/template"
)

func main() {
	log.SetupLogger()
	version := internal.ReadVersion()
	slog.Info("Build info",
		slog.String("version", version.Version), slog.String("goVersion", version.GoVersion),
		slog.String("revision", version.Revision[:7]), slog.Time("buildDatetime", version.BuildDatetime),
	)

	_, err := maxprocs.Set(maxprocs.Logger(func(s string, i ...any) {
		slog.Info(fmt.Sprintf(s, i...))
	}))
	if err != nil {
		slog.Warn("Failed to set GOMAXPROCS", log.Err(err))
	}

	configFile := flag.String("config", "config.yaml", "Path to the configuration file")
	flag.Parse()

	cfg, err := config.LoadConfig(*configFile)
	if err != nil {
		slog.Error("Cannot load config", log.Err(err))
		os.Exit(1)
	}

	providers := provider.NewProvidersFromConfig(cfg)

	ctx := context.Background()
	db, err := database.New(ctx, cfg.DatabaseUri, cfg.Secret)
	if err != nil {
		slog.Error("Cannot open database", log.Err(err))
		os.Exit(1)
	}
	defer db.Close()
	go db.GarbageCollector(ctx, 12*time.Hour)

	gitignores, err := mkrepo.PrepareGitignores(gitignore.FS)
	if err != nil {
		slog.Error("Cannot prepare gitignores", log.Err(err))
		os.Exit(1)
	}

	licenses, err := mkrepo.PrepareLicenses(license.FS)
	if err != nil {
		slog.Error("Cannot prepare licenses", log.Err(err))
		os.Exit(1)
	}

	dockerfiles, err := mkrepo.PrepareDockerfiles(docker.FS)
	if err != nil {
		slog.Error("Cannot prepare dockerfiles", log.Err(err))
		os.Exit(1)
	}

	err = mkrepo.PrepareTemplates(db, template.FS)
	if err != nil {
		slog.Error("Cannot prepare templates", log.Err(err))
		os.Exit(1)
	}

	repomaker := mkrepo.New(db, gitignore.FS, licenses, dockerfiles, docker.FS, template.FS)

	srv := server.NewServer(cfg, db, repomaker, providers, gitignores, licenses, dockerfiles)

	errCh := make(chan error)
	go func() {
		slog.Info("Starting listening", slog.String("addr", srv.Addr))
		errCh <- srv.ListenAndServe() // TODO: Use TLS
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
		err := srv.Shutdown(ctx)
		if err != nil {
			slog.Error("Cannot gracefully shutdown server", log.Err(err))
			os.Exit(1)
		}
	}
}
