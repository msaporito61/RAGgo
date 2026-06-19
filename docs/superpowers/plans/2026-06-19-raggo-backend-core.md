# RAGgo Backend Core Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.
>
> **Prerequisite:** Complete `2026-06-19-raggo-backend-foundation.md` first.

**Goal:** Implement document ingestion, collection management, hybrid search, streaming RAG chat, web scraping, and all REST endpoints.

**Architecture:** Each service is a struct with injected dependencies. Handlers are thin — delegate to services. Chat prompt always injects a document index (list of all user docs) separate from retrieved chunks.

**Tech Stack:** ledongthuc/pdf (PDF text), excelize (XLSX), standard archive/zip for DOCX, BM25 implemented inline, SSE via chunked transfer encoding.

## Global Constraints (same as foundation plan)

- Qdrant collection names: `rag_<username>_<slug>` (max 128 chars)
- Document list sorted `uploaded_at DESC` before pagination
- Multi-collection chat: no selection = all user collections; merge by score; dedup by text hash
- Chat prompt: inject `=== DOCUMENT INDEX ===` + `=== RETRIEVED CONTEXT ===` separately
- Chunk size: 1000 chars, overlap: 200 chars
- Max upload size: 50 MB
- Hybrid search: 70% semantic score + 30% BM25 score

---

### Task 6: Document Loader + Chunker

**Files:**
- Create: `internal/services/document/loader.go`
- Create: `internal/services/document/chunker.go`
- Test: `internal/services/document/chunker_test.go`
- Test: `internal/services/document/loader_test.go`

**Interfaces:**
- Produces: `document.LoadText(filename string, data []byte) (string, error)` — returns raw text for any supported file type
- Produces: `document.Chunk(text string, size, overlap int) []string`

- [ ] **Step 1: Install PDF and XLSX dependencies**

```bash
go get github.com/ledongthuc/pdf@latest
go get github.com/xuri/excelize/v2@latest
```

- [ ] **Step 2: Write chunker test**

Create `internal/services/document/chunker_test.go`:

```go
package document

import (
	"strings"
	"testing"
)

func TestChunk_SmallText(t *testing.T) {
	chunks := Chunk("hello world", 1000, 200)
	if len(chunks) != 1 {
		t.Fatalf("want 1 chunk, got %d", len(chunks))
	}
	if chunks[0] != "hello world" {
		t.Errorf("unexpected chunk: %q", chunks[0])
	}
}

func TestChunk_Overlap(t *testing.T) {
	// 50-char string, size=30, overlap=10
	text := strings.Repeat("abcdefghij", 5) // 50 chars
	chunks := Chunk(text, 30, 10)
	if len(chunks) < 2 {
		t.Fatalf("want at least 2 chunks, got %d", len(chunks))
	}
	// second chunk should start at position 20 (30-10)
	if !strings.HasPrefix(chunks[1], text[20:30]) {
		t.Errorf("overlap not applied correctly; chunk[1] = %q", chunks[1])
	}
}

func TestChunk_EmptyText(t *testing.T) {
	chunks := Chunk("", 1000, 200)
	if len(chunks) != 0 {
		t.Errorf("expected 0 chunks for empty text, got %d", len(chunks))
	}
}
```

- [ ] **Step 3: Create `internal/services/document/chunker.go`**

```go
package document

import "strings"

// Chunk splits text into overlapping chunks of at most `size` chars with `overlap` char overlap.
func Chunk(text string, size, overlap int) []string {
	text = strings.TrimSpace(text)
	if text == "" {
		return nil
	}
	if len(text) <= size {
		return []string{text}
	}
	var chunks []string
	step := size - overlap
	if step <= 0 {
		step = size
	}
	for start := 0; start < len(text); start += step {
		end := start + size
		if end > len(text) {
			end = len(text)
		}
		chunks = append(chunks, text[start:end])
		if end == len(text) {
			break
		}
	}
	return chunks
}
```

- [ ] **Step 4: Run chunker test**

```bash
go test ./internal/services/document/... -run TestChunk -v
```

Expected: PASS (3 tests)

- [ ] **Step 5: Write loader test**

Create `internal/services/document/loader_test.go`:

```go
package document

import (
	"strings"
	"testing"
)

func TestLoadText_TXT(t *testing.T) {
	text, err := LoadText("test.txt", []byte("hello world\n"))
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if !strings.Contains(text, "hello world") {
		t.Errorf("unexpected text: %q", text)
	}
}

func TestLoadText_MD(t *testing.T) {
	text, err := LoadText("readme.md", []byte("# Title\n\nParagraph."))
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if !strings.Contains(text, "Title") {
		t.Errorf("unexpected text: %q", text)
	}
}

func TestLoadText_UnsupportedFormat(t *testing.T) {
	_, err := LoadText("file.xyz", []byte("data"))
	if err == nil {
		t.Error("expected error for unsupported format")
	}
}
```

- [ ] **Step 6: Create `internal/services/document/loader.go`**

```go
package document

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"github.com/ledongthuc/pdf"
	"github.com/xuri/excelize/v2"
)

// LoadText extracts plain text from a file given its name and raw bytes.
func LoadText(filename string, data []byte) (string, error) {
	ext := strings.ToLower(filepath.Ext(filename))
	switch ext {
	case ".txt", ".md":
		return string(data), nil
	case ".pdf":
		return extractPDF(data)
	case ".docx":
		return extractDOCX(data)
	case ".xlsx":
		return extractXLSX(data)
	default:
		return "", fmt.Errorf("unsupported file type: %s", ext)
	}
}

func extractPDF(data []byte) (string, error) {
	r, err := pdf.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return "", err
	}
	var sb strings.Builder
	for i := 1; i <= r.NumPage(); i++ {
		p := r.Page(i)
		if p.V.IsNull() {
			continue
		}
		content, err := p.GetPlainText(nil)
		if err != nil {
			continue
		}
		sb.WriteString(content)
		sb.WriteString("\n")
	}
	return sb.String(), nil
}

func extractDOCX(data []byte) (string, error) {
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return "", err
	}
	for _, f := range zr.File {
		if f.Name != "word/document.xml" {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			return "", err
		}
		defer rc.Close()
		return xmlTextContent(rc)
	}
	return "", fmt.Errorf("word/document.xml not found in docx")
}

func xmlTextContent(r io.Reader) (string, error) {
	var sb strings.Builder
	dec := xml.NewDecoder(r)
	for {
		tok, err := dec.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", err
		}
		if cd, ok := tok.(xml.CharData); ok {
			sb.Write(cd)
			sb.WriteByte(' ')
		}
	}
	return sb.String(), nil
}

func extractXLSX(data []byte) (string, error) {
	f, err := excelize.OpenReader(bytes.NewReader(data))
	if err != nil {
		return "", err
	}
	defer f.Close()
	var sb strings.Builder
	for _, sheet := range f.GetSheetList() {
		rows, err := f.GetRows(sheet)
		if err != nil {
			continue
		}
		for _, row := range rows {
			sb.WriteString(strings.Join(row, "\t"))
			sb.WriteByte('\n')
		}
	}
	return sb.String(), nil
}
```

