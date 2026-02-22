package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"runtime"
	"syscall"
	"time"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/mkrepo-dev/mkrepo"
	"github.com/mkrepo-dev/mkrepo/internal"
	"github.com/mkrepo-dev/mkrepo/internal/adapter"
	"github.com/mkrepo-dev/mkrepo/internal/config"
	"github.com/mkrepo-dev/mkrepo/internal/log"
	"github.com/mkrepo-dev/mkrepo/internal/metrics"
	"github.com/mkrepo-dev/mkrepo/internal/provider"
	"github.com/mkrepo-dev/mkrepo/internal/server"
	"github.com/mkrepo-dev/mkrepo/internal/service"
	"github.com/mkrepo-dev/mkrepo/template/gitignore"
	"github.com/mkrepo-dev/mkrepo/template/license"
	"github.com/mkrepo-dev/mkrepo/template/template"
)

func printHelp() {
	fmt.Println("Usage: mkrepo <command>")
	fmt.Println("Commands:")
	fmt.Println("  server   - Start the mkrepo server")
	fmt.Println("  license  - Print the embedded license template")
	fmt.Println("  readme   - Print the embedded readme template")
}

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	if len(os.Args) < 2 {
		printHelp()
		os.Exit(0)
	}

	switch os.Args[1] {
	case "license":
		fmt.Println(mkrepo.License)
		os.Exit(0)
	case "readme":
		fmt.Println(mkrepo.Readme)
		os.Exit(0)
	case "help":
		printHelp()
		os.Exit(0)
	case "server":
	default:
		fmt.Println("Unknown command:", os.Args[1])
		printHelp()
		os.Exit(1)
	}

	configFile := flag.String("config", "config.yaml", "Path to the configuration file")
	flag.Parse()

	log.SetupLogger()
	slog.Info("Build info",
		slog.String("version", internal.Build.Version), slog.String("goVersion", internal.Build.GoVersion),
		slog.String("revision", internal.Build.Revision), slog.Time("buildDatetime", internal.Build.BuildDatetime),
	)
	slog.Info("Runtime info", slog.Int("GOMAXPROCS", runtime.GOMAXPROCS(0)), slog.Int("numCPU", runtime.NumCPU()))

	cfg, err := config.LoadConfig(*configFile)
	if err != nil {
		slog.Error("Cannot load config", log.Err(err))
		os.Exit(1)
	}

	providers := provider.NewProvidersFromConfig(cfg)

	db, err := adapter.New(ctx, cfg.DatabaseUri, cfg.SecretKey)
	if err != nil {
		slog.Error("Cannot open database", log.Err(err))
		os.Exit(1)
	}
	defer db.Close()
	go db.GarbageCollector(ctx, 12*time.Hour)

	reg := prometheus.NewRegistry()
	metrics := metrics.NewMetrics(reg, nil)

	var gitignoresFS fs.FS = gitignore.FS
	if cfg.GitignoresDir != "" {
		gitignoresFS = os.DirFS(cfg.GitignoresDir)
	}
	gitignores, err := service.PrepareGitignores(gitignoresFS)
	if err != nil {
		slog.Error("Cannot prepare gitignores", log.Err(err))
		os.Exit(1)
	}

	var licensesFS fs.FS = license.FS
	if cfg.LicensesDir != "" {
		licensesFS = os.DirFS(cfg.LicensesDir)
	}
	licenses, err := service.ParseLicenses(service.LicensesConfig{}, licensesFS)
	if err != nil {
		slog.Error("Cannot prepare licenses", log.Err(err))
		os.Exit(1)
	}

	var templatesFS fs.FS = template.FS
	if cfg.TemplatesDir != "" {
		templatesFS = os.DirFS(cfg.TemplatesDir)
	}
	err = service.PrepareTemplates(db, templatesFS)
	if err != nil {
		slog.Error("Cannot prepare templates", log.Err(err))
		os.Exit(1)
	}

	repomaker := service.NewService(metrics, db, gitignoresFS, licenses, templatesFS)

	srv := server.NewServer(cfg, reg, metrics, db, repomaker, providers, gitignores, licenses)

	errCh := make(chan error)
	go func() {
		slog.Info("Starting listening", slog.String("addr", srv.Addr))
		errCh <- srv.ListenAndServe() // TODO: Use TLS
		close(errCh)
	}()

	select {
	case err := <-errCh:
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
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
