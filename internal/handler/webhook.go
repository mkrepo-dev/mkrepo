package handler

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/mkrepo-dev/mkrepo/internal/db"
	"github.com/mkrepo-dev/mkrepo/internal/provider"
)

func Webhook(db *db.DB, providers provider.Providers) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		providerKey := r.PathValue("provider")
		prov, ok := providers[providerKey] // TODO: Validate providerKey in middreware and dont use value-ok pattern
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
	})
}
