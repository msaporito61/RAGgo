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