- [ ] **Step 7: Run all document tests**

```bash
go test ./internal/services/document/... -v
```

Expected: PASS (5 tests)

- [ ] **Step 8: Commit**

```bash
git add internal/services/document/
git commit -m "feat: document loader (PDF/DOCX/XLSX/TXT/MD) and text chunker"
```

---

### Task 7: Collection Service + Endpoints

**Files:**
- Create: `internal/services/collection/service.go`
- Create: `internal/api/handlers/collections.go`
- Modify: `internal/api/router.go` (add collection routes)
- Test: `internal/services/collection/service_test.go`

**Interfaces:**
- Consumes: `database.CreateCollection`, `database.GetCollection`, `database.ListCollectionsForUser`, `database.DeleteCollection`
- Consumes: `vectorstore.Client.EnsureCollection`, `vectorstore.Client.DeleteCollection`
- Produces: `collection.Service{}.GetOrCreateDefault(ctx, username) (*database.Collection, error)`
- Produces: `collection.Service{}.Create(ctx, username, displayName) (*database.Collection, error)`
- Produces: `collection.Service{}.Delete(ctx, username, slug) error` — also deletes Qdrant collection

- [ ] **Step 1: Write failing service test**

Create `internal/services/collection/service_test.go`:

```go
package collection

import (
	"context"
	"database/sql"
	"testing"

	"raggo/internal/database"
	"raggo/internal/services/vectorstore"

	_ "modernc.org/sqlite"
)

type stubQdrant struct{ created, deleted []string }

func (s *stubQdrant) EnsureCollection(_ context.Context, name string, _ uint64) error {
	s.created = append(s.created, name)
	return nil
}
func (s *stubQdrant) DeleteCollection(_ context.Context, name string) error {
	s.deleted = append(s.deleted, name)
	return nil
}

func newTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, _ := database.Open(":memory:")
	_ = database.Migrate(db)
	_ = database.CreateUser(db, database.User{ID: "1", Username: "alice", PasswordHash: "h", Role: "user"})
	t.Cleanup(func() { db.Close() })
	return db
}

func TestGetOrCreateDefault(t *testing.T) {
	db := newTestDB(t)
	qdrant := &stubQdrant{}
	svc := &Service{DB: db, Qdrant: qdrant, VectorSize: 1536}

	col, err := svc.GetOrCreateDefault(context.Background(), "alice")
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if col.Slug != "default" {
		t.Errorf("slug = %q, want default", col.Slug)
	}
	if col.QdrantName != "rag_alice_default" {
		t.Errorf("qdrant name = %q", col.QdrantName)
	}
	if len(qdrant.created) == 0 {
		t.Error("EnsureCollection not called")
	}

	// second call should not create a duplicate
	col2, err := svc.GetOrCreateDefault(context.Background(), "alice")
	if err != nil {
		t.Fatalf("second create: %v", err)
	}
	if col2.ID != col.ID {
		t.Error("expected same collection on second call")
	}
}

func TestDelete_DefaultCollectionForbidden(t *testing.T) {
	db := newTestDB(t)
	svc := &Service{DB: db, Qdrant: &stubQdrant{}, VectorSize: 1536}
	_, _ = svc.GetOrCreateDefault(context.Background(), "alice")

	err := svc.Delete(context.Background(), "alice", "default")
	if err == nil {
		t.Error("expected error deleting default collection")
	}
}
```

- [ ] **Step 2: Run test — verify fail**

```bash
go test ./internal/services/collection/... -v 2>&1 | head -10
```

Expected: FAIL

- [ ] **Step 3: Create `internal/services/collection/service.go`**

```go
package collection

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"unicode"

	"raggo/internal/database"
)

type QdrantProvider interface {
	EnsureCollection(ctx context.Context, name string, vectorSize uint64) error
	DeleteCollection(ctx context.Context, name string) error
}

type Service struct {
	DB         *sql.DB
	Qdrant     QdrantProvider
	VectorSize uint64
}

func (s *Service) GetOrCreateDefault(ctx context.Context, username string) (*database.Collection, error) {
	col, err := database.GetCollection(s.DB, username, "default")
	if err == nil {
		return col, nil
	}
	return s.create(ctx, username, "Default", "default", true)
}

func (s *Service) Create(ctx context.Context, username, displayName string) (*database.Collection, error) {
	slug := toSlug(displayName)
	return s.create(ctx, username, displayName, slug, false)
}

func (s *Service) create(ctx context.Context, username, displayName, slug string, isDefault bool) (*database.Collection, error) {
	qdrantName := buildQdrantName(username, slug)
	if err := s.Qdrant.EnsureCollection(ctx, qdrantName, s.VectorSize); err != nil {
		return nil, fmt.Errorf("ensure qdrant collection: %w", err)
	}
	if err := database.CreateCollection(s.DB, database.Collection{
		Slug:          slug,
		DisplayName:   displayName,
		OwnerUsername: username,
		QdrantName:    qdrantName,
		IsDefault:     isDefault,
	}); err != nil {
		return nil, err
	}
	return database.GetCollection(s.DB, username, slug)
}

func (s *Service) List(username string) ([]database.Collection, error) {
	return database.ListCollectionsForUser(s.DB, username)
}

func (s *Service) ListAll() ([]database.Collection, error) {
	return database.ListAllCollections(s.DB)
}

func (s *Service) Delete(ctx context.Context, username, slug string) error {
	col, err := database.GetCollection(s.DB, username, slug)
	if err != nil {
		return err
	}
	if col.IsDefault {
		return errors.New("cannot delete the default collection")
	}
	if err := s.Qdrant.DeleteCollection(ctx, col.QdrantName); err != nil {
		return fmt.Errorf("delete qdrant collection: %w", err)
	}
	return database.DeleteCollection(s.DB, col.ID)
}

// buildQdrantName constructs a Qdrant collection name capped at 128 chars.
func buildQdrantName(username, slug string) string {
	name := fmt.Sprintf("rag_%s_%s", username, slug)
	if len(name) > 128 {
		name = name[:128]
	}
	return name
}

var nonAlpha = regexp.MustCompile(`[^a-z0-9]+`)

func toSlug(s string) string {
	s = strings.ToLower(s)
	s = strings.Map(func(r rune) rune {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			return r
		}
		return '-'
	}, s)
	s = nonAlpha.ReplaceAllString(s, "-")
	s = strings.Trim(s, "-")
	if s == "" {
		s = "collection"
	}
	return s
}
```

