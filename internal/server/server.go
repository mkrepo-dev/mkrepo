package server

import (
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/mkrepo-dev/mkrepo/internal/config"
	"github.com/mkrepo-dev/mkrepo/internal/database"
	"github.com/mkrepo-dev/mkrepo/internal/handler"
	"github.com/mkrepo-dev/mkrepo/internal/handler/middleware"
	"github.com/mkrepo-dev/mkrepo/internal/mkrepo"
	"github.com/mkrepo-dev/mkrepo/internal/provider"
	"github.com/mkrepo-dev/mkrepo/static"
)

func NewServer(cfg config.Config, db *database.DB, repomaker *mkrepo.RepoMaker, providers provider.Providers, licenses mkrepo.Licenses) *http.Server {
	mux := http.NewServeMux()

	mux.Handle("GET /", handler.Index(providers))
	mux.Handle("GET /static/", http.StripPrefix("/static/", http.FileServer(http.FS(static.FS))))
	mux.Handle("GET /metrics", middleware.MetricsAuth(cfg.MetricsToken)(promhttp.Handler()))

	mux.HandleFunc("GET /auth/login", handler.Login(db, providers))
	mux.HandleFunc("GET /auth/logout", handler.Logout(db))
	mux.HandleFunc("GET /auth/oauth2/callback/{provider}", handler.OAuth2Callback(db, providers))

	mux.Handle("GET /new", middleware.MustAuthenticate(handler.MkrepoForm(db, providers, licenses)))
	mux.Handle("POST /new", middleware.MustAuthenticate(handler.MkrepoCreate(db, repomaker, providers, licenses)))

	mux.Handle("GET /templates", handler.Templates(db)) // TODO: Remove this

	mux.Handle("POST /webhook/{provider}", handler.Webhook(db, providers))

	handler := middleware.Authenticate(db)(mux)

	server := &http.Server{
		Addr:         ":8080",
		Handler:      handler,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 60 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	return server
}
