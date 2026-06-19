package handlers

import (
	"context"
	"database/sql"
	"net/http"
	"strconv"

	"raggo/internal/database"
	"raggo/internal/middleware"
	"raggo/internal/services/search"
)

type SearchHandler struct {
	SearchSvc *search.Service
	DB        *sql.DB
	CollSvc   interface {
		GetOrCreateDefault(ctx context.Context, username string) (*database.Collection, error)
	}
}

func (h *SearchHandler) Search(w http.ResponseWriter, r *http.Request) {
	c := middleware.ClaimsFromCtx(r.Context())

	query := r.URL.Query().Get("q")
	if query == "" {
		writeJSON(w, http.StatusBadRequest, errResp("query required"))
		return
	}
	limit := 10
	if l := r.URL.Query().Get("limit"); l != "" {
		if n, err := strconv.Atoi(l); err == nil && n > 0 && n <= 100 {
			limit = n
		}
	}
	collectionSlug := r.URL.Query().Get("collection_slug")
	useHybrid := true

	// resolve collection
	qdrantName := ""
	if collectionSlug != "" {
		col, err := database.GetCollection(h.DB, c.Username, collectionSlug)
		if err != nil {
			writeJSON(w, http.StatusNotFound, errResp("collection not found"))
			return
		}
		qdrantName = col.QdrantName
	} else {
		col, err := h.CollSvc.GetOrCreateDefault(r.Context(), c.Username)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, errResp("collection error"))
			return
		}
		qdrantName = col.QdrantName
	}

	results, err := h.SearchSvc.Search(r.Context(), query, qdrantName, limit, useHybrid)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errResp(err.Error()))
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"data":  results,
		"query": query,
		"total": len(results),
	})
}