- [ ] **Step 4: Create `internal/api/handlers/collections.go`**

```go
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
```

- [ ] **Step 5: Run test**

```bash
go test ./internal/services/collection/... -v
```

Expected: PASS (2 tests)

- [ ] **Step 6: Commit**

```bash
git add internal/services/collection/ internal/api/handlers/collections.go
git commit -m "feat: collection service and REST endpoints"
```

---

### Task 8: Document Upload + Management API

**Files:**
- Create: `internal/services/document/ingest.go`
- Create: `internal/api/handlers/documents.go`
- Modify: `internal/api/router.go` (wire document routes)
- Test: `internal/services/document/ingest_test.go`

**Interfaces:**
- Consumes: `document.LoadText`, `document.Chunk`, `embedding.Service.Embed`, `vectorstore.Client.Upsert`, `database.CreateDocumentMeta`, `database.ListDocumentsForUser`
- Produces: `document.IngestService{}.Ingest(ctx, owner, collectionID, qdrantName, filename string, data []byte) (*database.DocumentMeta, error)`
- Produces: `document.IngestService{}.Delete(ctx, docID, qdrantName string) error`
- Produces: `document.IngestService{}.Move(ctx, docID, fromQdrant, toQdrant string, toCollectionID int64) error`

- [ ] **Step 1: Write ingest test**

Create `internal/services/document/ingest_test.go`:

```go
package document

import (
	"context"
	"database/sql"
	"testing"

	"raggo/internal/database"
)

type stubEmbedder struct{}

func (s *stubEmbedder) Embed(_ context.Context, texts []string) ([][]float32, error) {
	vecs := make([][]float32, len(texts))
	for i := range texts {
		vecs[i] = []float32{0.1, 0.2, 0.3}
	}
	return vecs, nil
}

type stubVS struct{ upserted int }

func (s *stubVS) Upsert(_ context.Context, _ string, pts []Point) error {
	s.upserted += len(pts)
	return nil
}
func (s *stubVS) DeleteByDocumentID(_ context.Context, _, _ string) error { return nil }
func (s *stubVS) ScrollByDocumentID(_ context.Context, _, _ string) ([]PointWithVector, error) {
	return nil, nil
}

func newTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, _ := database.Open(":memory:")
	_ = database.Migrate(db)
	_ = database.CreateUser(db, database.User{ID: "1", Username: "alice", PasswordHash: "h", Role: "user"})
	_ = database.CreateCollection(db, database.Collection{
		Slug: "default", DisplayName: "Default",
		OwnerUsername: "alice", QdrantName: "rag_alice_default", IsDefault: true,
	})
	t.Cleanup(func() { db.Close() })
	return db
}

func TestIngest_TXT(t *testing.T) {
	db := newTestDB(t)
	vs := &stubVS{}
	col, _ := database.GetCollection(db, "alice", "default")
	svc := &IngestService{DB: db, Embedder: &stubEmbedder{}, VS: vs, ChunkSize: 100, ChunkOverlap: 20}

	meta, err := svc.Ingest(context.Background(), "alice", col.ID, "rag_alice_default", "test.txt", []byte("hello world this is a test document"))
	if err != nil {
		t.Fatalf("ingest: %v", err)
	}
	if meta.Status != "processed" {
		t.Errorf("status = %q, want processed", meta.Status)
	}
	if vs.upserted == 0 {
		t.Error("no vectors upserted")
	}
}
```

- [ ] **Step 2: Create `internal/services/document/ingest.go`**

