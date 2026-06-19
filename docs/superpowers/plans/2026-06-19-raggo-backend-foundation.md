# RAGgo Backend Foundation Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Scaffold the Go backend for RAGgo — project structure, config, SQLite database, JWT/API-key auth, user management, Qdrant vector store client, and OpenRouter embedding service.

**Architecture:** Chi router, modernc.org/sqlite (pure Go, no CGO), golang-jwt/jwt/v5, qdrant/go-client (gRPC), bcrypt password hashing, all services initialized once at startup and injected.

**Tech Stack:** Go 1.23, Chi v5, modernc.org/sqlite, golang-jwt/jwt/v5, qdrant/go-client, golang.org/x/crypto, joho/godotenv, google/uuid, rs/cors

## Global Constraints

- Module: `raggo`
- Backend port: `8080`
- Qdrant collection names: `rag_<username>_<slug>` (max 128 chars, slug derived from display_name)
- JWT: 30 min access + 7 day refresh; stored as Bearer in Authorization header
- Auth precedence: JWT Bearer > X-API-Key header
- Public routes (no auth): `GET /health`, `POST /auth/login`, `POST /auth/refresh`
- SQLite path: `./data/rag.db` (WAL mode, `check_same_thread=false`)
- Upload directory: `./uploads/`
- Admin user seeded from `.env` on first startup only; subsequent starts skip if user exists
- All unit tests pass before each commit

---

### Task 1: Go Project Scaffold

**Files:**
- Create: `go.mod`
- Create: `cmd/server/main.go`
- Create: `Makefile`
- Create: `.env.example`
- Create: `.gitignore`

**Interfaces:**
- Produces: compilable `go build ./...`, `make dev` runs the server

- [ ] **Step 1: Initialize Go module**

```bash
cd /Users/msaporito/Development/RAGgo
go mod init raggo
```

- [ ] **Step 2: Install core dependencies**

```bash
go get github.com/go-chi/chi/v5@latest
go get github.com/golang-jwt/jwt/v5@latest
go get modernc.org/sqlite@latest
go get github.com/qdrant/go-client@latest
go get golang.org/x/crypto@latest
go get github.com/joho/godotenv@latest
go get github.com/google/uuid@latest
go get github.com/rs/cors@latest
```

- [ ] **Step 3: Create directory structure**

```bash
mkdir -p cmd/server cmd/mcp
mkdir -p internal/{config,database,middleware,api/handlers}
mkdir -p internal/services/{user,collection,document,embedding,vectorstore,search,chat}
mkdir -p uploads data docs
```

- [ ] **Step 4: Create `cmd/server/main.go`**

```go
package main

import (
	"log"
	"net/http"
	"os"

	"github.com/joho/godotenv"
	"raggo/internal/api"
	"raggo/internal/config"
	"raggo/internal/database"
)

func main() {
	_ = godotenv.Load()

	cfg := config.Load()

	db, err := database.Open(cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("db open: %v", err)
	}
	defer db.Close()

	if err := database.Migrate(db); err != nil {
		log.Fatalf("migrate: %v", err)
	}

	router := api.NewRouter(cfg, db)

	addr := ":" + cfg.Port
	log.Printf("RAGgo listening on %s", addr)
	if err := http.ListenAndServe(addr, router); err != nil {
		log.Fatalf("server: %v", err)
		os.Exit(1)
	}
}
```

- [ ] **Step 5: Create `Makefile`**

```makefile
.PHONY: dev build test lint vet tidy

dev:
	go run ./cmd/server/...

build:
	go build -o bin/raggo ./cmd/server/...
	go build -o bin/raggo-mcp ./cmd/mcp/...

test:
	go test ./... -v -count=1

lint:
	go vet ./...

tidy:
	go mod tidy

up:
	docker compose up -d

down:
	docker compose down

logs:
	docker compose logs -f api
```

- [ ] **Step 6: Create `.env.example`**

```env
PORT=8080
ENVIRONMENT=development

# LLM
OPENROUTER_API_KEY=sk-or-...
OPENROUTER_BASE_URL=https://openrouter.ai/api/v1
LLM_MODEL=openai/gpt-4o-mini
EMBEDDING_MODEL=text-embedding-3-small

# Qdrant
QDRANT_HOST=localhost
QDRANT_PORT=6334

# Database
DATABASE_URL=./data/rag.db

# Security
SECURITY_SECRET_KEY=change-me-to-a-random-secret-32-chars
SECURITY_ADMIN_USERNAME=admin
SECURITY_ADMIN_PASSWORD=change-me
SECURITY_API_KEY=change-me-api-key
SECURITY_RATE_LIMIT_PER_MINUTE=60
```

- [ ] **Step 7: Create `.gitignore`**

```
bin/
data/*.db
uploads/
.env
*.log
```

- [ ] **Step 8: Verify it compiles (will fail until config package exists — expected)**

```bash
go build ./cmd/server/... 2>&1 || echo "Expected: missing packages"
```

- [ ] **Step 9: Commit**

```bash
git add go.mod go.sum cmd/ Makefile .env.example .gitignore
git commit -m "chore: go project scaffold and dependencies"
```

