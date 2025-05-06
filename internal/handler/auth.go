package handler

import (
	"crypto/rand"
	"html/template"
	"net/http"
	"time"

	"github.com/mkrepo-dev/mkrepo/internal/database"
	"github.com/mkrepo-dev/mkrepo/internal/handler/cookie"
	"github.com/mkrepo-dev/mkrepo/internal/provider"
	"github.com/mkrepo-dev/mkrepo/template/html"
)

func Login(db *database.DB, providers provider.Providers) http.HandlerFunc {
	type loginContext struct {
		baseContext
		Providers provider.Providers
	}
	tmpl := template.Must(template.ParseFS(html.FS, "base.html", "login.html"))
	return func(w http.ResponseWriter, r *http.Request) {
		provider, ok := providers[r.FormValue("provider")]
		if !ok {
			render(w, tmpl, loginContext{
				baseContext: getBaseContext(r),
				Providers:   providers,
			})
			return
		}

		state := rand.Text()
		err := db.CreateOAuth2State(r.Context(), state, time.Now().Add(15*time.Minute))
		if err != nil {
			internalServerError(w, "Failed to create state", err)
			return
		}

		http.Redirect(w, r, provider.OAuth2Config().AuthCodeURL(state), http.StatusFound)
	}
}

func Logout(db *database.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		sessionCookie, err := r.Cookie("session")
		if err != nil {
			http.Redirect(w, r, "/", http.StatusFound)
			return
		}

		err = db.DeleteSession(r.Context(), sessionCookie.Value)
		if err != nil {
			internalServerError(w, "Failed to delete session", err)
			return
		}

		http.SetCookie(w, cookie.NewDeleteCookie("session"))
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

		valid, err := db.ValidateOAuth2State(r.Context(), r.FormValue("state"))
		if err != nil || !valid {
			http.Error(w, "invalid state", http.StatusBadRequest)
			return
		}

		cfg := provider.OAuth2Config()
		token, err := cfg.Exchange(r.Context(), r.FormValue("code"))
		if err != nil {
			internalServerError(w, "Failed to exchange code for token", err)
			return
		}

		client := provider.NewClient(r.Context(), token)
		info, err := client.GetUser(r.Context())
		if err != nil {
			internalServerError(w, "Failed to get user info", err)
			return
		}

		session := rand.Text()
		sessionExpiresIn := 30 * 24 * 60 * 60
		sessionExpiresAt := time.Now().Add(time.Duration(sessionExpiresIn) * time.Second)
		err = db.CreateAccountSession(r.Context(), session, sessionExpiresAt, providerKey, client.Token(), info)
		if err != nil {
			internalServerError(w, "Failed to create account", err)
			return
		}

		http.SetCookie(w, cookie.NewCookie("session", session, sessionExpiresIn))

		redirectCookie, err := r.Cookie("redirect_uri")
		if err == nil {
			http.SetCookie(w, cookie.NewDeleteCookie("redirect_uri"))
			http.Redirect(w, r, redirectCookie.Value, http.StatusFound)
			return
		}

		http.Redirect(w, r, "/", http.StatusFound)
	}
}