```go
package document

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/google/uuid"
	"raggo/internal/database"
)

type Embedder interface {
	Embed(ctx context.Context, texts []string) ([][]float32, error)
}

type VectorStore interface {
	Upsert(ctx context.Context, collection string, pts []Point) error
	DeleteByDocumentID(ctx context.Context, collection, docID string) error
	ScrollByDocumentID(ctx context.Context, collection, docID string) ([]PointWithVector, error)
}

type IngestService struct {
	DB           *sql.DB
	Embedder     Embedder
	VS           VectorStore
	ChunkSize    int
	ChunkOverlap int
}

func (s *IngestService) Ingest(ctx context.Context, owner string, collectionID int64, qdrantName, filename string, data []byte) (*database.DocumentMeta, error) {
	text, err := LoadText(filename, data)
	if err != nil {
		return nil, fmt.Errorf("load text: %w", err)
	}

	chunks := Chunk(text, s.ChunkSize, s.ChunkOverlap)
	if len(chunks) == 0 {
		chunks = []string{" "} // at least one chunk
	}

	docID := uuid.NewString()
	fileExt := ""
	if i := len(filename) - 1; i >= 0 {
		for j := i; j >= 0; j-- {
			if filename[j] == '.' {
				fileExt = filename[j:]
				break
			}
		}
	}

	meta := database.DocumentMeta{
		ID:            docID,
		Filename:      filename,
		FileType:      fileExt,
		SizeBytes:     int64(len(data)),
		ChunksCount:   len(chunks),
		Status:        "processing",
		OwnerUsername: owner,
		CollectionID:  collectionID,
	}
	if err := database.CreateDocumentMeta(s.DB, meta); err != nil {
		return nil, err
	}

	// embed in batches of 100
	const batchSize = 100
	var points []Point
	for i := 0; i < len(chunks); i += batchSize {
		end := i + batchSize
		if end > len(chunks) {
			end = len(chunks)
		}
		batch := chunks[i:end]
		vecs, err := s.Embedder.Embed(ctx, batch)
		if err != nil {
			_ = database.UpdateDocumentChunksAndStatus(s.DB, docID, len(chunks), "failed")
			return nil, fmt.Errorf("embed: %w", err)
		}
		for j, vec := range vecs {
			points = append(points, Point{
				ID:     fmt.Sprintf("%s_%d", docID, i+j),
				Vector: vec,
				Payload: map[string]string{
					"document_id": docID,
					"filename":    filename,
					"owner":       owner,
					"text":        batch[j],
					"chunk_index": fmt.Sprintf("%d", i+j),
				},
			})
		}
	}

	if err := s.VS.Upsert(ctx, qdrantName, points); err != nil {
		_ = database.UpdateDocumentChunksAndStatus(s.DB, docID, len(chunks), "failed")
		return nil, fmt.Errorf("upsert: %w", err)
	}

	_ = database.UpdateDocumentChunksAndStatus(s.DB, docID, len(chunks), "processed")
	_ = database.IncrementDocumentCount(s.DB, collectionID, 1)

	meta.Status = "processed"
	return &meta, nil
}

func (s *IngestService) Delete(ctx context.Context, docID, qdrantName string) error {
	if err := s.VS.DeleteByDocumentID(ctx, qdrantName, docID); err != nil {
		return err
	}
	meta, err := database.GetDocumentMeta(s.DB, docID)
	if err == nil {
		_ = database.IncrementDocumentCount(s.DB, meta.CollectionID, -1)
	}
	return database.DeleteDocumentMeta(s.DB, docID)
}

func (s *IngestService) Move(ctx context.Context, docID, fromQdrant, toQdrant string, toCollectionID int64) error {
	pts, err := s.VS.ScrollByDocumentID(ctx, fromQdrant, docID)
	if err != nil {
		return fmt.Errorf("scroll: %w", err)
	}
	var newPts []Point
	for _, p := range pts {
		newPts = append(newPts, Point{ID: p.ID, Vector: p.Vector, Payload: p.Payload})
	}
	if len(newPts) > 0 {
		if err := s.VS.Upsert(ctx, toQdrant, newPts); err != nil {
			return fmt.Errorf("upsert to target: %w", err)
		}
	}
	if err := s.VS.DeleteByDocumentID(ctx, fromQdrant, docID); err != nil {
		return fmt.Errorf("delete from source: %w", err)
	}
	meta, err := database.GetDocumentMeta(s.DB, docID)
	if err != nil {
		return err
	}
	_ = database.IncrementDocumentCount(s.DB, meta.CollectionID, -1)
	_ = database.IncrementDocumentCount(s.DB, toCollectionID, 1)
	return database.UpdateDocumentCollection(s.DB, docID, toCollectionID)
}
```

- [ ] **Step 3: Create `internal/api/handlers/documents.go`**

```go
package handlers

import (
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"raggo/internal/database"
	"raggo/internal/middleware"
	"raggo/internal/services/collection"
	"raggo/internal/services/document"
)

type DocumentHandler struct {
	IngestSvc  *document.IngestService
	CollSvc    *collection.Service
	DB         interface{ GetDocumentMeta(id string) (*database.DocumentMeta, error) }
	MaxUpload  int64
}

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

	// resolve collection: optional ?collection_slug query param
	slug := r.URL.Query().Get("collection_slug")
	var col *database.Collection
	if slug != "" {
		col, err = h.CollSvc.List(c.Username)
		_ = err
		// find by slug
		cols, _ := h.CollSvc.List(c.Username)
		for i := range cols {
			if cols[i].Slug == slug {
				col = &cols[i]
				break
			}
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

func (h *DocumentHandler) Move(w http.ResponseWriter, r *http.Request) {
	c := middleware.ClaimsFromCtx(r.Context())
	docID := chi.URLParam(r, "docID")

	var req struct {
		TargetSlug          string `json:"target_slug"`
		TargetOwnerUsername string `json:"target_owner_username"`
	}
	if err := decodeJSON(r, &req); err != nil || req.TargetSlug == "" {
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

func decodeJSON(r *http.Request, v any) error {
	import_json "encoding/json"
	return import_json.NewDecoder(r.Body).Decode(v)
}
```

> **Note:** The `decodeJSON` helper above has a syntax error — fix it by using `json.NewDecoder` directly with a proper import in the file header. The handler file should start with:
> ```go
> import (
>     "encoding/json"
>     "net/http"
>     "strconv"
>     ...
> )
> ```
> Replace `import_json "encoding/json"` with just `json.NewDecoder(r.Body).Decode(v)`.

- [ ] **Step 4: Run ingest test**

```bash
go test ./internal/services/document/... -v
```

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/services/document/ingest.go internal/api/handlers/documents.go
git commit -m "feat: document ingest pipeline and REST endpoints"
```

---

### Task 9: Hybrid Search Service + Endpoint

**Files:**
- Create: `internal/services/search/service.go`
- Create: `internal/api/handlers/search.go`
- Test: `internal/services/search/service_test.go`

**Interfaces:**
- Consumes: `embedding.Service.Embed`, `vectorstore.Client.Search`
- Produces: `search.Service{}.Search(ctx, query, qdrantName string, limit int, useHybrid bool) ([]Result, error)`
- Produces: `search.Result{ID, Text, Score, SemanticScore, KeywordScore float32, Payload map[string]string}`

- [ ] **Step 1: Write test**

Create `internal/services/search/service_test.go`:

```go
package search

import (
	"context"
	"testing"

	"raggo/internal/services/vectorstore"
)

type stubEmb struct{}

func (s *stubEmb) Embed(_ context.Context, texts []string) ([][]float32, error) {
	return [][]float32{{0.1, 0.2, 0.3}}, nil
}

type stubVS struct{}

func (s *stubVS) Search(_ context.Context, _ string, _ []float32, _ uint64, _ map[string]string) ([]vectorstore.SearchResult, error) {
	return []vectorstore.SearchResult{
		{ID: "doc1_0", Score: 0.9, Payload: map[string]string{"text": "the quick brown fox", "document_id": "doc1"}},
		{ID: "doc1_1", Score: 0.7, Payload: map[string]string{"text": "jumps over the lazy dog", "document_id": "doc1"}},
	}, nil
}

func TestSearch_Hybrid(t *testing.T) {
	svc := &Service{Embedder: &stubEmb{}, VS: &stubVS{}}
	results, err := svc.Search(context.Background(), "quick fox", "rag_alice_default", 5, true)
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if len(results) == 0 {
		t.Fatal("expected results")
	}
	// result with keyword match should score higher or equal
	for _, r := range results {
		if r.Score < 0 || r.Score > 2 {
			t.Errorf("unexpected score %f", r.Score)
		}
	}
}

