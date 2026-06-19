package handlers

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/go-chi/chi/v5"
	"raggo/internal/database"
	"raggo/internal/middleware"
	"raggo/internal/services/chat"
	"raggo/internal/services/collection"
)

type ChatHandler struct {
	ChatSvc *chat.Service
	CollSvc *collection.Service
	DB      *sql.DB
}

// CreateSession creates a new chat session.
func (h *ChatHandler) CreateSession(w http.ResponseWriter, r *http.Request) {
	c := middleware.ClaimsFromCtx(r.Context())
	var body struct {
		SessionID string `json:"session_id"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)

	req := chat.ChatRequest{
		Username:  c.Username,
		SessionID: body.SessionID,
	}
	sessionID, err := h.ChatSvc.EnsureSession(req.SessionID, req.Username)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errResp(err.Error()))
		return
	}
	writeJSON(w, http.StatusCreated, map[string]string{"session_id": sessionID})
}

// ListSessions returns all sessions for the current user.
func (h *ChatHandler) ListSessions(w http.ResponseWriter, r *http.Request) {
	c := middleware.ClaimsFromCtx(r.Context())
	sessions, err := database.ListChatSessionsForUser(h.DB, c.Username)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errResp(err.Error()))
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": sessions, "total": len(sessions)})
}

// DeleteSession removes a session and its messages.
func (h *ChatHandler) DeleteSession(w http.ResponseWriter, r *http.Request) {
	c := middleware.ClaimsFromCtx(r.Context())
	sessionID := chi.URLParam(r, "sessionID")

	// Verify ownership
	sess, err := database.GetChatSession(h.DB, sessionID)
	if err != nil || sess.Username != c.Username {
		writeJSON(w, http.StatusNotFound, errResp("session not found"))
		return
	}

	if err := database.DeleteChatSession(h.DB, sessionID); err != nil {
		writeJSON(w, http.StatusInternalServerError, errResp(err.Error()))
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// Chat handles non-streaming chat (returns full answer).
func (h *ChatHandler) Chat(w http.ResponseWriter, r *http.Request) {
	c := middleware.ClaimsFromCtx(r.Context())
	req, qdrantNames, err := h.parseChatRequest(r, c.Username)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, errResp(err.Error()))
		return
	}
	req.QdrantNames = qdrantNames

	answer, results, err := h.ChatSvc.Chat(r.Context(), *req)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errResp(err.Error()))
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"session_id": req.SessionID,
		"answer":     answer,
		"sources":    results,
	})
}

// ChatStream handles streaming SSE chat.
func (h *ChatHandler) ChatStream(w http.ResponseWriter, r *http.Request) {
	c := middleware.ClaimsFromCtx(r.Context())
	req, qdrantNames, err := h.parseChatRequest(r, c.Username)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, errResp(err.Error()))
		return
	}
	req.QdrantNames = qdrantNames

	if err := h.ChatSvc.ChatStream(r.Context(), *req, w); err != nil {
		// headers likely already written; best effort log via stderr would go here
		return
	}
}

func (h *ChatHandler) parseChatRequest(r *http.Request, username string) (*chat.ChatRequest, []string, error) {
	var body struct {
		Message         string   `json:"message"`
		SessionID       string   `json:"session_id"`
		CollectionSlugs []string `json:"collection_slugs"`
		UseHybrid       *bool    `json:"use_hybrid_search"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Message == "" {
		return nil, nil, fmt.Errorf("message required")
	}
	useHybrid := true
	if body.UseHybrid != nil {
		useHybrid = *body.UseHybrid
	}

	// resolve collection slugs → qdrant names
	var qdrantNames []string
	if len(body.CollectionSlugs) > 0 {
		for _, slug := range body.CollectionSlugs {
			col, err := database.GetCollection(h.DB, username, slug)
			if err == nil {
				qdrantNames = append(qdrantNames, col.QdrantName)
			}
		}
	}
	if len(qdrantNames) == 0 {
		// all user collections
		cols, err := database.ListCollectionsForUser(h.DB, username)
		if err == nil {
			for _, col := range cols {
				qdrantNames = append(qdrantNames, col.QdrantName)
			}
		}
	}

	return &chat.ChatRequest{
		Message:   body.Message,
		SessionID: body.SessionID,
		Username:  username,
		UseHybrid: useHybrid,
	}, qdrantNames, nil
}
