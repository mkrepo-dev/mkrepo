package handler

import (
	"log/slog"
	"net/http"

	"github.com/FilipSolich/mkrepo/internal/log"
	"github.com/FilipSolich/mkrepo/internal/provider"
)

type Login struct {
	providers provider.Providers
}

func NewLogin(providers provider.Providers) *Login {
	return &Login{providers: providers}
}

func (h *Login) LoginProvider(w http.ResponseWriter, r *http.Request) {
	providerKey := r.FormValue("provider")
	if providerKey == "" {
		http.Error(w, "provider is required", http.StatusBadRequest)
		return
	}

	provider, ok := h.providers[providerKey]
	if !ok {
		http.Error(w, "unsupported provider", http.StatusBadRequest)
		return
	}
	url := provider.OAuth2Config().AuthCodeURL("")
	http.Redirect(w, r, url, http.StatusFound)
}

func (h *Login) Oauth2Callback(w http.ResponseWriter, r *http.Request) {
	providerKey := r.PathValue("provider")
	if providerKey == "" {
		http.Error(w, "provider is required", http.StatusBadRequest)
		return
	}

	code := r.FormValue("code")
	if code == "" {
		http.Error(w, "code is required", http.StatusBadRequest)
		return
	}

	provider, ok := h.providers[providerKey]
	if !ok {
		http.Error(w, "unsupported provider", http.StatusBadRequest)
		return
	}
	token, err := provider.OAuth2Config().Exchange(r.Context(), code)
	if err != nil {
		slog.Error("Failed to exchange code for token", log.Err(err))
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	cookie := &http.Cookie{
		Name:     "session",
		Value:    token.AccessToken,
		Path:     "/",
		MaxAge:   30 * 24 * 60 * 60,
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteStrictMode,
	}
	http.SetCookie(w, cookie)
}