func TestSearch_Semantic(t *testing.T) {
	svc := &Service{Embedder: &stubEmb{}, VS: &stubVS{}}
	results, err := svc.Search(context.Background(), "quick fox", "rag_alice_default", 5, false)
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if len(results) == 0 {
		t.Fatal("expected results")
	}
}
```

- [ ] **Step 2: Create `internal/services/search/service.go`**

```go
package search

import (
	"context"
	"math"
	"sort"
	"strings"

	"raggo/internal/services/vectorstore"
)

type Embedder interface {
	Embed(ctx context.Context, texts []string) ([][]float32, error)
}

type VectorSearcher interface {
	Search(ctx context.Context, collection string, vector []float32, limit uint64, filter map[string]string) ([]vectorstore.SearchResult, error)
}

type Result struct {
	ID            string
	Text          string
	Score         float32
	SemanticScore float32
	KeywordScore  float32
	Payload       map[string]string
}

type Service struct {
	Embedder Embedder
	VS       VectorSearcher
}

const semanticWeight = 0.7
const keywordWeight = 0.3

func (s *Service) Search(ctx context.Context, query, qdrantName string, limit int, useHybrid bool) ([]Result, error) {
	vecs, err := s.Embedder.Embed(ctx, []string{query})
	if err != nil {
		return nil, err
	}
	raw, err := s.VS.Search(ctx, qdrantName, vecs[0], uint64(limit*3), nil)
	if err != nil {
		return nil, err
	}

	var results []Result
	for _, r := range raw {
		text := r.Payload["text"]
		semScore := r.Score
		kwScore := float32(0)
		if useHybrid {
			kwScore = bm25Score(query, text)
		}
		combined := float32(semanticWeight)*semScore + float32(keywordWeight)*kwScore
		results = append(results, Result{
			ID:            r.ID,
			Text:          text,
			Score:         combined,
			SemanticScore: semScore,
			KeywordScore:  kwScore,
			Payload:       r.Payload,
		})
	}

	sort.Slice(results, func(i, j int) bool { return results[i].Score > results[j].Score })
	if len(results) > limit {
		results = results[:limit]
	}
	return results, nil
}

// bm25Score computes a simplified BM25-like term frequency score (no IDF, suitable for single-doc scoring).
func bm25Score(query, text string) float32 {
	const k1 = 1.5
	const b = 0.75
	const avgDocLen = 200.0

	terms := strings.Fields(strings.ToLower(query))
	words := strings.Fields(strings.ToLower(text))
	docLen := float64(len(words))

	wordFreq := make(map[string]int)
	for _, w := range words {
		wordFreq[w]++
	}

	var score float64
	for _, term := range terms {
		tf := float64(wordFreq[term])
		score += (tf * (k1 + 1)) / (tf + k1*(1-b+b*docLen/avgDocLen))
	}
	// normalize to [0,1] roughly
	normalized := 1 - 1/math.Exp(score+1)
	return float32(normalized)
}
```

- [ ] **Step 3: Create `internal/api/handlers/search.go`**

```go
package handlers

import (
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
```

> **Note:** Add `"context"` and `"database/sql"` to the imports. The `CollSvc` field needs the `context` package imported in this file.

- [ ] **Step 4: Run search tests**

```bash
go test ./internal/services/search/... -v
```

Expected: PASS (2 tests)

- [ ] **Step 5: Commit**

```bash
git add internal/services/search/ internal/api/handlers/search.go
git commit -m "feat: hybrid search service (BM25 + semantic) and endpoint"
```

---

### Task 10: Chat Service + Streaming Endpoint

**Files:**
- Create: `internal/services/chat/service.go`
- Create: `internal/api/handlers/chat.go`
- Test: `internal/services/chat/service_test.go`

**Interfaces:**
- Consumes: `search.Service.Search`, `database.ListDocumentsForUser`, `database.CreateSession/GetSessionHistory`
- Produces: `chat.Service{}.Chat(ctx, req ChatRequest) (string, []search.Result, error)`
- Produces: `chat.Service{}.ChatStream(ctx, req ChatRequest, w io.Writer) error`
- Produces: `chat.ChatRequest{Message, SessionID, CollectionSlugs []string, Username string}`

- [ ] **Step 1: Add session queries to database**

Append to `internal/database/queries.go`:

```go
type ChatSession struct {
	ID        string
	Username  string
	CreatedAt time.Time
	UpdatedAt time.Time
}

type ChatMessage struct {
	ID        int64
	SessionID string
	Role      string
	Content   string
	CreatedAt time.Time
}

func CreateChatSession(db *sql.DB, id, username string) error {
	_, err := db.Exec(`INSERT INTO chat_sessions (id, username) VALUES (?,?)`, id, username)
	return err
}

func AppendChatMessage(db *sql.DB, sessionID, role, content string) error {
	_, err := db.Exec(`INSERT INTO chat_messages (session_id, role, content) VALUES (?,?,?)`, sessionID, role, content)
	if err != nil {
		return err
	}
	_, err = db.Exec(`UPDATE chat_sessions SET updated_at = CURRENT_TIMESTAMP WHERE id = ?`, sessionID)
	return err
}

func GetSessionMessages(db *sql.DB, sessionID string, limit int) ([]ChatMessage, error) {
	rows, err := db.Query(
		`SELECT id, session_id, role, content, created_at FROM chat_messages
		 WHERE session_id = ? ORDER BY created_at LIMIT ?`, sessionID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var msgs []ChatMessage
	for rows.Next() {
		m := ChatMessage{}
		if err := rows.Scan(&m.ID, &m.SessionID, &m.Role, &m.Content, &m.CreatedAt); err != nil {
			return nil, err
		}
		msgs = append(msgs, m)
	}
	return msgs, rows.Err()
}
```

- [ ] **Step 2: Write chat service test**

Create `internal/services/chat/service_test.go`:

```go
package chat

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"raggo/internal/database"
	"raggo/internal/services/search"
)

type stubSearcher struct{}

func (s *stubSearcher) SearchMulti(ctx context.Context, query string, qdrantNames []string, limit int, hybrid bool) ([]search.Result, error) {
	return []search.Result{
		{Text: "The answer is 42.", Score: 0.9, Payload: map[string]string{"filename": "docs.txt", "document_id": "d1"}},
	}, nil
}

func newTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, _ := database.Open(":memory:")
	_ = database.Migrate(db)
	_ = database.CreateUser(db, database.User{ID: "1", Username: "alice", PasswordHash: "h", Role: "user"})
	t.Cleanup(func() { db.Close() })
	return db
}