---

### Task 2: Config + SQLite Database

**Files:**
- Create: `internal/config/config.go`
- Create: `internal/database/db.go`
- Create: `internal/database/schema.sql`
- Create: `internal/database/queries.go`
- Test: `internal/database/db_test.go`

**Interfaces:**
- Produces: `config.Load() *Config`, `database.Open(url string) (*sql.DB, error)`, `database.Migrate(db *sql.DB) error`
- Produces: `database.CreateUser(db, User) error`, `database.GetUserByUsername(db, username) (*User, error)`, `database.GetUserByAPIKey(db, hash) (*User, error)`
- Produces: `database.CreateCollection(db, Collection) error`, `database.ListCollectionsForUser(db, username) ([]Collection, error)`, `database.GetCollection(db, owner, slug) (*Collection, error)`, `database.DeleteCollection(db, id) error`

- [ ] **Step 1: Create `internal/config/config.go`**

```go
package config

import (
	"os"
	"strconv"
)

type Config struct {
	Port        string
	Environment string

	OpenRouterAPIKey  string
	OpenRouterBaseURL string
	LLMModel          string
	EmbeddingModel    string

	QdrantHost string
	QdrantPort int

	DatabaseURL string

	SecretKey              string
	AdminUsername          string
	AdminPassword          string
	GlobalAPIKey           string
	RateLimitPerMinute     int
}

func Load() *Config {
	return &Config{
		Port:              getenv("PORT", "8080"),
		Environment:       getenv("ENVIRONMENT", "development"),
		OpenRouterAPIKey:  getenv("OPENROUTER_API_KEY", ""),
		OpenRouterBaseURL: getenv("OPENROUTER_BASE_URL", "https://openrouter.ai/api/v1"),
		LLMModel:          getenv("LLM_MODEL", "openai/gpt-4o-mini"),
		EmbeddingModel:    getenv("EMBEDDING_MODEL", "text-embedding-3-small"),
		QdrantHost:        getenv("QDRANT_HOST", "localhost"),
		QdrantPort:        getenvInt("QDRANT_PORT", 6334),
		DatabaseURL:       getenv("DATABASE_URL", "./data/rag.db"),
		SecretKey:         getenv("SECURITY_SECRET_KEY", ""),
		AdminUsername:     getenv("SECURITY_ADMIN_USERNAME", "admin"),
		AdminPassword:     getenv("SECURITY_ADMIN_PASSWORD", ""),
		GlobalAPIKey:      getenv("SECURITY_API_KEY", ""),
		RateLimitPerMinute: getenvInt("SECURITY_RATE_LIMIT_PER_MINUTE", 60),
	}
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getenvInt(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return fallback
}
```

- [ ] **Step 2: Create `internal/database/schema.sql`**

```sql
CREATE TABLE IF NOT EXISTS users (
    id          TEXT PRIMARY KEY,
    username    TEXT UNIQUE NOT NULL,
    password_hash TEXT NOT NULL,
    api_key_hash  TEXT,
    role        TEXT NOT NULL DEFAULT 'user',
    created_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS collections (
    id           INTEGER PRIMARY KEY AUTOINCREMENT,
    slug         TEXT NOT NULL,
    display_name TEXT NOT NULL,
    owner_username TEXT NOT NULL REFERENCES users(username),
    qdrant_name  TEXT NOT NULL UNIQUE,
    is_default   INTEGER NOT NULL DEFAULT 0,
    document_count INTEGER NOT NULL DEFAULT 0,
    created_at   DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(owner_username, slug)
);

CREATE TABLE IF NOT EXISTS document_metadata (
    id           TEXT PRIMARY KEY,
    filename     TEXT NOT NULL,
    file_type    TEXT NOT NULL,
    size_bytes   INTEGER NOT NULL DEFAULT 0,
    chunks_count INTEGER NOT NULL DEFAULT 0,
    status       TEXT NOT NULL DEFAULT 'processing',
    owner_username TEXT NOT NULL REFERENCES users(username),
    collection_id  INTEGER NOT NULL REFERENCES collections(id),
    uploaded_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS chat_sessions (
    id         TEXT PRIMARY KEY,
    username   TEXT NOT NULL REFERENCES users(username),
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS chat_messages (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    session_id TEXT NOT NULL REFERENCES chat_sessions(id) ON DELETE CASCADE,
    role       TEXT NOT NULL,
    content    TEXT NOT NULL,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS audit_log (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    username   TEXT,
    event      TEXT NOT NULL,
    ip         TEXT,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
```

- [ ] **Step 3: Create `internal/database/db.go`**

```go
package database

import (
	"database/sql"
	_ "embed"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"
)

//go:embed schema.sql
var schema string

func Open(dsn string) (*sql.DB, error) {
	if err := os.MkdirAll(filepath.Dir(dsn), 0755); err != nil {
		return nil, err
	}
	db, err := sql.Open("sqlite", dsn+"?_journal_mode=WAL&_foreign_keys=on")
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1) // SQLite write serialization
	return db, db.Ping()
}

func Migrate(db *sql.DB) error {
	_, err := db.Exec(schema)
	return err
}
```

