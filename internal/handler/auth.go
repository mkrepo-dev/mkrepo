package handler

import (
	"crypto/rand"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/FilipSolich/mkrepo/internal/log"
	"github.com/FilipSolich/mkrepo/internal/provider"
)

var stateLifetime = 15 * time.Minute

type Auth struct {
	providers provider.Providers
	states    map[string]time.Time
	statesMu  sync.Mutex
}

func NewAuth(providers provider.Providers) *Auth {
	handler := &Auth{providers: providers, states: make(map[string]time.Time)}
	go handler.stateCleaner(12 * time.Hour)
	return handler
}

func (h *Auth) LoginWithProvider(w http.ResponseWriter, r *http.Request) {
	provider, ok := h.providers[r.FormValue("provider")]
	if !ok {
		http.Error(w, "unsupported provider", http.StatusBadRequest)
		return
	}

	url := provider.OAuth2Config().AuthCodeURL(h.createState())
	http.Redirect(w, r, url, http.StatusFound)
}

func (h *Auth) Oauth2Callback(w http.ResponseWriter, r *http.Request) {
	provider, ok := h.providers[r.PathValue("provider")]
	if !ok {
		http.Error(w, "unsupported provider", http.StatusBadRequest)
		return
	}

	code := r.FormValue("code")
	if code == "" {
		http.Error(w, "code is required", http.StatusBadRequest)
		return
	}
	state := r.FormValue("state")
	if state == "" || !h.validateState(state) {
		http.Error(w, "invalid state", http.StatusBadRequest)
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

	// TODO: Redirect from where the user came
}

func (h *Auth) createState() string {
	h.statesMu.Lock()
	defer h.statesMu.Unlock()
	state := rand.Text()
	h.states[state] = time.Now()
	return state
}

func (h *Auth) validateState(state string) bool {
	h.statesMu.Lock()
	defer h.statesMu.Unlock()
	if t, ok := h.states[state]; ok {
		delete(h.states, state)
		if time.Since(t) < stateLifetime {
			return true
		}
	}
	return false
}

func (h *Auth) cleanExpiredState() {
	h.statesMu.Lock()
	defer h.statesMu.Unlock()
	for state, t := range h.states {
		if time.Since(t) > stateLifetime {
			delete(h.states, state)
		}
	}
}

func (h *Auth) stateCleaner(interval time.Duration) {
	for range time.Tick(interval) {
		h.cleanExpiredState()
	}
}
