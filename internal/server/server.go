package server

import (
	"log/slog"
	"net"
	"net/http"
	"strconv"
	"time"

	"github.com/mkrepo-dev/mkrepo/internal/config"
	"github.com/mkrepo-dev/mkrepo/internal/database"
	"github.com/mkrepo-dev/mkrepo/internal/handler"
	"github.com/mkrepo-dev/mkrepo/internal/log"
	"github.com/mkrepo-dev/mkrepo/internal/provider"
	mkrepo "github.com/mkrepo-dev/mkrepo/internal/service"
	"github.com/mkrepo-dev/mkrepo/static"
	"github.com/mkrepo-dev/mkrepo/template/html"
)

func NewServer(
	logger *slog.Logger,
	cfg config.Config,
	db *database.DB,
	repomaker *mkrepo.MkrepoService,
	providers provider.Providers,
	gitignores []string,
	licenses mkrepo.Licenses,
) *http.Server {
	logger = log.Component(logger, "server")

	mux := http.NewServeMux()

	mux.Handle("GET /", handler.Index(logger, providers))
	mux.Handle("GET /static/", http.StripPrefix("/static/", http.FileServer(http.FS(static.FS))))

	mux.HandleFunc("GET /auth/login", handler.Login(logger, html.FS, db, providers))
	mux.HandleFunc("GET /auth/logout", handler.Logout(logger, db))
	mux.HandleFunc("GET /auth/oauth2/callback/{provider}", handler.OAuth2Callback(logger, db, providers))
	mux.Handle("GET /auth/delete-account", handler.MustAuthenticate(handler.DeleteAccount(logger, db)))

	mux.Handle("GET /new", handler.MustAuthenticate(handler.MkrepoForm(logger, db, providers, gitignores, licenses)))
	mux.Handle("POST /new", handler.MustAuthenticate(handler.MkrepoCreate(logger, db, repomaker, providers)))

	mux.Handle("GET /schemas", handler.Schemas(licenses))

	mux.Handle("GET /templates", handler.Templates(logger, db))
	mux.Handle("POST /templates", handler.MustAuthenticate(handler.RegisterTemplate(repomaker)))

	handler := handler.AuthenticateMiddleware(logger, db, providers)(mux)

	server := &http.Server{
		Addr:         net.JoinHostPort(cfg.Addr, strconv.Itoa(cfg.Port)),
		Handler:      handler,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 60 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	return server
}