- [ ] **Step 4: Create `internal/database/queries.go`** — user and collection CRUD

```go
package database

import (
	"database/sql"
	"time"
)

type User struct {
	ID           string
	Username     string
	PasswordHash string
	APIKeyHash   string
	Role         string
	CreatedAt    time.Time
}

type Collection struct {
	ID            int64
	Slug          string
	DisplayName   string
	OwnerUsername string
	QdrantName    string
	IsDefault     bool
	DocumentCount int
	CreatedAt     time.Time
}

type DocumentMeta struct {
	ID            string
	Filename      string
	FileType      string
	SizeBytes     int64
	ChunksCount   int
	Status        string
	OwnerUsername string
	CollectionID  int64
	UploadedAt    time.Time
}

// Users

func CreateUser(db *sql.DB, u User) error {
	_, err := db.Exec(
		`INSERT INTO users (id, username, password_hash, api_key_hash, role) VALUES (?,?,?,?,?)`,
		u.ID, u.Username, u.PasswordHash, u.APIKeyHash, u.Role,
	)
	return err
}

func GetUserByUsername(db *sql.DB, username string) (*User, error) {
	u := &User{}
	var apiKey sql.NullString
	err := db.QueryRow(
		`SELECT id, username, password_hash, api_key_hash, role, created_at FROM users WHERE username = ?`,
		username,
	).Scan(&u.ID, &u.Username, &u.PasswordHash, &apiKey, &u.Role, &u.CreatedAt)
	if err != nil {
		return nil, err
	}
	u.APIKeyHash = apiKey.String
	return u, nil
}

func GetUserByID(db *sql.DB, id string) (*User, error) {
	u := &User{}
	var apiKey sql.NullString
	err := db.QueryRow(
		`SELECT id, username, password_hash, api_key_hash, role, created_at FROM users WHERE id = ?`,
		id,
	).Scan(&u.ID, &u.Username, &u.PasswordHash, &apiKey, &u.Role, &u.CreatedAt)
	if err != nil {
		return nil, err
	}
	u.APIKeyHash = apiKey.String
	return u, nil
}

func ListUsers(db *sql.DB) ([]User, error) {
	rows, err := db.Query(`SELECT id, username, role, created_at FROM users ORDER BY created_at`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var users []User
	for rows.Next() {
		var u User
		if err := rows.Scan(&u.ID, &u.Username, &u.Role, &u.CreatedAt); err != nil {
			return nil, err
		}
		users = append(users, u)
	}
	return users, rows.Err()
}

func UpdateUserPasswordHash(db *sql.DB, username, hash string) error {
	_, err := db.Exec(`UPDATE users SET password_hash = ? WHERE username = ?`, hash, username)
	return err
}

func UpdateUserAPIKeyHash(db *sql.DB, username, hash string) error {
	_, err := db.Exec(`UPDATE users SET api_key_hash = ? WHERE username = ?`, hash, username)
	return err
}

func DeleteUser(db *sql.DB, username string) error {
	_, err := db.Exec(`DELETE FROM users WHERE username = ?`, username)
	return err
}

// Collections

func CreateCollection(db *sql.DB, c Collection) error {
	isDefault := 0
	if c.IsDefault {
		isDefault = 1
	}
	_, err := db.Exec(
		`INSERT INTO collections (slug, display_name, owner_username, qdrant_name, is_default) VALUES (?,?,?,?,?)`,
		c.Slug, c.DisplayName, c.OwnerUsername, c.QdrantName, isDefault,
	)
	return err
}

func GetCollection(db *sql.DB, ownerUsername, slug string) (*Collection, error) {
	c := &Collection{}
	var isDefault int
	err := db.QueryRow(
		`SELECT id, slug, display_name, owner_username, qdrant_name, is_default, document_count, created_at
		 FROM collections WHERE owner_username = ? AND slug = ?`,
		ownerUsername, slug,
	).Scan(&c.ID, &c.Slug, &c.DisplayName, &c.OwnerUsername, &c.QdrantName, &isDefault, &c.DocumentCount, &c.CreatedAt)
	if err != nil {
		return nil, err
	}
	c.IsDefault = isDefault == 1
	return c, nil
}

func GetCollectionByID(db *sql.DB, id int64) (*Collection, error) {
	c := &Collection{}
	var isDefault int
	err := db.QueryRow(
		`SELECT id, slug, display_name, owner_username, qdrant_name, is_default, document_count, created_at
		 FROM collections WHERE id = ?`, id,
	).Scan(&c.ID, &c.Slug, &c.DisplayName, &c.OwnerUsername, &c.QdrantName, &isDefault, &c.DocumentCount, &c.CreatedAt)
	if err != nil {
		return nil, err
	}
	c.IsDefault = isDefault == 1
	return c, nil
}

func ListCollectionsForUser(db *sql.DB, username string) ([]Collection, error) {
	rows, err := db.Query(
		`SELECT id, slug, display_name, owner_username, qdrant_name, is_default, document_count, created_at
		 FROM collections WHERE owner_username = ? ORDER BY is_default DESC, created_at`,
		username,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanCollections(rows)
}

func ListAllCollections(db *sql.DB) ([]Collection, error) {
	rows, err := db.Query(
		`SELECT id, slug, display_name, owner_username, qdrant_name, is_default, document_count, created_at
		 FROM collections ORDER BY owner_username, is_default DESC, created_at`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanCollections(rows)
}

func DeleteCollection(db *sql.DB, id int64) error {
	_, err := db.Exec(`DELETE FROM collections WHERE id = ?`, id)
	return err
}

func IncrementDocumentCount(db *sql.DB, collectionID int64, delta int) error {
	_, err := db.Exec(`UPDATE collections SET document_count = document_count + ? WHERE id = ?`, delta, collectionID)
	return err
}

func scanCollections(rows *sql.Rows) ([]Collection, error) {
	var cols []Collection
	for rows.Next() {
		c := Collection{}
		var isDefault int
		if err := rows.Scan(&c.ID, &c.Slug, &c.DisplayName, &c.OwnerUsername, &c.QdrantName, &isDefault, &c.DocumentCount, &c.CreatedAt); err != nil {
			return nil, err
		}
		c.IsDefault = isDefault == 1
		cols = append(cols, c)
	}
	return cols, rows.Err()
}

// DocumentMeta

func CreateDocumentMeta(db *sql.DB, d DocumentMeta) error {
	_, err := db.Exec(
		`INSERT INTO document_metadata (id, filename, file_type, size_bytes, chunks_count, status, owner_username, collection_id)
		 VALUES (?,?,?,?,?,?,?,?)`,
		d.ID, d.Filename, d.FileType, d.SizeBytes, d.ChunksCount, d.Status, d.OwnerUsername, d.CollectionID,
	)
	return err
}

func GetDocumentMeta(db *sql.DB, id string) (*DocumentMeta, error) {
	d := &DocumentMeta{}
	err := db.QueryRow(
		`SELECT id, filename, file_type, size_bytes, chunks_count, status, owner_username, collection_id, uploaded_at
		 FROM document_metadata WHERE id = ?`, id,
	).Scan(&d.ID, &d.Filename, &d.FileType, &d.SizeBytes, &d.ChunksCount, &d.Status, &d.OwnerUsername, &d.CollectionID, &d.UploadedAt)
	if err != nil {
		return nil, err
	}
	return d, nil
}

func ListDocumentsForUser(db *sql.DB, username string, page, pageSize int) ([]DocumentMeta, int, error) {
	offset := (page - 1) * pageSize
	var total int
	if err := db.QueryRow(`SELECT COUNT(*) FROM document_metadata WHERE owner_username = ?`, username).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := db.Query(
		`SELECT id, filename, file_type, size_bytes, chunks_count, status, owner_username, collection_id, uploaded_at
		 FROM document_metadata WHERE owner_username = ? ORDER BY uploaded_at DESC LIMIT ? OFFSET ?`,
		username, pageSize, offset,
	)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	var docs []DocumentMeta
	for rows.Next() {
		d := DocumentMeta{}
		if err := rows.Scan(&d.ID, &d.Filename, &d.FileType, &d.SizeBytes, &d.ChunksCount, &d.Status, &d.OwnerUsername, &d.CollectionID, &d.UploadedAt); err != nil {
			return nil, 0, err
		}
		docs = append(docs, d)
	}
	return docs, total, rows.Err()
}

func DeleteDocumentMeta(db *sql.DB, id string) error {
	_, err := db.Exec(`DELETE FROM document_metadata WHERE id = ?`, id)
	return err
}

func UpdateDocumentCollection(db *sql.DB, docID string, newCollectionID int64) error {
	_, err := db.Exec(`UPDATE document_metadata SET collection_id = ? WHERE id = ?`, newCollectionID, docID)
	return err
}

func UpdateDocumentChunksAndStatus(db *sql.DB, id string, chunks int, status string) error {
	_, err := db.Exec(`UPDATE document_metadata SET chunks_count = ?, status = ? WHERE id = ?`, chunks, status, id)
	return err
}

// Audit

func InsertAuditLog(db *sql.DB, username, event, ip string) error {
	_, err := db.Exec(`INSERT INTO audit_log (username, event, ip) VALUES (?,?,?)`, username, event, ip)
	return err
}
```

