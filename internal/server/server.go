package server

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/mkrepo-dev/mkrepo/internal/adapter"
	"github.com/mkrepo-dev/mkrepo/internal/app"
	"github.com/mkrepo-dev/mkrepo/internal/config"
	"github.com/mkrepo-dev/mkrepo/internal/handler"
	"github.com/mkrepo-dev/mkrepo/internal/handler/middleware"
	"github.com/mkrepo-dev/mkrepo/internal/metrics"
	"github.com/mkrepo-dev/mkrepo/internal/provider"
	mkrepo "github.com/mkrepo-dev/mkrepo/internal/service"
	"github.com/mkrepo-dev/mkrepo/static"
	"github.com/mkrepo-dev/mkrepo/template/html"
)

func NewServer(
	logger *slog.Logger,
	cfg config.Config,
	reg *prometheus.Registry,
	metrics *metrics.Metrics,
	db *adapter.Repository,
	authService *app.AuthService,
	repomaker *mkrepo.MkrepoService,
	providers provider.Providers,
	gitignores []string,
	licenses mkrepo.Licenses,
) *http.Server {
	mux := http.NewServeMux()

	mux.Handle("GET /", handler.Index(providers))
	mux.Handle("GET /static/", http.StripPrefix("/static/", http.FileServer(http.FS(static.FS))))
	mux.Handle("GET /metrics", middleware.MetricsAuth(cfg.MetricsToken)(promhttp.HandlerFor(reg, promhttp.HandlerOpts{
		Registry: reg,
	})))

	mux.HandleFunc("GET /auth/login", handler.Login(logger, html.FS, authService, providers))
	mux.HandleFunc("GET /auth/logout", handler.Logout(logger, authService))
	mux.HandleFunc("GET /auth/oauth2/callback/{provider}", handler.OAuth2Callback(logger, authService))

	mux.Handle("GET /new", middleware.MustAuthenticate(handler.MkrepoForm(db, providers, gitignores, licenses)))
	mux.Handle("POST /new", middleware.MustAuthenticate(handler.MkrepoCreate(db, repomaker, providers)))

	mux.Handle("GET /schemas", handler.Schemas(licenses))

	mux.Handle("GET /templates", handler.Templates(db))
	mux.Handle("POST /templates", middleware.MustAuthenticate(handler.RegisterTemplate(repomaker)))

	handler := middleware.Metrics(metrics)(mux)
	handler = middleware.Authenticate(logger, authService)(handler)

	server := &http.Server{
		Addr:         ":8080",
		Handler:      handler,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 60 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	return server
}
