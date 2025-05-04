package handler

import (
	"crypto/rand"
	"html/template"
	"net/http"
	"time"

	"github.com/mkrepo-dev/mkrepo/internal/database"
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
			// TODO: Perserve redirect uri
			render(w, tmpl, loginContext{
				baseContext: getBaseContext(r),
				Providers:   providers,
			})
			return
		}

		// TODO: Set redirect_uri to cookie and use it in the callback
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
		cookie, err := r.Cookie("session")
		if err != nil {
			http.Redirect(w, r, "/", http.StatusFound)
			return
		}

		err = db.DeleteSession(r.Context(), cookie.Value)
		if err != nil {
			internalServerError(w, "Failed to delete session", err)
			return
		}

		cookie.MaxAge = -1
		http.SetCookie(w, cookie)
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

		_, expiresAt, err := db.GetAndDeleteOAuth2State(r.Context(), r.FormValue("state"))
		if err != nil || expiresAt.Before(time.Now()) {
			http.Error(w, "invalid state", http.StatusBadRequest)
			return
		}

		cfg := provider.OAuth2Config(r.FormValue("redirect_uri"))
		token, err := cfg.Exchange(r.Context(), r.FormValue("code"))
		if err != nil {
			internalServerError(w, "Failed to exchange code for token", err)
			return
		}

		client := provider.NewClient(r.Context(), token, cfg.RedirectURL)
		info, err := client.GetUser(r.Context())
		if err != nil {
			internalServerError(w, "Failed to get user info", err)
			return
		}

		session := rand.Text()
		sessionExpiresIn := 30 * 24 * 60 * 60
		sessionExpiresAt := time.Now().Add(time.Duration(sessionExpiresIn) * time.Second)
		err = db.CreateAccountSession(r.Context(), session, sessionExpiresAt, providerKey, client.Token(), cfg.RedirectURL, info)
		if err != nil {
			internalServerError(w, "Failed to create account", err)
			return
		}

		sessionCookie := &http.Cookie{
			Name:     "session",
			Value:    session,
			Path:     "/",
			MaxAge:   sessionExpiresIn,
			HttpOnly: true,
			Secure:   true,
			SameSite: http.SameSiteLaxMode,
		}
		http.SetCookie(w, sessionCookie)

		redirectCookie, err := r.Cookie("redirecturi")
		if err == nil {
			//cookie := &http.Cookie{
			//	Name:     "redirect_uri",
			//	Value:    "",
			//	Path:     "/",
			//	MaxAge:   -1,
			//	HttpOnly: true,
			//	Secure:   true,
			//	SameSite: http.SameSiteLaxMode,
			//}
			//redirect := redirectCookie.Value
			v := redirectCookie.Value
			redirectCookie.MaxAge = -1
			//redirectCookie.Path = "/"
			//redirectCookie.Value = ""
			http.SetCookie(w, redirectCookie)
			http.Redirect(w, r, v, http.StatusFound)
			return
		}

		//http.Redirect(w, r, r.FormValue("redirect_uri"), http.StatusFound)
		http.Redirect(w, r, "/", http.StatusFound)
	}
}