- [ ] **Step 5: Write test for database**

Create `internal/database/db_test.go`:

```go
package database

import (
	"testing"

	"github.com/google/uuid"
)

func newTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := Open(":memory:")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if err := Migrate(db); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func TestUserCRUD(t *testing.T) {
	db := newTestDB(t)

	u := User{ID: uuid.NewString(), Username: "alice", PasswordHash: "hash", Role: "user"}
	if err := CreateUser(db, u); err != nil {
		t.Fatalf("create: %v", err)
	}

	got, err := GetUserByUsername(db, "alice")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Username != "alice" {
		t.Errorf("got %q, want alice", got.Username)
	}
	if got.Role != "user" {
		t.Errorf("got role %q, want user", got.Role)
	}
}

func TestCollectionCRUD(t *testing.T) {
	db := newTestDB(t)

	u := User{ID: uuid.NewString(), Username: "bob", PasswordHash: "h", Role: "user"}
	_ = CreateUser(db, u)

	col := Collection{
		Slug: "default", DisplayName: "Default",
		OwnerUsername: "bob", QdrantName: "rag_bob_default", IsDefault: true,
	}
	if err := CreateCollection(db, col); err != nil {
		t.Fatalf("create collection: %v", err)
	}

	got, err := GetCollection(db, "bob", "default")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.QdrantName != "rag_bob_default" {
		t.Errorf("qdrant name mismatch: %q", got.QdrantName)
	}
	if !got.IsDefault {
		t.Error("expected is_default=true")
	}
}

func TestListDocumentsOrder(t *testing.T) {
	db := newTestDB(t)
	u := User{ID: uuid.NewString(), Username: "carol", PasswordHash: "h", Role: "user"}
	_ = CreateUser(db, u)
	col := Collection{Slug: "default", DisplayName: "Default", OwnerUsername: "carol", QdrantName: "rag_carol_default", IsDefault: true}
	_ = CreateCollection(db, col)
	c, _ := GetCollection(db, "carol", "default")

	for i, name := range []string{"first.txt", "second.txt", "third.txt"} {
		_ = CreateDocumentMeta(db, DocumentMeta{
			ID: uuid.NewString(), Filename: name, FileType: ".txt",
			OwnerUsername: "carol", CollectionID: c.ID, Status: "processed",
		})
		_ = i
	}

	docs, total, err := ListDocumentsForUser(db, "carol", 1, 10)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if total != 3 {
		t.Errorf("total %d, want 3", total)
	}
	// newest first — last inserted should be first
	if docs[0].Filename != "third.txt" {
		t.Errorf("expected newest first, got %q", docs[0].Filename)
	}
}
```

