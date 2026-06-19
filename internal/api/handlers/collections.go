package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"raggo/internal/middleware"
	"raggo/internal/services/collection"
)

type CollectionHandler struct{ Svc *collection.Service }

func (h *CollectionHandler) List(w http.ResponseWriter, r *http.Request) {
	c := middleware.ClaimsFromCtx(r.Context())
	cols, err := h.Svc.List(c.Username)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errResp("failed to list collections"))
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": cols, "total": len(cols)})
}

func (h *CollectionHandler) ListAll(w http.ResponseWriter, r *http.Request) {
	cols, err := h.Svc.ListAll()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errResp("failed to list all collections"))
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": cols, "total": len(cols)})
}

func (h *CollectionHandler) Create(w http.ResponseWriter, r *http.Request) {
	c := middleware.ClaimsFromCtx(r.Context())
	var req struct {
		DisplayName string `json:"display_name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.DisplayName == "" {
		writeJSON(w, http.StatusBadRequest, errResp("display_name required"))
		return
	}
	col, err := h.Svc.Create(r.Context(), c.Username, req.DisplayName)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errResp(err.Error()))
		return
	}
	writeJSON(w, http.StatusCreated, col)
}

func (h *CollectionHandler) Delete(w http.ResponseWriter, r *http.Request) {
	c := middleware.ClaimsFromCtx(r.Context())
	slug := chi.URLParam(r, "slug")
	if err := h.Svc.Delete(r.Context(), c.Username, slug); err != nil {
		writeJSON(w, http.StatusBadRequest, errResp(err.Error()))
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
