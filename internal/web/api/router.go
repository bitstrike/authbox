package api

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
)

func RegisterRoutes(r chi.Router) {
	r.Route("/api/v1", func(r chi.Router) {
		r.Get("/ssh/ca.pub", getCAPublicKey)
		// Remaining endpoints require auth - added in Phase 3
	})

	r.Route("/internal", func(r chi.Router) {
		// Internal sync endpoints - added in Phase 10
	})
}

func getCAPublicKey(w http.ResponseWriter, r *http.Request) {
	// TODO: return CA public key from ca package
	w.Header().Set("Content-Type", "text/plain")
	w.WriteHeader(http.StatusNotImplemented)
	w.Write([]byte("not yet initialized"))
}

func respondJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func respondError(w http.ResponseWriter, status int, code string, message string) {
	respondJSON(w, status, map[string]any{
		"error": map[string]string{
			"code":    code,
			"message": message,
		},
	})
}
