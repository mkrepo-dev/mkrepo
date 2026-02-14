package handler

import (
	"encoding/json"
	"net/http"

	"github.com/mkrepo-dev/mkrepo/internal/app"
)

func Live(healthService *app.HealthService) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		live := healthService.Live()
		code := http.StatusOK
		if !live {
			code = http.StatusServiceUnavailable
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(code)
		json.NewEncoder(w).Encode(struct { // nolint:errcheck
			Live bool `json:"live"`
		}{
			Live: live,
		})
	})
}

func Ready(healthService *app.HealthService) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		status := healthService.Ready(r.Context())
		code := http.StatusOK
		if !status.Repository {
			code = http.StatusServiceUnavailable
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(code)
		json.NewEncoder(w).Encode(struct { // nolint:errcheck
			Repository bool `json:"repository"`
		}{
			Repository: status.Repository,
		})
	})
}