Add missing import to the test file header:
```go
import (
	"database/sql"
	"testing"
	"github.com/google/uuid"
)
```

- [ ] **Step 6: Run the test**

```bash
go test ./internal/database/... -v
```

Expected: PASS (3 tests)

- [ ] **Step 7: Commit**

```bash
git add internal/config/ internal/database/
git commit -m "feat: config, SQLite schema and CRUD queries"
```

---

### Task 3: Auth Middleware + /auth Endpoints

**Files:**
- Create: `internal/middleware/auth.go`
- Create: `internal/middleware/security.go`
- Create: `internal/services/user/service.go`
- Create: `internal/api/handlers/auth.go`
- Create: `internal/api/router.go`
- Test: `internal/middleware/auth_test.go`
- Test: `internal/services/user/service_test.go`

**Interfaces:**
- Consumes: `database.GetUserByUsername`, `database.GetUserByID`
- Produces: `middleware.Authenticator(cfg, db) func(http.Handler) http.Handler`
- Produces: `middleware.ClaimsFromCtx(ctx) *Claims` — extracts authenticated user from context
- Produces: `user.Service{}.Login(username, password) (accessToken, refreshToken string, err error)`
- Produces: `user.Service{}.RefreshToken(refreshToken string) (newAccessToken string, err error)`
- Produces: `api.NewRouter(cfg, db) http.Handler`

- [ ] **Step 1: Write failing test for auth middleware**

Create `internal/middleware/auth_test.go`:

```go
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
```

- [ ] **Step 2: Run test — verify it fails**

```bash
go test ./internal/middleware/... -v 2>&1 | head -20
```

Expected: FAIL — package not found

- [ ] **Step 3: Create `internal/middleware/auth.go`**

```go
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
```

- [ ] **Step 4: Create `internal/middleware/security.go`**

```go
package middleware

import "net/http"

func SecurityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("X-XSS-Protection", "1; mode=block")
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
		next.ServeHTTP(w, r)
	})
}
```

- [ ] **Step 5: Create `internal/services/user/service.go`**

```go
package user

import (
	"database/sql"
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
	"raggo/internal/config"
	"raggo/internal/database"
)

type Service struct {
	DB  *sql.DB
	Cfg *config.Config
}

func (s *Service) SeedAdmin() error {
	_, err := database.GetUserByUsername(s.DB, s.Cfg.AdminUsername)
	if err == nil {
		return nil // already exists
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(s.Cfg.AdminPassword), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	return database.CreateUser(s.DB, database.User{
		ID:           uuid.NewString(),
		Username:     s.Cfg.AdminUsername,
		PasswordHash: string(hash),
		Role:         "admin",
	})
}

func (s *Service) Login(username, password string) (access, refresh string, err error) {
	u, err := database.GetUserByUsername(s.DB, username)
	if err != nil {
		return "", "", errors.New("invalid credentials")
	}
	if bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(password)) != nil {
		return "", "", errors.New("invalid credentials")
	}
	access, err = s.makeToken(u.Username, u.Role, "access", time.Now().Add(30*time.Minute))
	if err != nil {
		return "", "", err
	}
	refresh, err = s.makeToken(u.Username, u.Role, "refresh", time.Now().Add(7*24*time.Hour))
	return
}

func (s *Service) RefreshToken(refreshTok string) (string, error) {
	tok, err := jwt.Parse(refreshTok, func(t *jwt.Token) (any, error) {
		return []byte(s.Cfg.SecretKey), nil
	})
	if err != nil || !tok.Valid {
		return "", errors.New("invalid refresh token")
	}
	mc := tok.Claims.(jwt.MapClaims)
	if mc["type"] != "refresh" {
		return "", errors.New("not a refresh token")
	}
	return s.makeToken(mc["sub"].(string), mc["role"].(string), "access", time.Now().Add(30*time.Minute))
}

func (s *Service) makeToken(username, role, typ string, exp time.Time) (string, error) {
	claims := jwt.MapClaims{
		"sub":  username,
		"role": role,
		"type": typ,
		"exp":  exp.Unix(),
		"iat":  time.Now().Unix(),
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(s.Cfg.SecretKey))
}

func (s *Service) CreateUser(username, password, role string) error {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	return database.CreateUser(s.DB, database.User{
		ID:           uuid.NewString(),
		Username:     username,
		PasswordHash: string(hash),
		Role:         role,
	})
}

func (s *Service) SetAPIKey(username, plainKey string) error {
	hash, err := bcrypt.GenerateFromPassword([]byte(plainKey), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	return database.UpdateUserAPIKeyHash(s.DB, username, string(hash))
}
```

