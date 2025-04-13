package handler

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/mkrepo-dev/mkrepo/internal/db"
	"github.com/mkrepo-dev/mkrepo/internal/provider"
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
	prov, ok := h.providers[providerKey]
	if !ok {
		http.Error(w, "unsupported provider", http.StatusBadRequest)
		return
	}

	event, err := prov.ParseWebhookEvent(r)
	if err != nil {
		if errors.Is(err, provider.ErrIgnoreEvent) {
			return
		}
		http.Error(w, "failed to parse webhook event", http.StatusBadRequest)
		return
	}

	fmt.Println("Webhook event:", event) // TODO: Remove

	// TODO: Handle logic: pull repo -> parse mkrepo.yaml -> update db (long term cache repo)
}
