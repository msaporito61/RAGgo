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
		DB:         db,
		Searcher:   &stubSearcher{},
		LLMBaseURL: llmSrv.URL,
		LLMAPIKey:  "test",
		LLMModel:   "test-model",
		client:     &http.Client{},
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

func TestChat_BuildPrompt(t *testing.T) {
	db := newTestDB(t)
	svc := &Service{
		DB:       db,
		Searcher: &stubSearcher{},
		client:   &http.Client{},
	}

	results := []search.Result{
		{Text: "The answer is 42.", Score: 0.9, Payload: map[string]string{"filename": "docs.txt"}},
	}

	messages := svc.buildPrompt("what is the answer?", results, "(no documents indexed)", nil)

	if len(messages) < 2 {
		t.Fatalf("expected at least 2 messages, got %d", len(messages))
	}
	// First message must be system
	if messages[0]["role"] != "system" {
		t.Errorf("expected first message role=system, got %q", messages[0]["role"])
	}
	// System message must contain the context block
	if !strings.Contains(messages[0]["content"], "42") {
		t.Errorf("system message does not contain retrieved context")
	}
	// Last message is the user message
	last := messages[len(messages)-1]
	if last["role"] != "user" {
		t.Errorf("expected last message role=user, got %q", last["role"])
	}
	if last["content"] != "what is the answer?" {
		t.Errorf("unexpected user message: %q", last["content"])
	}
}
