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
