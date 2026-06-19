package middleware

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
	"raggo/internal/config"
	"raggo/internal/database"
)

type ctxKey int

const claimsKey ctxKey = iota

type Claims struct {
	Username string
	Role     string
}

func ClaimsFromCtx(ctx context.Context) *Claims {
	c, _ := ctx.Value(claimsKey).(*Claims)
	return c
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func Authenticator(cfg *config.Config, db *sql.DB) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			claims, err := extractClaims(r, cfg, db)
			if err != nil {
				writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
				return
			}
			ctx := context.WithValue(r.Context(), claimsKey, claims)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func extractClaims(r *http.Request, cfg *config.Config, db *sql.DB) (*Claims, error) {
	// JWT takes priority
	if auth := r.Header.Get("Authorization"); strings.HasPrefix(auth, "Bearer ") {
		tokenStr := strings.TrimPrefix(auth, "Bearer ")
		return parseJWT(tokenStr, cfg.SecretKey)
	}

	// API key fallback
	if key := r.Header.Get("X-API-Key"); key != "" {
		return validateAPIKey(key, cfg, db)
	}

	return nil, errors.New("no credentials")
}

func parseJWT(tokenStr, secret string) (*Claims, error) {
	tok, err := jwt.Parse(tokenStr, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("unexpected signing method")
		}
		return []byte(secret), nil
	})
	if err != nil || !tok.Valid {
		return nil, errors.New("invalid token")
	}
	mc, ok := tok.Claims.(jwt.MapClaims)
	if !ok {
		return nil, errors.New("invalid claims")
	}
	if mc["type"] != "access" {
		return nil, errors.New("not an access token")
	}
	return &Claims{
		Username: mc["sub"].(string),
		Role:     mc["role"].(string),
	}, nil
}

func validateAPIKey(key string, cfg *config.Config, db *sql.DB) (*Claims, error) {
	// Global API key — resolves to admin
	if cfg.GlobalAPIKey != "" && key == cfg.GlobalAPIKey {
		u, err := database.GetUserByUsername(db, cfg.AdminUsername)
		if err != nil {
			return &Claims{Username: cfg.AdminUsername, Role: "admin"}, nil
		}
		return &Claims{Username: u.Username, Role: u.Role}, nil
	}

	// Per-user API keys stored as bcrypt hashes in SQLite
	users, err := database.ListUsers(db)
	if err != nil {
		return nil, err
	}
	for _, u := range users {
		if u.APIKeyHash == "" {
			continue
		}
		if bcrypt.CompareHashAndPassword([]byte(u.APIKeyHash), []byte(key)) == nil {
			return &Claims{Username: u.Username, Role: u.Role}, nil
		}
	}
	return nil, errors.New("invalid API key")
}

func RequireRole(role string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			c := ClaimsFromCtx(r.Context())
			if c == nil || c.Role != role {
				writeJSON(w, http.StatusForbidden, map[string]string{"error": "forbidden"})
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