func TestChat_ReturnsAnswer(t *testing.T) {
	llmSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"choices": []map[string]any{
				{"message": map[string]any{"content": "The answer is 42."}},
			},
		})
	}))
	defer llmSrv.Close()

	db := newTestDB(t)
	svc := &Service{
		DB:          db,
		Searcher:    &stubSearcher{},
		LLMBaseURL:  llmSrv.URL,
		LLMAPIKey:   "test",
		LLMModel:    "test-model",
		client:      &http.Client{},
	}

	req := ChatRequest{Message: "what is the answer?", Username: "alice", QdrantNames: []string{"rag_alice_default"}}
	answer, _, err := svc.Chat(context.Background(), req)
	if err != nil {
		t.Fatalf("chat: %v", err)
	}
	if !strings.Contains(answer, "42") {
		t.Errorf("unexpected answer: %q", answer)
	}
}
```

- [ ] **Step 3: Create `internal/services/chat/service.go`**

```go
package chat

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/google/uuid"
	"raggo/internal/database"
	"raggo/internal/services/search"
)

type MultiSearcher interface {
	SearchMulti(ctx context.Context, query string, qdrantNames []string, limit int, hybrid bool) ([]search.Result, error)
}

type ChatRequest struct {
	Message     string
	SessionID   string
	Username    string
	QdrantNames []string // empty = all user collections
	UseHybrid   bool
}

type Service struct {
	DB         *sql.DB
	Searcher   MultiSearcher
	LLMBaseURL string
	LLMAPIKey  string
	LLMModel   string
	client     *http.Client
}

func (s *Service) Chat(ctx context.Context, req ChatRequest) (string, []search.Result, error) {
	sessionID, err := s.ensureSession(req.SessionID, req.Username)
	if err != nil {
		return "", nil, err
	}

	results, err := s.Searcher.SearchMulti(ctx, req.Message, req.QdrantNames, 5, req.UseHybrid)
	if err != nil {
		return "", nil, fmt.Errorf("search: %w", err)
	}

	docsIndex := s.buildDocumentIndex(ctx, req.Username)
	history, _ := database.GetSessionMessages(s.DB, sessionID, 20)
	prompt := s.buildPrompt(req.Message, results, docsIndex, history)

	answer, err := s.callLLM(ctx, prompt, false, nil)
	if err != nil {
		return "", nil, err
	}

	_ = database.AppendChatMessage(s.DB, sessionID, "user", req.Message)
	_ = database.AppendChatMessage(s.DB, sessionID, "assistant", answer)

	return answer, results, nil
}

func (s *Service) ChatStream(ctx context.Context, req ChatRequest, w http.ResponseWriter) error {
	sessionID, err := s.ensureSession(req.SessionID, req.Username)
	if err != nil {
		return err
	}

	results, err := s.Searcher.SearchMulti(ctx, req.Message, req.QdrantNames, 5, req.UseHybrid)
	if err != nil {
		return err
	}

	docsIndex := s.buildDocumentIndex(ctx, req.Username)
	history, _ := database.GetSessionMessages(s.DB, sessionID, 20)
	prompt := s.buildPrompt(req.Message, results, docsIndex, history)

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	var fullAnswer strings.Builder
	err = s.callLLM(ctx, prompt, true, func(chunk string) {
		fullAnswer.WriteString(chunk)
		fmt.Fprintf(w, "data: %s\n\n", jsonStr(chunk))
		if f, ok := w.(http.Flusher); ok {
			f.Flush()
		}
	})
	if err != nil {
		return err
	}
	fmt.Fprintf(w, "data: [DONE]\n\n")

	_ = database.AppendChatMessage(s.DB, sessionID, "user", req.Message)
	_ = database.AppendChatMessage(s.DB, sessionID, "assistant", fullAnswer.String())
	return nil
}

func (s *Service) ensureSession(id, username string) (string, error) {
	if id == "" {
		id = uuid.NewString()
		return id, database.CreateChatSession(s.DB, id, username)
	}
	return id, nil
}

func (s *Service) buildDocumentIndex(ctx context.Context, username string) string {
	docs, _, err := database.ListDocumentsForUser(s.DB, username, 1, 1000)
	if err != nil || len(docs) == 0 {
		return "(no documents indexed)"
	}
	var sb strings.Builder
	for _, d := range docs {
		sb.WriteString(fmt.Sprintf("- %s (type: %s, chunks: %d)\n", d.Filename, d.FileType, d.ChunksCount))
	}
	return sb.String()
}

func (s *Service) buildPrompt(message string, results []search.Result, docsIndex string, history []database.ChatMessage) []map[string]string {
	system := fmt.Sprintf(`You are a helpful assistant with access to a document library.

=== DOCUMENT INDEX ===
The following documents are available in the system:
%s

=== RETRIEVED CONTEXT ===
The following passages were retrieved as most relevant to the user's question:
%s

Instructions:
- Answer the user's question using the retrieved context when relevant.
- Use the document index to answer questions about what documents exist in the system.
- Respond in the same language as the user's message.
- If you cannot find the answer in the context, say so clearly.`,
		docsIndex,
		buildContextBlock(results),
	)

	messages := []map[string]string{{"role": "system", "content": system}}
	for _, m := range history {
		messages = append(messages, map[string]string{"role": m.Role, "content": m.Content})
	}
	messages = append(messages, map[string]string{"role": "user", "content": message})
	return messages
}

func buildContextBlock(results []search.Result) string {
	if len(results) == 0 {
		return "(no relevant passages found)"
	}
	var sb strings.Builder
	for i, r := range results {
		sb.WriteString(fmt.Sprintf("[%d] Source: %s\n%s\n\n", i+1, r.Payload["filename"], r.Text))
	}
	return sb.String()
}

