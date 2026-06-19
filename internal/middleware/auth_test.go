package middleware

import (
	"database/sql"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"raggo/internal/config"
	"raggo/internal/database"
)

func makeTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, _ := database.Open(":memory:")
	_ = database.Migrate(db)
	t.Cleanup(func() { db.Close() })
	return db
}

func makeToken(secret, username, role, typ string, exp time.Time) string {
	claims := jwt.MapClaims{
		"sub":  username,
		"role": role,
		"type": typ,
		"exp":  exp.Unix(),
	}
	tok, _ := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(secret))
	return tok
}

func TestAuthMiddleware_ValidJWT(t *testing.T) {
	cfg := &config.Config{SecretKey: "testsecret", GlobalAPIKey: "gk"}
	db := makeTestDB(t)
	_ = database.CreateUser(db, database.User{
		ID: "1", Username: "alice", PasswordHash: "h", Role: "user",
	})

	tok := makeToken("testsecret", "alice", "user", "access", time.Now().Add(time.Hour))
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer "+tok)

	var called bool
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c := ClaimsFromCtx(r.Context())
		if c == nil || c.Username != "alice" {
			t.Error("expected alice in context")
		}
		called = true
	})

	rr := httptest.NewRecorder()
	Authenticator(cfg, db)(next).ServeHTTP(rr, req)

	if !called {
		t.Error("next handler not called")
	}
}

func TestAuthMiddleware_MissingToken(t *testing.T) {
	cfg := &config.Config{SecretKey: "testsecret", GlobalAPIKey: "gk"}
	db := makeTestDB(t)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("next should not be called")
	})
	Authenticator(cfg, db)(next).ServeHTTP(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("want 401, got %d", rr.Code)
	}
}

func TestAuthMiddleware_GlobalAPIKey(t *testing.T) {
	cfg := &config.Config{SecretKey: "testsecret", GlobalAPIKey: "myglobalkey"}
	db := makeTestDB(t)
	_ = database.CreateUser(db, database.User{
		ID: "1", Username: "admin", PasswordHash: "h", Role: "admin",
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-API-Key", "myglobalkey")

	var called bool
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { called = true })
	rr := httptest.NewRecorder()
	Authenticator(cfg, db)(next).ServeHTTP(rr, req)
	if !called {
		t.Error("next not called for valid global API key")
	}
}
