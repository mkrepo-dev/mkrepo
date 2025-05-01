package server

import (
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/mkrepo-dev/mkrepo/internal/database"
	"github.com/mkrepo-dev/mkrepo/internal/handler"
	"github.com/mkrepo-dev/mkrepo/internal/middleware"
	"github.com/mkrepo-dev/mkrepo/internal/mkrepo"
	"github.com/mkrepo-dev/mkrepo/internal/provider"
	"github.com/mkrepo-dev/mkrepo/static"
	"github.com/mkrepo-dev/mkrepo/template"
)

func NewServer(db *database.DB, repomaker *mkrepo.RepoMaker, providers provider.Providers, licenses template.Licenses) *http.Server {
	mux := http.NewServeMux()

	mux.Handle("GET /", handler.Index(providers))
	mux.Handle("GET /static/", http.StripPrefix("/static/", http.FileServer(http.FS(static.FS))))
	mux.Handle("GET /metrics", promhttp.Handler())

	auth := handler.NewAuth(db, providers)
	mux.HandleFunc("GET /auth/login", auth.Login)
	mux.HandleFunc("GET /auth/logout", auth.Logout)
	mux.HandleFunc("GET /auth/oauth2/callback/{provider}", auth.OAuth2Callback)

	mux.Handle("GET /new", handler.MkrepoForm(db, providers, licenses))
	mux.Handle("POST /new", handler.MkrepoCreate(db, repomaker, providers, licenses))

	mux.Handle("GET /templates", handler.Templates(db))

	mux.Handle("POST /webhook/{provider}", handler.Webhook(db, providers))

	handler := middleware.NewAuthenticate(db)(mux)

	server := &http.Server{
		Addr:         ":8080",
		Handler:      handler,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 60 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	return server
}
