package handler

import (
	"fmt"
	"net/http"

	"github.com/FilipSolich/mkrepo/internal/db"
	"github.com/FilipSolich/mkrepo/internal/provider"
)

type Webhook struct {
	db        *db.DB
	providers provider.Providers
}

func NewWebhook(db *db.DB, providers provider.Providers) *Webhook {
	return &Webhook{
		db:        db,
		providers: providers,
	}
}

// TODO: Implement webhook handler for all providers
func (h *Webhook) Handle(w http.ResponseWriter, r *http.Request) {
	providerKey := r.PathValue("provider")
	provider, ok := h.providers[providerKey]
	if !ok {
		http.Error(w, "unsupported provider", http.StatusBadRequest)
		return
	}

	event, err := provider.ParseWebhookEvent(r)
	if err != nil {
		http.Error(w, "failed to parse webhook event", http.StatusBadRequest)
		return
	}
	fmt.Println("Webhook event:", event)

	// TODO: Handle logic: pull repo -> parse mkrepo.yaml -> update db (long term cache repo)
}
