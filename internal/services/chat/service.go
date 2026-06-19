package chat

import (
	"bufio"
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

func New(db *sql.DB, searcher MultiSearcher, llmBaseURL, llmAPIKey, llmModel string) *Service {
	return &Service{
		DB:         db,
		Searcher:   searcher,
		LLMBaseURL: llmBaseURL,
		LLMAPIKey:  llmAPIKey,
		LLMModel:   llmModel,
		client:     &http.Client{},
	}
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
	err = s.callLLMStream(ctx, prompt, func(chunk string) {
		fullAnswer.WriteString(chunk)
		fmt.Fprintf(w, "data: %s\n\n", jsonStr(map[string]string{"delta": chunk}))
		if f, ok := w.(http.Flusher); ok {
			f.Flush()
		}
	})
	if err != nil {
		return err
	}
	fmt.Fprintf(w, "data: [DONE]\n\n")
	if f, ok := w.(http.Flusher); ok {
		f.Flush()
	}

	_ = database.AppendChatMessage(s.DB, sessionID, "user", req.Message)
	_ = database.AppendChatMessage(s.DB, sessionID, "assistant", fullAnswer.String())
	return nil
}

func (s *Service) ensureSession(id, username string) (string, error) {
	return s.EnsureSession(id, username)
}

// EnsureSession returns an existing session ID or creates a new one.
func (s *Service) EnsureSession(id, username string) (string, error) {
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

// callLLM makes a non-streaming LLM call and returns the response content.
func (s *Service) callLLM(ctx context.Context, messages []map[string]string, stream bool, _ func(string)) (string, error) {
	body, _ := json.Marshal(map[string]any{
		"model":    s.LLMModel,
		"messages": messages,
		"stream":   false,
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

	var out struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
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

// callLLMStream makes a streaming LLM call and invokes onChunk for each delta token.
func (s *Service) callLLMStream(ctx context.Context, messages []map[string]string, onChunk func(string)) error {
	body, _ := json.Marshal(map[string]any{
		"model":    s.LLMModel,
		"messages": messages,
		"stream":   true,
	})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.LLMBaseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+s.LLMAPIKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		data := strings.TrimPrefix(line, "data: ")
		if data == "[DONE]" {
			break
		}
		var chunk struct {
			Choices []struct {
				Delta struct {
					Content string `json:"content"`
				} `json:"delta"`
			} `json:"choices"`
		}
		if jsonErr := json.Unmarshal([]byte(data), &chunk); jsonErr == nil {
			if len(chunk.Choices) > 0 && onChunk != nil {
				onChunk(chunk.Choices[0].Delta.Content)
			}
		}
	}
	return scanner.Err()
}

func jsonStr(v any) string {
	b, _ := json.Marshal(v)
	return string(b)
}
