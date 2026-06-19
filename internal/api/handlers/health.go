package handlers

import (
	"net/http"

	"raggo/internal/config"
)

func Health(cfg *config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{
			"status":  "healthy",
			"service": "RAGgo",
			"version": "0.1.0",
		})
	}
}
