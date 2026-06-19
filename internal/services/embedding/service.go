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
	if s.client == nil {
		s.client = &http.Client{}
	}
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
