package handlers

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"

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
	var req struct {
		Query          string `json:"query"`
		Limit          int    `json:"limit"`
		CollectionSlug string `json:"collection_slug"`
		UseHybrid      *bool  `json:"use_hybrid"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Query == "" {
		writeJSON(w, http.StatusBadRequest, errResp("query required"))
		return
	}
	if req.Limit <= 0 || req.Limit > 100 {
		req.Limit = 10
	}
	useHybrid := true
	if req.UseHybrid != nil {
		useHybrid = *req.UseHybrid
	}

	// resolve collection
	qdrantName := ""
	if req.CollectionSlug != "" {
		col, err := database.GetCollection(h.DB, c.Username, req.CollectionSlug)
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

	results, err := h.SearchSvc.Search(r.Context(), req.Query, qdrantName, req.Limit, useHybrid)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errResp(err.Error()))
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"data":  results,
		"query": req.Query,
		"total": len(results),
	})
}
