package handler

import (
	"crypto/rand"
	"net/http"
	"time"

	"github.com/FilipSolich/mkrepo/internal/db"
	"github.com/FilipSolich/mkrepo/internal/middleware"
	"github.com/FilipSolich/mkrepo/internal/provider"
	"github.com/FilipSolich/mkrepo/template"
)

var stateLifetime = 15 * time.Minute

type Auth struct {
	db        *db.DB
	providers provider.Providers
}

func NewAuth(db *db.DB, providers provider.Providers) *Auth {
	return &Auth{db: db, providers: providers}
}

func (h *Auth) Login(w http.ResponseWriter, r *http.Request) {
	provider, ok := h.providers[r.FormValue("provider")]
	if !ok {
		// TODO: Perserve redirect uri
		template.Render(w, template.Login, template.LoginContext{
			BaseContext: getBaseContext(r),
			Providers:   h.providers,
		})
		return
	}

	config := provider.OAuth2Config(r.FormValue("redirect_uri"))

	state := rand.Text()
	err := h.db.CreateOAuth2State(r.Context(), state, time.Now().Add(stateLifetime))
	if err != nil {
		internalServerError(w, "Failed to create state", err)
		return
	}

	http.Redirect(w, r, config.AuthCodeURL(state), http.StatusFound)
}

func (h *Auth) Logout(w http.ResponseWriter, r *http.Request) {
	provider, username := splitProviderUser(r)
	err := h.db.DeleteAccount(r.Context(), middleware.Session(r.Context()), provider, username)
	if err != nil {
		internalServerError(w, "Failed to delete account", err)
		return
	}
	http.Redirect(w, r, "/", http.StatusFound)
}

func (h *Auth) OAuth2Callback(w http.ResponseWriter, r *http.Request) {
	providerKey := r.PathValue("provider")
	provider, ok := h.providers[providerKey]
	if !ok {
		http.Error(w, "unsupported provider", http.StatusBadRequest)
		return
	}

	code := r.FormValue("code")
	if code == "" {
		http.Error(w, "code is required", http.StatusBadRequest)
		return
	}

	err := h.db.ValidateAndDeleteOAuth2State(r.Context(), r.FormValue("state"))
	if err != nil {
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

	client, token := provider.NewClient(r.Context(), token, cfg.RedirectURL)
	info, err := client.GetUserInfo(r.Context())
	if err != nil {
		internalServerError(w, "Failed to get user info", err)
		return
	}
	err = h.db.CreateOrOverwriteAccount(r.Context(), session, providerKey, token, cfg.RedirectURL, info)
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
