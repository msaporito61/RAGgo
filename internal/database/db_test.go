package database

import (
	"database/sql"
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
