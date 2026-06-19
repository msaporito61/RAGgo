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
	if len(text) > 64 {
		return text[:64]
	}
	return text
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
