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
