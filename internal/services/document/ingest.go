package document

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/google/uuid"
	"raggo/internal/database"
)

// Point is a vector point to be stored in the vector store.
type Point struct {
	ID      string
	Vector  []float32
	Payload map[string]string
}

// PointWithVector is a retrieved vector point including its vector data.
type PointWithVector struct {
	ID      string
	Vector  []float32
	Payload map[string]string
}

// Embedder produces embedding vectors for a slice of texts.
type Embedder interface {
	Embed(ctx context.Context, texts []string) ([][]float32, error)
}

// VectorStore is the subset of vectorstore.Client used by IngestService.
type VectorStore interface {
	Upsert(ctx context.Context, collection string, pts []Point) error
	DeleteByDocumentID(ctx context.Context, collection, docID string) error
	ScrollByDocumentID(ctx context.Context, collection, docID string) ([]PointWithVector, error)
}

// IngestService handles the document ingestion pipeline:
// load → chunk → embed → upsert into vector store → persist metadata.
type IngestService struct {
	DB           *sql.DB
	Embedder     Embedder
	VS           VectorStore
	ChunkSize    int
	ChunkOverlap int
}

// Ingest loads, chunks, embeds, and stores a document returning its metadata.
func (s *IngestService) Ingest(ctx context.Context, owner string, collectionID int64, qdrantName, filename string, data []byte) (*database.DocumentMeta, error) {
	text, err := LoadText(filename, data)
	if err != nil {
		return nil, fmt.Errorf("load text: %w", err)
	}

	chunks := Chunk(text, s.ChunkSize, s.ChunkOverlap)
	if len(chunks) == 0 {
		chunks = []string{" "} // at least one chunk
	}

	docID := uuid.NewString()

	// Extract file extension.
	fileExt := ""
	for j := len(filename) - 1; j >= 0; j-- {
		if filename[j] == '.' {
			fileExt = filename[j:]
			break
		}
	}

	meta := database.DocumentMeta{
		ID:            docID,
		Filename:      filename,
		FileType:      fileExt,
		SizeBytes:     int64(len(data)),
		ChunksCount:   len(chunks),
		Status:        "processing",
		OwnerUsername: owner,
		CollectionID:  collectionID,
	}
	if err := database.CreateDocumentMeta(s.DB, meta); err != nil {
		return nil, err
	}

	// Embed in batches of 100 and collect points.
	const batchSize = 100
	var points []Point
	for i := 0; i < len(chunks); i += batchSize {
		end := i + batchSize
		if end > len(chunks) {
			end = len(chunks)
		}
		batch := chunks[i:end]
		vecs, err := s.Embedder.Embed(ctx, batch)
		if err != nil {
			_ = database.UpdateDocumentChunksAndStatus(s.DB, docID, len(chunks), "failed")
			return nil, fmt.Errorf("embed: %w", err)
		}
		for j, vec := range vecs {
			points = append(points, Point{
				ID:     fmt.Sprintf("%s_%d", docID, i+j),
				Vector: vec,
				Payload: map[string]string{
					"document_id": docID,
					"filename":    filename,
					"owner":       owner,
					"text":        batch[j],
					"chunk_index": fmt.Sprintf("%d", i+j),
				},
			})
		}
	}

	if err := s.VS.Upsert(ctx, qdrantName, points); err != nil {
		_ = database.UpdateDocumentChunksAndStatus(s.DB, docID, len(chunks), "failed")
		return nil, fmt.Errorf("upsert: %w", err)
	}

	_ = database.UpdateDocumentChunksAndStatus(s.DB, docID, len(chunks), "processed")
	_ = database.IncrementDocumentCount(s.DB, collectionID, 1)

	meta.Status = "processed"
	return &meta, nil
}

// Delete removes a document's vectors and its metadata record.
func (s *IngestService) Delete(ctx context.Context, docID, qdrantName string) error {
	meta, err := database.GetDocumentMeta(s.DB, docID)
	if err != nil {
		return fmt.Errorf("document not found: %w", err)
	}
	if err := s.VS.DeleteByDocumentID(ctx, qdrantName, docID); err != nil {
		return err
	}
	_ = database.IncrementDocumentCount(s.DB, meta.CollectionID, -1)
	return database.DeleteDocumentMeta(s.DB, docID)
}

// Move copies a document's vectors to a new collection then deletes them from
// the old one, and updates the metadata collection reference.
func (s *IngestService) Move(ctx context.Context, docID, fromQdrant, toQdrant string, toCollectionID int64) error {
	pts, err := s.VS.ScrollByDocumentID(ctx, fromQdrant, docID)
	if err != nil {
		return fmt.Errorf("scroll: %w", err)
	}
	var newPts []Point
	for _, p := range pts {
		newPts = append(newPts, Point{ID: p.ID, Vector: p.Vector, Payload: p.Payload})
	}
	if len(newPts) > 0 {
		if err := s.VS.Upsert(ctx, toQdrant, newPts); err != nil {
			return fmt.Errorf("upsert to target: %w", err)
		}
	}
	if err := s.VS.DeleteByDocumentID(ctx, fromQdrant, docID); err != nil {
		return fmt.Errorf("delete from source: %w", err)
	}
	meta, err := database.GetDocumentMeta(s.DB, docID)
	if err != nil {
		return err
	}
	_ = database.IncrementDocumentCount(s.DB, meta.CollectionID, -1)
	_ = database.IncrementDocumentCount(s.DB, toCollectionID, 1)
	return database.UpdateDocumentCollection(s.DB, docID, toCollectionID)
}
