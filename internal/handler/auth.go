package handler

import (
	"crypto/rand"
	"net/http"
	"time"

	"github.com/mkrepo-dev/mkrepo/internal/database"
	"github.com/mkrepo-dev/mkrepo/internal/middleware"
	"github.com/mkrepo-dev/mkrepo/internal/provider"
	"github.com/mkrepo-dev/mkrepo/template"
)

func Login(db *database.DB, providers provider.Providers) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		provider, ok := providers[r.FormValue("provider")]
		if !ok {
			// TODO: Perserve redirect uri
			template.Render(w, template.Login, template.LoginContext{
				BaseContext: getBaseContext(r),
				Providers:   providers,
			})
			return
		}

		config := provider.OAuth2Config(r.FormValue("redirect_uri"))

		state := rand.Text()
		err := db.CreateOAuth2State(r.Context(), state, time.Now().Add(15*time.Minute))
		if err != nil {
			internalServerError(w, "Failed to create state", err)
			return
		}

		http.Redirect(w, r, config.AuthCodeURL(state), http.StatusFound)
	}
}

func Logout(db *database.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		provider, username := splitProviderUser(r)
		err := db.DeleteAccount(r.Context(), middleware.Session(r.Context()), provider, username)
		if err != nil {
			internalServerError(w, "Failed to delete account", err)
			return
		}
		http.Redirect(w, r, "/", http.StatusFound)
	}
}

func OAuth2Callback(db *database.DB, providers provider.Providers) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		providerKey := r.PathValue("provider")
		provider, ok := providers[providerKey]
		if !ok {
			http.Error(w, "unsupported provider", http.StatusBadRequest)
			return
		}

		code := r.FormValue("code")
		if code == "" {
			http.Error(w, "code is required", http.StatusBadRequest)
			return
		}

		_, expiresAt, err := db.GetAndDeleteOAuth2State(r.Context(), r.FormValue("state"))
		if err != nil || expiresAt.Before(time.Now()) {
			http.Error(w, "invalid state", http.StatusBadRequest)
			return
		}

		cfg := provider.OAuth2Config(r.FormValue("redirect_uri"))
		token, err := cfg.Exchange(r.Context(), code)
		if err != nil {
			internalServerError(w, "Failed to exchange code for token", err)
			return
		}

		session := middleware.Session(r.Context())
		if session == "" {
			session = rand.Text() // TODO: Is 128 bit of randomness enough?
		}

		client := provider.NewClient(r.Context(), token, cfg.RedirectURL)
		info, err := client.GetUser(r.Context())
		if err != nil {
			internalServerError(w, "Failed to get user info", err)
			return
		}
		err = db.CreateOrOverwriteAccount(r.Context(), session, providerKey, client.Token(), cfg.RedirectURL, info)
		if err != nil {
			internalServerError(w, "Failed to create account", err)
			return
		}

		cookie := &http.Cookie{
			Name: "session", Value: session, Path: "/", MaxAge: 30 * 24 * 60 * 60,
			HttpOnly: true, Secure: true, SameSite: http.SameSiteLaxMode,
		}
		http.SetCookie(w, cookie)

		http.Redirect(w, r, r.FormValue("redirect_uri"), http.StatusFound)
	}
}
