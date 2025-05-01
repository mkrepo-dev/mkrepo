package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
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
	"github.com/mkrepo-dev/mkrepo/template"
	templatefs "github.com/mkrepo-dev/mkrepo/template/template"
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
		log.Fatal("Cannot load config", err)
	}

	providers := provider.NewProvidersFromConfig(cfg)

	ctx := context.Background()
	db, err := database.New(ctx, "postgres://mkrepo:mkrepo@localhost:5432/mkrepo?sslmode=disable") // TODO: Use this from env or config
	if err != nil {
		log.Fatal("Cannot open database", err)
	}
	defer db.Close()
	go db.GarbageCollector(ctx, 12*time.Hour)

	licenses, err := template.PrepareLicenses(template.LicenseFS)
	if err != nil {
		log.Fatal("Cannot prepare licenses", err)
	}
	slog.Info("Licenses prepared", slog.Int("count", len(licenses)))

	err = template.PrepareTemplates(db, templatefs.FS)
	if err != nil {
		log.Fatal("Cannot prepare templates", err)
	}
	slog.Info("Templates prepared")

	repomaker := mkrepo.New(licenses)

	srv := server.NewServer(db, repomaker, providers, licenses)

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
			log.Fatal("Cannot run server", err)
		}
	case <-ctx.Done():
		timeout := 15 * time.Second
		slog.Info("Shutting down server", slog.Duration("timeout", timeout))
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()
		err := srv.Shutdown(ctx)
		if err != nil {
			log.Fatal("Cannot gracefully shutdown server", err)
		}
	}
}