- [ ] **Step 6: Create `internal/api/handlers/auth.go`**

```go
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
```

- [ ] **Step 7: Create `internal/api/router.go`** (skeleton — handlers added in later tasks)

```go
package api

import (
	"database/sql"
	"net/http"

	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"
	"github.com/rs/cors"
	"raggo/internal/api/handlers"
	"raggo/internal/config"
	"raggo/internal/middleware"
	"raggo/internal/services/user"
)

func NewRouter(cfg *config.Config, db *sql.DB) http.Handler {
	r := chi.NewRouter()

	r.Use(chimw.Logger)
	r.Use(chimw.Recoverer)
	r.Use(middleware.SecurityHeaders)

	corsHandler := cors.New(cors.Options{
		AllowedOrigins:   []string{"*"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Authorization", "Content-Type", "X-API-Key"},
		AllowCredentials: true,
	})
	r.Use(corsHandler.Handler)

	userSvc := &user.Service{DB: db, Cfg: cfg}
	authH := &handlers.AuthHandler{UserSvc: userSvc}

	// Public routes
	r.Get("/health", handlers.Health(cfg))
	r.Post("/auth/login", authH.Login)
	r.Post("/auth/refresh", authH.Refresh)

	// Protected routes
	r.Group(func(r chi.Router) {
		r.Use(middleware.Authenticator(cfg, db))
		// handlers added in later tasks
	})

	return r
}
```

- [ ] **Step 8: Add health handler stub**

Create `internal/api/handlers/health.go`:

```go
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
```

- [ ] **Step 9: Run middleware tests**

```bash
go test ./internal/middleware/... ./internal/services/user/... -v
```

Expected: PASS

- [ ] **Step 10: Verify server compiles**

```bash
go build ./...
```

Expected: no errors

- [ ] **Step 11: Commit**

```bash
git add internal/middleware/ internal/services/user/ internal/api/
git commit -m "feat: JWT/API-key auth middleware and /auth endpoints"
```

---

### Task 4: Qdrant Vector Store Client

**Files:**
- Create: `internal/services/vectorstore/qdrant.go`
- Test: `internal/services/vectorstore/qdrant_test.go`

**Interfaces:**
- Produces: `vectorstore.Client` struct with methods:
  - `EnsureCollection(ctx, name string, vectorSize uint64) error`
  - `DeleteCollection(ctx, name string) error`
  - `Upsert(ctx, collection string, points []Point) error`
  - `Search(ctx, collection string, vector []float32, limit uint64, filter map[string]string) ([]SearchResult, error)`
  - `DeleteByDocumentID(ctx, collection, docID string) error`
  - `ScrollByDocumentID(ctx, collection, docID string) ([]PointWithVector, error)` — used for document moves

- [ ] **Step 1: Install Qdrant client**

```bash
go get github.com/qdrant/go-client@latest
```

- [ ] **Step 2: Write failing test (uses real Qdrant — skip if not running)**

Create `internal/services/vectorstore/qdrant_test.go`:

```go
package vectorstore

import (
	"context"
	"os"
	"testing"
)

func TestQdrantIntegration(t *testing.T) {
	if os.Getenv("QDRANT_HOST") == "" {
		t.Skip("QDRANT_HOST not set; skipping integration test")
	}
	c, err := NewClient("localhost", 6334)
	if err != nil {
		t.Fatalf("new client: %v", err)
	}
	ctx := context.Background()

	const coll = "test_raggo_vectorstore"
	if err := c.EnsureCollection(ctx, coll, 3); err != nil {
		t.Fatalf("ensure: %v", err)
	}
	defer c.DeleteCollection(ctx, coll)

	pts := []Point{
		{ID: "doc1_0", Vector: []float32{0.1, 0.2, 0.3}, Payload: map[string]string{"document_id": "doc1", "text": "hello"}},
		{ID: "doc1_1", Vector: []float32{0.4, 0.5, 0.6}, Payload: map[string]string{"document_id": "doc1", "text": "world"}},
	}
	if err := c.Upsert(ctx, coll, pts); err != nil {
		t.Fatalf("upsert: %v", err)
	}

	results, err := c.Search(ctx, coll, []float32{0.1, 0.2, 0.3}, 5, nil)
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if len(results) == 0 {
		t.Error("expected results")
	}

	if err := c.DeleteByDocumentID(ctx, coll, "doc1"); err != nil {
		t.Fatalf("delete: %v", err)
	}
}
```

