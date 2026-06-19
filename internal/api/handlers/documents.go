package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"raggo/internal/database"
	"raggo/internal/middleware"
	"raggo/internal/services/collection"
	"raggo/internal/services/document"
)

// DocumentHandler handles document upload, listing, deletion, and moving.
type DocumentHandler struct {
	IngestSvc *document.IngestService
	CollSvc   *collection.Service
	MaxUpload int64
}

// Upload handles multipart file upload, resolves the target collection,
// runs the ingest pipeline, and returns the resulting document metadata.
func (h *DocumentHandler) Upload(w http.ResponseWriter, r *http.Request) {
	c := middleware.ClaimsFromCtx(r.Context())
	if err := r.ParseMultipartForm(h.MaxUpload); err != nil {
		writeJSON(w, http.StatusBadRequest, errResp("file too large or invalid form"))
		return
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		writeJSON(w, http.StatusBadRequest, errResp("file field required"))
		return
	}
	defer file.Close()

	data := make([]byte, header.Size)
	if _, err := file.Read(data); err != nil {
		writeJSON(w, http.StatusInternalServerError, errResp("read error"))
		return
	}

	// Resolve collection: use ?collection_slug if provided, else get/create default.
	var col *database.Collection
	if slug := r.URL.Query().Get("collection_slug"); slug != "" {
		cols, listErr := h.CollSvc.List(c.Username)
		if listErr == nil {
			for i := range cols {
				if cols[i].Slug == slug {
					col = &cols[i]
					break
				}
			}
		}
		if col == nil {
			writeJSON(w, http.StatusNotFound, errResp("collection not found"))
			return
		}
	}
	if col == nil {
		col, err = h.CollSvc.GetOrCreateDefault(r.Context(), c.Username)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, errResp("failed to resolve collection"))
			return
		}
	}

	meta, err := h.IngestSvc.Ingest(r.Context(), c.Username, col.ID, col.QdrantName, header.Filename, data)
	if err != nil {
		writeJSON(w, http.StatusUnprocessableEntity, errResp(err.Error()))
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": meta, "message": "Document uploaded and processed successfully"})
}

// List returns a paginated list of documents owned by the authenticated user.
func (h *DocumentHandler) List(w http.ResponseWriter, r *http.Request) {
	c := middleware.ClaimsFromCtx(r.Context())
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}
	pageSize, _ := strconv.Atoi(r.URL.Query().Get("page_size"))
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	docs, total, err := database.ListDocumentsForUser(h.IngestSvc.DB, c.Username, page, pageSize)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errResp("list error"))
		return
	}
	totalPages := (total + pageSize - 1) / pageSize
	writeJSON(w, http.StatusOK, map[string]any{
		"data":  docs,
		"total": total,
		"meta":  map[string]any{"page": page, "page_size": pageSize, "total_pages": totalPages},
	})
}

// Delete removes a document and its vectors. Only the owner or an admin may delete.
func (h *DocumentHandler) Delete(w http.ResponseWriter, r *http.Request) {
	c := middleware.ClaimsFromCtx(r.Context())
	docID := chi.URLParam(r, "docID")

	meta, err := database.GetDocumentMeta(h.IngestSvc.DB, docID)
	if err != nil {
		writeJSON(w, http.StatusNotFound, errResp("document not found"))
		return
	}
	if meta.OwnerUsername != c.Username && c.Role != "admin" {
		writeJSON(w, http.StatusForbidden, errResp("forbidden"))
		return
	}
	col, err := database.GetCollectionByID(h.IngestSvc.DB, meta.CollectionID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errResp("collection not found"))
		return
	}
	if err := h.IngestSvc.Delete(r.Context(), docID, col.QdrantName); err != nil {
		writeJSON(w, http.StatusInternalServerError, errResp(err.Error()))
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// Move transfers a document from its current collection to a target collection.
func (h *DocumentHandler) Move(w http.ResponseWriter, r *http.Request) {
	c := middleware.ClaimsFromCtx(r.Context())
	docID := chi.URLParam(r, "docID")

	var req struct {
		TargetSlug          string `json:"target_slug"`
		TargetOwnerUsername string `json:"target_owner_username"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.TargetSlug == "" {
		writeJSON(w, http.StatusBadRequest, errResp("target_slug required"))
		return
	}
	targetOwner := c.Username
	if req.TargetOwnerUsername != "" && c.Role == "admin" {
		targetOwner = req.TargetOwnerUsername
	}

	srcMeta, err := database.GetDocumentMeta(h.IngestSvc.DB, docID)
	if err != nil {
		writeJSON(w, http.StatusNotFound, errResp("document not found"))
		return
	}
	if srcMeta.OwnerUsername != c.Username && c.Role != "admin" {
		writeJSON(w, http.StatusForbidden, errResp("forbidden"))
		return
	}
	srcCol, err := database.GetCollectionByID(h.IngestSvc.DB, srcMeta.CollectionID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errResp("source collection not found"))
		return
	}
	dstCol, err := database.GetCollection(h.IngestSvc.DB, targetOwner, req.TargetSlug)
	if err != nil {
		writeJSON(w, http.StatusNotFound, errResp("target collection not found"))
		return
	}
	if err := h.IngestSvc.Move(r.Context(), docID, srcCol.QdrantName, dstCol.QdrantName, dstCol.ID); err != nil {
		writeJSON(w, http.StatusInternalServerError, errResp(err.Error()))
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"message": "moved"})
}
