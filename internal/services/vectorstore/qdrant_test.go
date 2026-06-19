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