- [ ] **Step 3: Create `internal/services/vectorstore/qdrant.go`**

```go
package vectorstore

import (
	"context"
	"fmt"

	"github.com/qdrant/go-client/qdrant"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type Point struct {
	ID      string
	Vector  []float32
	Payload map[string]string
}

type SearchResult struct {
	ID      string
	Score   float32
	Payload map[string]string
}

type PointWithVector struct {
	ID      string
	Vector  []float32
	Payload map[string]string
}

type Client struct {
	conn        *grpc.ClientConn
	collections qdrant.CollectionsClient
	points      qdrant.PointsClient
}

func NewClient(host string, port int) (*Client, error) {
	addr := fmt.Sprintf("%s:%d", host, port)
	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, err
	}
	return &Client{
		conn:        conn,
		collections: qdrant.NewCollectionsClient(conn),
		points:      qdrant.NewPointsClient(conn),
	}, nil
}

func (c *Client) Close() { c.conn.Close() }

func (c *Client) EnsureCollection(ctx context.Context, name string, vectorSize uint64) error {
	_, err := c.collections.Get(ctx, &qdrant.GetCollectionInfoRequest{CollectionName: name})
	if err == nil {
		return nil // already exists
	}
	_, err = c.collections.Create(ctx, &qdrant.CreateCollection{
		CollectionName: name,
		VectorsConfig: &qdrant.VectorsConfig{
			Config: &qdrant.VectorsConfig_Params{
				Params: &qdrant.VectorParams{
					Size:     vectorSize,
					Distance: qdrant.Distance_Cosine,
				},
			},
		},
	})
	return err
}

func (c *Client) DeleteCollection(ctx context.Context, name string) error {
	_, err := c.collections.Delete(ctx, &qdrant.DeleteCollection{CollectionName: name})
	return err
}

func (c *Client) Upsert(ctx context.Context, collection string, points []Point) error {
	var qpts []*qdrant.PointStruct
	for _, p := range points {
		payload := make(map[string]*qdrant.Value)
		for k, v := range p.Payload {
			payload[k] = &qdrant.Value{Kind: &qdrant.Value_StringValue{StringValue: v}}
		}
		qpts = append(qpts, &qdrant.PointStruct{
			Id:      &qdrant.PointId{PointIdOptions: &qdrant.PointId_Uuid{Uuid: p.ID}},
			Vectors: &qdrant.Vectors{VectorsOptions: &qdrant.Vectors_Vector{Vector: &qdrant.Vector{Data: p.Vector}}},
			Payload: payload,
		})
	}
	_, err := c.points.Upsert(ctx, &qdrant.UpsertPoints{CollectionName: collection, Points: qpts, Wait: boolPtr(true)})
	return err
}

func (c *Client) Search(ctx context.Context, collection string, vector []float32, limit uint64, filter map[string]string) ([]SearchResult, error) {
	req := &qdrant.SearchPoints{
		CollectionName: collection,
		Vector:         vector,
		Limit:          limit,
		WithPayload:    &qdrant.WithPayloadSelector{SelectorOptions: &qdrant.WithPayloadSelector_Enable{Enable: true}},
	}
	if len(filter) > 0 {
		var conditions []*qdrant.Condition
		for k, v := range filter {
			conditions = append(conditions, &qdrant.Condition{
				ConditionOneOf: &qdrant.Condition_Field{
					Field: &qdrant.FieldCondition{
						Key: k,
						Match: &qdrant.Match{MatchValue: &qdrant.Match_Keyword{Keyword: v}},
					},
				},
			})
		}
		req.Filter = &qdrant.Filter{Must: conditions}
	}
	resp, err := c.points.Search(ctx, req)
	if err != nil {
		return nil, err
	}
	var results []SearchResult
	for _, r := range resp.Result {
		payload := make(map[string]string)
		for k, v := range r.Payload {
			if sv, ok := v.Kind.(*qdrant.Value_StringValue); ok {
				payload[k] = sv.StringValue
			}
		}
		results = append(results, SearchResult{
			ID:      r.Id.GetUuid(),
			Score:   r.Score,
			Payload: payload,
		})
	}
	return results, nil
}

func (c *Client) DeleteByDocumentID(ctx context.Context, collection, docID string) error {
	_, err := c.points.Delete(ctx, &qdrant.DeletePoints{
		CollectionName: collection,
		Points: &qdrant.PointsSelector{
			PointsSelectorOneOf: &qdrant.PointsSelector_Filter{
				Filter: &qdrant.Filter{
					Must: []*qdrant.Condition{{
						ConditionOneOf: &qdrant.Condition_Field{
							Field: &qdrant.FieldCondition{
								Key:   "document_id",
								Match: &qdrant.Match{MatchValue: &qdrant.Match_Keyword{Keyword: docID}},
							},
						},
					}},
				},
			},
		},
		Wait: boolPtr(true),
	})
	return err
}

func (c *Client) ScrollByDocumentID(ctx context.Context, collection, docID string) ([]PointWithVector, error) {
	resp, err := c.points.Scroll(ctx, &qdrant.ScrollPoints{
		CollectionName: collection,
		Filter: &qdrant.Filter{
			Must: []*qdrant.Condition{{
				ConditionOneOf: &qdrant.Condition_Field{
					Field: &qdrant.FieldCondition{
						Key:   "document_id",
						Match: &qdrant.Match{MatchValue: &qdrant.Match_Keyword{Keyword: docID}},
					},
				},
			}},
		},
		WithVectors: &qdrant.WithVectorsSelector{SelectorOptions: &qdrant.WithVectorsSelector_Enable{Enable: true}},
		WithPayload: &qdrant.WithPayloadSelector{SelectorOptions: &qdrant.WithPayloadSelector_Enable{Enable: true}},
		Limit:       uint32Ptr(10000),
	})
	if err != nil {
		return nil, err
	}
	var pts []PointWithVector
	for _, p := range resp.Result {
		payload := make(map[string]string)
		for k, v := range p.Payload {
			if sv, ok := v.Kind.(*qdrant.Value_StringValue); ok {
				payload[k] = sv.StringValue
			}
		}
		var vec []float32
		if p.Vectors != nil {
			if v := p.Vectors.GetVector(); v != nil {
				vec = v.Data
			}
		}
		pts = append(pts, PointWithVector{ID: p.Id.GetUuid(), Vector: vec, Payload: payload})
	}
	return pts, nil
}

func boolPtr(b bool) *bool         { return &b }
func uint32Ptr(n uint32) *uint32   { return &n }
```

