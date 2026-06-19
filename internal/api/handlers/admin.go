package handlers

import (
	"database/sql"
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"raggo/internal/database"
	"raggo/internal/middleware"
	"raggo/internal/services/user"
)

type AdminHandler struct {
	UserSvc *user.Service
	DB      *sql.DB
}

func (h *AdminHandler) ListUsers(w http.ResponseWriter, r *http.Request) {
	users, err := database.ListUsers(h.DB)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errResp("list error"))
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": users, "total": len(users)})
}

func (h *AdminHandler) CreateUser(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
		Role     string `json:"role"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Username == "" || req.Password == "" {
		writeJSON(w, http.StatusBadRequest, errResp("username and password required"))
		return
	}
	role := req.Role
	if role != "admin" && role != "user" {
		role = "user"
	}
	if err := h.UserSvc.CreateUser(req.Username, req.Password, role); err != nil {
		writeJSON(w, http.StatusConflict, errResp(err.Error()))
		return
	}
	w.WriteHeader(http.StatusCreated)
}

func (h *AdminHandler) DeleteUser(w http.ResponseWriter, r *http.Request) {
	username := chi.URLParam(r, "username")
	c := middleware.ClaimsFromCtx(r.Context())
	if username == c.Username {
		writeJSON(w, http.StatusBadRequest, errResp("cannot delete yourself"))
		return
	}
	if err := database.DeleteUser(h.DB, username); err != nil {
		writeJSON(w, http.StatusInternalServerError, errResp(err.Error()))
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *AdminHandler) SetAPIKey(w http.ResponseWriter, r *http.Request) {
	c := middleware.ClaimsFromCtx(r.Context())
	var req struct {
		Key string `json:"key"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Key == "" {
		writeJSON(w, http.StatusBadRequest, errResp("key required"))
		return
	}
	if err := h.UserSvc.SetAPIKey(c.Username, req.Key); err != nil {
		writeJSON(w, http.StatusInternalServerError, errResp(err.Error()))
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"message": "API key updated"})
}
