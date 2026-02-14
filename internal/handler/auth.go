package handler

import (
	"html/template"
	"io/fs"
	"log/slog"
	"net/http"
	"time"

	"github.com/mkrepo-dev/mkrepo/internal/app"
	"github.com/mkrepo-dev/mkrepo/internal/provider"
)

const (
	sessionCookieName  = "session"
	redirectCookieName = "redirect_uri" // TODO: This is not set anywhere
)

func baseCookie(name string) *http.Cookie {
	return &http.Cookie{
		Name:     name,
		Path:     "/",
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteLaxMode,
	}
}

func Authenticate(logger *slog.Logger, authService *app.AuthService) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()
			sessionCookie, err := r.Cookie(sessionCookieName)
			if err == nil {
				account, err := authService.Authenticate(ctx, sessionCookie.Value)
				if err != nil {
					Logout(logger, authService)(w, r)
					return
				}
				ctx = app.ContextWithAccount(ctx, account)
			}
			next.ServeHTTP(w, r.WithContext(ctx))
		})
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

		authURL, err := authService.GetAuthURL(r.Context(), app.ProviderKey(providerKey))
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
		sessionCookie, err := r.Cookie(sessionCookieName)
		if err != nil {
			http.Redirect(w, r, "/", http.StatusFound)
			return
		}

		err = authService.Logout(r.Context(), sessionCookie.Value)
		if err != nil {
			logger.Error("Failed to logout user.", "err", err)
		}

		cookie := baseCookie(sessionCookieName)
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
			http.Error(w, "Missing state.", http.StatusBadRequest)
			return
		}
		code := r.FormValue("code")
		if code == "" {
			http.Error(w, "Missing code.", http.StatusBadRequest)
			return
		}
		providerKey := r.PathValue("provider")
		if providerKey == "" {
			http.Error(w, "Missing provider.", http.StatusBadRequest)
			return
		}

		session, err := authService.LoginWithOAuth2Callback(r.Context(), app.ProviderKey(providerKey), state, code)
		if err != nil {
			logger.Error("Failed to login with OAuth2 callback.", "err", err)
			http.Error(w, "Failed to login.", http.StatusInternalServerError)
			return
		}

		cookie := baseCookie(sessionCookieName)
		cookie.Value = session.ID
		cookie.MaxAge = int(time.Until(session.ExpiresAt).Seconds())
		http.SetCookie(w, cookie)

		redirectCookie, err := r.Cookie(redirectCookieName)
		if err == nil {
			redirectURL := redirectCookie.Value
			redirectCookie = baseCookie(redirectCookieName)
			redirectCookie.Value = ""
			redirectCookie.MaxAge = -1
			http.SetCookie(w, redirectCookie)
			http.Redirect(w, r, redirectURL, http.StatusFound)
			return
		}

		http.Redirect(w, r, "/", http.StatusFound)
	}
}