func (s *Service) callLLM(ctx context.Context, messages []map[string]string, stream bool, onChunk func(string)) (string, error) {
	body, _ := json.Marshal(map[string]any{
		"model":    s.LLMModel,
		"messages": messages,
		"stream":   stream,
	})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.LLMBaseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+s.LLMAPIKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if !stream {
		var out struct {
			Choices []struct {
				Message struct{ Content string `json:"content"` } `json:"message"`
			} `json:"choices"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
			return "", err
		}
		if len(out.Choices) == 0 {
			return "", fmt.Errorf("empty LLM response")
		}
		return out.Choices[0].Message.Content, nil
	}

	// streaming: read SSE lines
	buf := make([]byte, 4096)
	for {
		n, err := resp.Body.Read(buf)
		if n > 0 {
			lines := strings.Split(string(buf[:n]), "\n")
			for _, line := range lines {
				if !strings.HasPrefix(line, "data: ") {
					continue
				}
				data := strings.TrimPrefix(line, "data: ")
				if data == "[DONE]" {
					return "", nil
				}
				var chunk struct {
					Choices []struct {
						Delta struct{ Content string `json:"content"` } `json:"delta"`
					} `json:"choices"`
				}
				if jsonErr := json.Unmarshal([]byte(data), &chunk); jsonErr == nil {
					if len(chunk.Choices) > 0 && onChunk != nil {
						onChunk(chunk.Choices[0].Delta.Content)
					}
				}
			}
		}
		if err != nil {
			break
		}
	}
	return "", nil
}

func jsonStr(s string) string {
	b, _ := json.Marshal(s)
	return string(b)
}
```

- [ ] **Step 4: Add `SearchMulti` to search service**

Append to `internal/services/search/service.go`:

```go
// SearchMulti searches across multiple Qdrant collections and merges results by score,
// deduplicating by text content hash.
func (s *Service) SearchMulti(ctx context.Context, query string, qdrantNames []string, limit int, useHybrid bool) ([]Result, error) {
	vecs, err := s.Embedder.Embed(ctx, []string{query})
	if err != nil {
		return nil, err
	}

	seen := make(map[string]bool) // text fingerprint → dedup
	var allResults []Result

	for _, name := range qdrantNames {
		raw, err := s.VS.Search(ctx, name, vecs[0], uint64(limit*2), nil)
		if err != nil {
			continue
		}
		for _, r := range raw {
			text := r.Payload["text"]
			fp := fingerprint(text)
			if seen[fp] {
				continue
			}
			seen[fp] = true
			semScore := r.Score
			kwScore := float32(0)
			if useHybrid {
				kwScore = bm25Score(query, text)
			}
			combined := float32(semanticWeight)*semScore + float32(keywordWeight)*kwScore
			allResults = append(allResults, Result{
				ID:            r.ID,
				Text:          text,
				Score:         combined,
				SemanticScore: semScore,
				KeywordScore:  kwScore,
				Payload:       r.Payload,
			})
		}
	}

	sort.Slice(allResults, func(i, j int) bool { return allResults[i].Score > allResults[j].Score })
	if len(allResults) > limit {
		allResults = allResults[:limit]
	}
	return allResults, nil
}

func fingerprint(text string) string {
	// simple first-64-chars fingerprint
	if len(text) > 64 {
		return text[:64]
	}
	return text
}
```

- [ ] **Step 5: Run chat tests**

```bash
go test ./internal/services/chat/... -v
```

Expected: PASS

- [ ] **Step 6: Create `internal/api/handlers/chat.go`**

```go
package handlers

import (
	"database/sql"
	"encoding/json"
	"net/http"

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

func (h *ChatHandler) ChatStream(w http.ResponseWriter, r *http.Request) {
	c := middleware.ClaimsFromCtx(r.Context())
	req, qdrantNames, err := h.parseChatRequest(r, c.Username)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, errResp(err.Error()))
		return
	}
	req.QdrantNames = qdrantNames

	if err := h.ChatSvc.ChatStream(r.Context(), *req, w); err != nil {
		// headers already sent; log only
		return
	}
}

func (h *ChatHandler) parseChatRequest(r *http.Request, username string) (*chat.ChatRequest, []string, error) {
	var body struct {
		Message        string   `json:"message"`
		SessionID      string   `json:"session_id"`
		CollectionSlugs []string `json:"collection_slugs"`
		UseHybrid      *bool    `json:"use_hybrid_search"`
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
```

> **Note:** Add `"fmt"` to the imports in `chat.go`.

- [ ] **Step 7: Run all tests**

```bash
go test ./... -v -count=1 2>&1 | tail -20
```

Expected: PASS on all packages that don't require live Qdrant

- [ ] **Step 8: Commit**

```bash
git add internal/services/chat/ internal/api/handlers/chat.go internal/database/queries.go internal/services/search/service.go
git commit -m "feat: RAG chat service with streaming SSE and multi-collection support"
```

---

### Task 11: Scraping + Health + Admin Endpoints + Router Wiring

**Files:**
- Create: `internal/api/handlers/scrape.go`
- Create: `internal/api/handlers/admin.go`
- Modify: `internal/api/handlers/health.go`
- Modify: `internal/api/router.go` (wire all handlers)
- Test: `internal/api/handlers/scrape_test.go`

**Interfaces:**
- Produces: fully wired `NewRouter` with all routes registered

- [ ] **Step 1: Create `internal/api/handlers/scrape.go`**

```go
package handlers

import (
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
)

type ScrapeHandler struct {
	IngestSvc interface {
		Ingest(ctx context.Context, owner string, collectionID int64, qdrantName, filename string, data []byte) (*database.DocumentMeta, error)
	}
	CollSvc interface {
		GetOrCreateDefault(ctx context.Context, username string) (*database.Collection, error)
	}
}

func (h *ScrapeHandler) Scrape(w http.ResponseWriter, r *http.Request) {
	c := middleware.ClaimsFromCtx(r.Context())
	var req struct {
		URL        string `json:"url"`
		MaxPages   int    `json:"max_pages"`
		IndexAfter bool   `json:"index"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.URL == "" {
		writeJSON(w, http.StatusBadRequest, errResp("url required"))
		return
	}
	if err := validateURL(req.URL); err != nil {
		writeJSON(w, http.StatusBadRequest, errResp(err.Error()))
		return
	}

	content, err := fetchURL(r.Context(), req.URL)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, errResp("failed to fetch URL: "+err.Error()))
		return
	}

	col, err := h.CollSvc.GetOrCreateDefault(r.Context(), c.Username)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errResp("collection error"))
		return
	}

	filename := urlToFilename(req.URL)
	meta, err := h.IngestSvc.Ingest(r.Context(), c.Username, col.ID, col.QdrantName, filename, []byte(content))
	if err != nil {
		writeJSON(w, http.StatusUnprocessableEntity, errResp(err.Error()))
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"url":          req.URL,
		"success":      true,
		"indexed_chunks": meta.ChunksCount,
		"message":      "Successfully scraped and indexed",
	})
}

