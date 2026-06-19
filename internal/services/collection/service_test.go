package collection

import (
	"context"
	"database/sql"
	"testing"

	"raggo/internal/database"

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
