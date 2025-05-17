package server

import (
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/mkrepo-dev/mkrepo/internal/config"
	"github.com/mkrepo-dev/mkrepo/internal/database"
	"github.com/mkrepo-dev/mkrepo/internal/handler"
	"github.com/mkrepo-dev/mkrepo/internal/handler/middleware"
	"github.com/mkrepo-dev/mkrepo/internal/metrics"
	"github.com/mkrepo-dev/mkrepo/internal/mkrepo"
	"github.com/mkrepo-dev/mkrepo/internal/provider"
	"github.com/mkrepo-dev/mkrepo/static"
)

func NewServer(
	cfg config.Config,
	reg *prometheus.Registry,
	metrics *metrics.Metrics,
	db *database.DB,
	repomaker *mkrepo.RepoMaker,
	providers provider.Providers,
	gitignores []string,
	licenses mkrepo.Licenses,
	dockerfiles mkrepo.Dockerfiles,
) *http.Server {
	mux := http.NewServeMux()

	mux.Handle("GET /", handler.Index(providers))
	mux.Handle("GET /static/", http.StripPrefix("/static/", http.FileServer(http.FS(static.FS))))
	mux.Handle("GET /livez", handler.Healthz())
	mux.Handle("GET /readyz", handler.Readyz(db.DB))
	mux.Handle("GET /metrics", middleware.MetricsAuth(cfg.MetricsToken)(promhttp.HandlerFor(reg, promhttp.HandlerOpts{
		Registry: reg,
	})))

	mux.HandleFunc("GET /auth/login", handler.Login(db, providers))
	mux.HandleFunc("GET /auth/logout", handler.Logout(db))
	mux.HandleFunc("GET /auth/oauth2/callback/{provider}", handler.OAuth2Callback(db, providers))

	mux.Handle("GET /new", middleware.MustAuthenticate(handler.MkrepoForm(db, providers, gitignores, licenses, dockerfiles)))
	mux.Handle("POST /new", middleware.MustAuthenticate(handler.MkrepoCreate(db, repomaker, providers)))

	mux.Handle("GET /templates", handler.Templates(db)) // TODO: Remove this

	mux.Handle("POST /webhook/{provider}", handler.Webhook(db, providers))

	handler := middleware.Metrics(metrics)(mux)
	handler = middleware.Authenticate(db)(handler)

	server := &http.Server{
		Addr:         ":8080",
		Handler:      handler,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 60 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	return server
}