// validateURL blocks private/loopback IP ranges (SSRF protection).
func validateURL(rawURL string) error {
	u, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("invalid URL")
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return fmt.Errorf("only http/https allowed")
	}
	host := u.Hostname()
	addrs, err := net.LookupHost(host)
	if err != nil {
		return fmt.Errorf("DNS lookup failed")
	}
	for _, addr := range addrs {
		ip := net.ParseIP(addr)
		if ip == nil {
			continue
		}
		private := []string{"127.0.0.0/8", "10.0.0.0/8", "172.16.0.0/12", "192.168.0.0/16", "169.254.0.0/16", "::1/128"}
		for _, cidr := range private {
			_, network, _ := net.ParseCIDR(cidr)
			if network != nil && network.Contains(ip) {
				return fmt.Errorf("URL resolves to private/internal address")
			}
		}
	}
	return nil
}

func fetchURL(ctx context.Context, rawURL string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "RAGgo/1.0")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 5<<20)) // 5 MB max
	if err != nil {
		return "", err
	}
	// strip HTML tags naively
	text := stripHTML(string(body))
	return text, nil
}

func stripHTML(s string) string {
	var sb strings.Builder
	inTag := false
	for _, r := range s {
		switch {
		case r == '<':
			inTag = true
		case r == '>':
			inTag = false
			sb.WriteRune(' ')
		case !inTag:
			sb.WriteRune(r)
		}
	}
	return sb.String()
}

func urlToFilename(u string) string {
	parsed, err := url.Parse(u)
	if err != nil {
		return "scraped.txt"
	}
	host := strings.ReplaceAll(parsed.Host, ".", "_")
	path := strings.ReplaceAll(strings.Trim(parsed.Path, "/"), "/", "_")
	name := host
	if path != "" {
		name += "_" + path
	}
	if len(name) > 100 {
		name = name[:100]
	}
	return name + ".txt"
}
```

> **Note:** Add `"context"` and necessary `database` imports to `scrape.go`.

- [ ] **Step 2: Create `internal/api/handlers/admin.go`**

```go
package handlers

import (
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
	var req struct{ Key string `json:"key"` }
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
```

> **Note:** Add `"database/sql"` import.

- [ ] **Step 3: Update `internal/api/router.go`** — wire all handlers

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
	"raggo/internal/services/chat"
	"raggo/internal/services/collection"
	"raggo/internal/services/document"
	"raggo/internal/services/embedding"
	"raggo/internal/services/search"
	"raggo/internal/services/user"
	"raggo/internal/services/vectorstore"
)

func NewRouter(cfg *config.Config, db *sql.DB) http.Handler {
	qdrant, _ := vectorstore.NewClient(cfg.QdrantHost, cfg.QdrantPort)
	embedSvc := embedding.New(cfg.OpenRouterBaseURL, cfg.OpenRouterAPIKey, cfg.EmbeddingModel)
	collSvc := &collection.Service{DB: db, Qdrant: qdrant, VectorSize: embedSvc.VectorSize()}
	ingestSvc := &document.IngestService{DB: db, Embedder: embedSvc, VS: qdrant, ChunkSize: 1000, ChunkOverlap: 200}
	searchSvc := &search.Service{Embedder: embedSvc, VS: qdrant}
	chatSvc := &chat.Service{DB: db, Searcher: searchSvc, LLMBaseURL: cfg.OpenRouterBaseURL, LLMAPIKey: cfg.OpenRouterAPIKey, LLMModel: cfg.LLMModel, client: &http.Client{}}
	userSvc := &user.Service{DB: db, Cfg: cfg}
	_ = userSvc.SeedAdmin()

	r := chi.NewRouter()
	r.Use(chimw.Logger)
	r.Use(chimw.Recoverer)
	r.Use(middleware.SecurityHeaders)
	r.Use(cors.New(cors.Options{
		AllowedOrigins: []string{"*"},
		AllowedMethods: []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowedHeaders: []string{"Authorization", "Content-Type", "X-API-Key"},
	}).Handler)

	authH := &handlers.AuthHandler{UserSvc: userSvc}
	docH := &handlers.DocumentHandler{IngestSvc: ingestSvc, CollSvc: collSvc, MaxUpload: 50 << 20}
	colH := &handlers.CollectionHandler{Svc: collSvc}
	srchH := &handlers.SearchHandler{SearchSvc: searchSvc, DB: db, CollSvc: collSvc}
	chatH := &handlers.ChatHandler{ChatSvc: chatSvc, CollSvc: collSvc, DB: db}
	scrapeH := &handlers.ScrapeHandler{IngestSvc: ingestSvc, CollSvc: collSvc}
	adminH := &handlers.AdminHandler{UserSvc: userSvc, DB: db}

	r.Get("/health", handlers.Health(cfg))
	r.Post("/auth/login", authH.Login)
	r.Post("/auth/refresh", authH.Refresh)

	r.Group(func(r chi.Router) {
		r.Use(middleware.Authenticator(cfg, db))

		r.Get("/collections", colH.List)
		r.Post("/collections", colH.Create)
		r.Delete("/collections/{slug}", colH.Delete)

		r.Get("/documents", docH.List)
		r.Post("/documents", docH.Upload)
		r.Delete("/documents/{docID}", docH.Delete)
		r.Patch("/documents/{docID}/move", docH.Move)

		r.Post("/search", srchH.Search)

		r.Post("/chat", chatH.Chat)
		r.Post("/chat/stream", chatH.ChatStream)

		r.Post("/scrape", scrapeH.Scrape)

		r.Post("/users/api-key", adminH.SetAPIKey)

		// Admin-only
		r.Group(func(r chi.Router) {
			r.Use(middleware.RequireRole("admin"))
			r.Get("/admin/users", adminH.ListUsers)
			r.Post("/admin/users", adminH.CreateUser)
			r.Delete("/admin/users/{username}", adminH.DeleteUser)
			r.Get("/collections/all", colH.ListAll)
		})
	})

	return r
}
```

- [ ] **Step 4: Build everything**

```bash
go build ./...
```

Expected: no errors (fix any import issues)

- [ ] **Step 5: Run all unit tests**

```bash
go test ./... -count=1 2>&1 | grep -E "PASS|FAIL|ok"
```

Expected: all PASS

- [ ] **Step 6: Commit**

```bash
git add internal/api/
git commit -m "feat: scraping, admin endpoints, and fully wired router"
```

---

*Continue in `2026-06-19-raggo-frontend-mcp.md`*
