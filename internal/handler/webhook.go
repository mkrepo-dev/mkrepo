package handler

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/google/go-github/v69/github"

	"github.com/FilipSolich/mkrepo/internal/db"
)

type Webhook struct {
	db *db.DB
}

func NewWebhook(db *db.DB) *Webhook {
	return &Webhook{db: db}
}

// TODO: Implement webhook handler for all providers
func (h *Webhook) Handle(w http.ResponseWriter, r *http.Request) {
	payload, err := github.ValidatePayload(r, nil)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	event, err := github.ParseWebHook(github.WebHookType(r), payload)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	switch event := event.(type) {
	case *github.PushEvent:
		tag, ok := strings.CutPrefix(event.GetRef(), "refs/tags/")
		if ok {
			fmt.Println(tag)
		}
	}
}