- [ ] **Step 4: Run test (skips without Qdrant)**

```bash
go test ./internal/services/vectorstore/... -v
```

Expected: SKIP or PASS

- [ ] **Step 5: Commit**

```bash
git add internal/services/vectorstore/
git commit -m "feat: Qdrant gRPC client wrapper"
```

---

### Task 5: Embedding Service

**Files:**
- Create: `internal/services/embedding/service.go`
- Test: `internal/services/embedding/service_test.go`

**Interfaces:**
- Produces: `embedding.Service{}.Embed(ctx, texts []string) ([][]float32, error)`
- Produces: `embedding.Service{}.VectorSize() uint64`

- [ ] **Step 1: Write failing test**

Create `internal/services/embedding/service_test.go`:

```go
package embedding

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestEmbed(t *testing.T) {
	// stub server returning fake embeddings
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]any{
			"data": []map[string]any{
				{"embedding": []float32{0.1, 0.2, 0.3}, "index": 0},
				{"embedding": []float32{0.4, 0.5, 0.6}, "index": 1},
			},
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	svc := &Service{BaseURL: srv.URL, APIKey: "test", Model: "test-model"}
	vecs, err := svc.Embed(context.Background(), []string{"hello", "world"})
	if err != nil {
		t.Fatalf("embed: %v", err)
	}
	if len(vecs) != 2 {
		t.Fatalf("want 2 vectors, got %d", len(vecs))
	}
	if vecs[0][0] != 0.1 {
		t.Errorf("unexpected value: %v", vecs[0][0])
	}
}
```

- [ ] **Step 2: Run test — verify fail**

```bash
go test ./internal/services/embedding/... -v 2>&1 | head -10
```

Expected: FAIL — package not found

- [ ] **Step 3: Create `internal/services/embedding/service.go`**

```go
package embedding

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

const defaultVectorSize = 1536 // text-embedding-3-small

type Service struct {
	BaseURL string
	APIKey  string
	Model   string
	client  *http.Client
}

func New(baseURL, apiKey, model string) *Service {
	return &Service{BaseURL: baseURL, APIKey: apiKey, Model: model, client: &http.Client{}}
}

func (s *Service) VectorSize() uint64 { return defaultVectorSize }

func (s *Service) Embed(ctx context.Context, texts []string) ([][]float32, error) {
	body, _ := json.Marshal(map[string]any{"model": s.Model, "input": texts})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.BaseURL+"/embeddings", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+s.APIKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("embeddings API: status %d", resp.StatusCode)
	}

	var out struct {
		Data []struct {
			Embedding []float32 `json:"embedding"`
			Index     int       `json:"index"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}

	// sort by index to preserve order
	result := make([][]float32, len(texts))
	for _, d := range out.Data {
		if d.Index < len(result) {
			result[d.Index] = d.Embedding
		}
	}
	return result, nil
}
```

- [ ] **Step 4: Run test**

```bash
go test ./internal/services/embedding/... -v
```

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/services/embedding/
git commit -m "feat: OpenRouter embedding service"
```

---

*Continue in `2026-06-19-raggo-backend-core.md`*
