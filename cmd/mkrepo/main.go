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
	"sort"
	"syscall"
	"time"

	"github.com/mkrepo-dev/mkrepo"
	"github.com/mkrepo-dev/mkrepo/internal"
	"github.com/mkrepo-dev/mkrepo/internal/config"
	"github.com/mkrepo-dev/mkrepo/internal/database"
	"github.com/mkrepo-dev/mkrepo/internal/log"
	"github.com/mkrepo-dev/mkrepo/internal/provider"
	"github.com/mkrepo-dev/mkrepo/internal/server"
	"github.com/mkrepo-dev/mkrepo/internal/service"
	"github.com/mkrepo-dev/mkrepo/template/gitignore"
	"github.com/mkrepo-dev/mkrepo/template/license"
	"github.com/mkrepo-dev/mkrepo/template/template"
)

var ErrNoPrint = errors.New("no print")

func main() {
	err := run(context.Background(), os.Args)
	if err != nil {
		if !errors.Is(err, ErrNoPrint) {
			fmt.Fprintln(os.Stderr, "Error:", err)
		}
		os.Exit(1)
	}
}

func run(ctx context.Context, args []string) error {
	ctx, stop := signal.NotifyContext(ctx, syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	serverCmd := flag.NewFlagSet("server", flag.ExitOnError)
	serverConfigFile := serverCmd.String("config", "config.yaml", "Path to the configuration file")

	flag.Usage = func() {
		commands := [][]string{
			{"help", "Show this help message"},
			{"version", "Show version"},
			{"license", "Show license"},
			{"readme", "Show readme"},
			{"server", "Start the mkrepo server"},
		}
		commandsCopy := make([][]string, len(commands))
		copy(commandsCopy, commands)
		sort.Slice(commandsCopy, func(i, j int) bool {
			return len(commandsCopy[i][0]) > len(commandsCopy[j][0])
		})
		padding := len(commandsCopy[0][0]) + 2
		fmt.Printf("Create and template new repository\n\n")
		fmt.Printf("Usage: %s <command> [options]\n\n", args[0])
		fmt.Printf("Available commands:\n")
		for _, cmd := range commands {
			fmt.Printf("  %-*s%s\n", padding, cmd[0], cmd[1])
		}
	}

	if len(args) < 2 {
		flag.Usage()
		return nil
	}

	switch args[1] {
	case "help":
		flag.Usage()
		return nil
	case "version":
		fmt.Printf("version: %s (%s)\n", internal.Build.Version, internal.Build.GoVersion)
		fmt.Printf("revision: %s\n", internal.Build.Revision)
		fmt.Printf("build datetime: %s\n", internal.Build.BuildDatetime.Format(time.RFC3339))
		return nil
	case "license":
		fmt.Print(mkrepo.License)
		return nil
	case "readme":
		fmt.Print(mkrepo.Readme)
		return nil
	case "server":
		serverCmd.Parse(args[2:])
		return runServer(ctx, serverConfigFile)
	default:
		return fmt.Errorf("unknown command: %s", args[1])
	}
}

func runServer(ctx context.Context, serverConfigFile *string) error {
	logger := log.SetupLogger()
	logger.InfoContext(ctx, "Build info",
		slog.String("version", internal.Build.Version), slog.String("goVersion", internal.Build.GoVersion),
		slog.String("revision", internal.Build.Revision), slog.Time("buildDatetime", internal.Build.BuildDatetime),
	)
	logger.InfoContext(ctx, "Runtime info", slog.Int("GOMAXPROCS", runtime.GOMAXPROCS(0)), slog.Int("numCPU", runtime.NumCPU()))

	cfg, err := config.LoadConfig(*serverConfigFile)
	if err != nil {
		logger.ErrorContext(ctx, "Cannot load config", log.Err(err))
		return ErrNoPrint
	}

	providers := provider.NewProvidersFromConfig(ctx, logger, cfg)

	db, err := database.New(ctx, logger, cfg.DatabaseUri, cfg.SecretKey)
	if err != nil {
		logger.ErrorContext(ctx, "Cannot open database", log.Err(err))
		return ErrNoPrint
	}
	defer db.Close()
	go db.GarbageCollector(ctx, 12*time.Hour)

	var gitignoresFS fs.FS = gitignore.FS
	if cfg.GitignoresDir != "" {
		gitignoresFS = os.DirFS(cfg.GitignoresDir)
	}
	gitignores, err := service.PrepareGitignores(gitignoresFS)
	if err != nil {
		logger.ErrorContext(ctx, "Cannot prepare gitignores", log.Err(err))
		return ErrNoPrint
	}

	var licensesFS fs.FS = license.FS
	if cfg.LicensesDir != "" {
		licensesFS = os.DirFS(cfg.LicensesDir)
	}
	licenses, err := service.ParseLicensesFromBytes(license.LicenseConfig, licensesFS)
	if err != nil {
		logger.ErrorContext(ctx, "Cannot prepare licenses", log.Err(err))
		return ErrNoPrint
	}

	var templatesFS fs.FS = template.FS
	if cfg.TemplatesDir != "" {
		templatesFS = os.DirFS(cfg.TemplatesDir)
	}
	err = service.PrepareTemplates(db, templatesFS)
	if err != nil {
		logger.ErrorContext(ctx, "Cannot prepare templates", log.Err(err))
		return ErrNoPrint
	}

	repomaker := service.NewService(db, gitignoresFS, licenses, templatesFS)

	srv := server.NewServer(logger, cfg, db, repomaker, providers, gitignores, licenses)

	errCh := make(chan error)
	go func() {
		logger.InfoContext(ctx, "Starting listening", slog.String("addr", srv.Addr))
		errCh <- srv.ListenAndServe() // TODO: Use TLS
		close(errCh)
	}()

	select {
	case err := <-errCh:
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.ErrorContext(ctx, "Cannot run server", log.Err(err))
			return ErrNoPrint
		}
	case <-ctx.Done():
		timeout := 15 * time.Second
		logger.InfoContext(ctx, "Shutting down server", slog.Duration("timeout", timeout))
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()
		err := srv.Shutdown(ctx)
		if err != nil {
			logger.ErrorContext(ctx, "Cannot gracefully shutdown server", log.Err(err))
			return ErrNoPrint
		}
	}

	return nil
}
