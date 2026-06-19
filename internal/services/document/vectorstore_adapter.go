package document

import (
	"context"

	"raggo/internal/services/vectorstore"
)

// VectorStoreAdapter wraps *vectorstore.Client to satisfy the VectorStore interface
// used by IngestService. The two Point types are structurally identical; this adapter
// bridges the package-level type difference.
type VectorStoreAdapter struct {
	Client *vectorstore.Client
}

func (a *VectorStoreAdapter) Upsert(ctx context.Context, collection string, pts []Point) error {
	qpts := make([]vectorstore.Point, len(pts))
	for i, p := range pts {
		qpts[i] = vectorstore.Point{ID: p.ID, Vector: p.Vector, Payload: p.Payload}
	}
	return a.Client.Upsert(ctx, collection, qpts)
}

func (a *VectorStoreAdapter) DeleteByDocumentID(ctx context.Context, collection, docID string) error {
	return a.Client.DeleteByDocumentID(ctx, collection, docID)
}

func (a *VectorStoreAdapter) ScrollByDocumentID(ctx context.Context, collection, docID string) ([]PointWithVector, error) {
	raw, err := a.Client.ScrollByDocumentID(ctx, collection, docID)
	if err != nil {
		return nil, err
	}
	result := make([]PointWithVector, len(raw))
	for i, p := range raw {
		result[i] = PointWithVector{ID: p.ID, Vector: p.Vector, Payload: p.Payload}
	}
	return result, nil
}
