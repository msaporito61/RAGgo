package handlers

import (
	"encoding/json"
	"net/http"

	"raggo/internal/services/user"
)

type AuthHandler struct{ UserSvc *user.Service }

func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, errResp("invalid request"))
		return
	}
	access, refresh, err := h.UserSvc.Login(req.Username, req.Password)
	if err != nil {
		writeJSON(w, http.StatusUnauthorized, errResp("invalid credentials"))
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"access_token":  access,
		"refresh_token": refresh,
		"token_type":    "bearer",
		"expires_in":    1800,
	})
}

func (h *AuthHandler) Refresh(w http.ResponseWriter, r *http.Request) {
	var req struct {
		RefreshToken string `json:"refresh_token"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, errResp("invalid request"))
		return
	}
	access, err := h.UserSvc.RefreshToken(req.RefreshToken)
	if err != nil {
		writeJSON(w, http.StatusUnauthorized, errResp("invalid refresh token"))
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"access_token": access,
		"token_type":   "bearer",
		"expires_in":   1800,
	})
}

// shared helper used across handlers
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func errResp(msg string) map[string]string { return map[string]string{"error": msg} }
