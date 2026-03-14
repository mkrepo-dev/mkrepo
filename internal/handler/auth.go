package handler

import (
	"html/template"
	"io/fs"
	"log/slog"
	"net/http"
	"net/url"
	"time"

	"github.com/mkrepo-dev/mkrepo/internal/app"
	"github.com/mkrepo-dev/mkrepo/internal/provider"
)

const SessionCookieName = "session"

func baseCookie() *http.Cookie {
	return &http.Cookie{
		Name:     SessionCookieName,
		Path:     "/",
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteLaxMode,
	}
}

func Login(logger *slog.Logger, fs fs.FS, authService *app.AuthService, providers provider.Providers) http.HandlerFunc {
	tmpl := template.Must(template.ParseFS(fs, "base.html", "login.html"))
	return func(w http.ResponseWriter, r *http.Request) {
		providerKey := r.FormValue("provider")
		if providerKey == "" {
			render(w, tmpl, struct {
				baseContext
				Providers provider.Providers
			}{
				baseContext: getBaseContext(r),
				Providers:   providers,
			})
		}
		redirectURL, _ := url.QueryUnescape(r.FormValue("redirect")) // nolint:errcheck

		authURL, err := authService.GetAuthURL(r.Context(), provider.ProviderKey(providerKey), redirectURL)
		if err != nil {
			logger.Error("Failed to get auth URL.", "err", err)
			http.Error(w, "Failed to get auth URL.", http.StatusInternalServerError)
			return
		}

		http.Redirect(w, r, authURL, http.StatusFound)
	}
}

func Logout(logger *slog.Logger, authService *app.AuthService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		sessionCookie, err := r.Cookie(SessionCookieName)
		if err != nil {
			http.Redirect(w, r, "/", http.StatusFound)
			return
		}

		err = authService.Logout(r.Context(), sessionCookie.Value)
		if err != nil {
			logger.Error("Failed to logout user.", "err", err)
		}

		cookie := baseCookie()
		cookie.Value = ""
		cookie.MaxAge = -1
		http.SetCookie(w, cookie)
		http.Redirect(w, r, "/", http.StatusFound)
	}
}

func OAuth2Callback(logger *slog.Logger, authService *app.AuthService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		errMsg := r.FormValue("error")
		if errMsg != "" {
			http.Error(w, "OAuth2 error: "+errMsg, http.StatusBadRequest)
			return
		}
		state := r.FormValue("state")
		if state == "" {
			http.Error(w, "Missing state", http.StatusBadRequest)
			return
		}
		code := r.FormValue("code")
		if code == "" {
			http.Error(w, "Missing code", http.StatusBadRequest)
			return
		}
		providerKey := r.PathValue("provider")
		if providerKey == "" {
			http.Error(w, "Missing provider", http.StatusBadRequest)
			return
		}

		session, err := authService.LoginWithOAuth2Callback(r.Context(), provider.ProviderKey(providerKey), state, code)
		if err != nil {
			logger.Error("Failed to login with OAuth2 callback.", "err", err)
			http.Error(w, "Failed to login.", http.StatusInternalServerError)
			return
		}

		cookie := baseCookie()
		cookie.Value = session.ID
		cookie.MaxAge = int(time.Until(session.ExpiresAt).Seconds())
		http.SetCookie(w, cookie)
		http.Redirect(w, r, "/", http.StatusFound)
	}
}
