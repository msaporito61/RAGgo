package vectorstore

import (
	"context"
	"fmt"

	"github.com/qdrant/go-client/qdrant"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type Point struct {
	ID      string
	Vector  []float32
	Payload map[string]string
}

type SearchResult struct {
	ID      string
	Score   float32
	Payload map[string]string
}

type PointWithVector struct {
	ID      string
	Vector  []float32
	Payload map[string]string
}

type Client struct {
	conn        *grpc.ClientConn
	collections qdrant.CollectionsClient
	points      qdrant.PointsClient
}

func NewClient(host string, port int) (*Client, error) {
	addr := fmt.Sprintf("%s:%d", host, port)
	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, err
	}
	return &Client{
		conn:        conn,
		collections: qdrant.NewCollectionsClient(conn),
		points:      qdrant.NewPointsClient(conn),
	}, nil
}

func (c *Client) Close() { c.conn.Close() }

func (c *Client) EnsureCollection(ctx context.Context, name string, vectorSize uint64) error {
	_, err := c.collections.Get(ctx, &qdrant.GetCollectionInfoRequest{CollectionName: name})
	if err == nil {
		return nil // already exists
	}
	_, err = c.collections.Create(ctx, &qdrant.CreateCollection{
		CollectionName: name,
		VectorsConfig: &qdrant.VectorsConfig{
			Config: &qdrant.VectorsConfig_Params{
				Params: &qdrant.VectorParams{
					Size:     vectorSize,
					Distance: qdrant.Distance_Cosine,
				},
			},
		},
	})
	return err
}

func (c *Client) DeleteCollection(ctx context.Context, name string) error {
	_, err := c.collections.Delete(ctx, &qdrant.DeleteCollection{CollectionName: name})
	return err
}

func (c *Client) Upsert(ctx context.Context, collection string, points []Point) error {
	var qpts []*qdrant.PointStruct
	for _, p := range points {
		payload := make(map[string]*qdrant.Value)
		for k, v := range p.Payload {
			payload[k] = &qdrant.Value{Kind: &qdrant.Value_StringValue{StringValue: v}}
		}
		qpts = append(qpts, &qdrant.PointStruct{
			Id:      &qdrant.PointId{PointIdOptions: &qdrant.PointId_Uuid{Uuid: p.ID}},
			Vectors: &qdrant.Vectors{VectorsOptions: &qdrant.Vectors_Vector{Vector: &qdrant.Vector{Data: p.Vector}}},
			Payload: payload,
		})
	}
	_, err := c.points.Upsert(ctx, &qdrant.UpsertPoints{CollectionName: collection, Points: qpts, Wait: boolPtr(true)})
	return err
}

func (c *Client) Search(ctx context.Context, collection string, vector []float32, limit uint64, filter map[string]string) ([]SearchResult, error) {
	req := &qdrant.SearchPoints{
		CollectionName: collection,
		Vector:         vector,
		Limit:          limit,
		WithPayload:    &qdrant.WithPayloadSelector{SelectorOptions: &qdrant.WithPayloadSelector_Enable{Enable: true}},
	}
	if len(filter) > 0 {
		var conditions []*qdrant.Condition
		for k, v := range filter {
			conditions = append(conditions, &qdrant.Condition{
				ConditionOneOf: &qdrant.Condition_Field{
					Field: &qdrant.FieldCondition{
						Key:   k,
						Match: &qdrant.Match{MatchValue: &qdrant.Match_Keyword{Keyword: v}},
					},
				},
			})
		}
		req.Filter = &qdrant.Filter{Must: conditions}
	}
	resp, err := c.points.Search(ctx, req)
	if err != nil {
		return nil, err
	}
	var results []SearchResult
	for _, r := range resp.Result {
		payload := make(map[string]string)
		for k, v := range r.Payload {
			if sv, ok := v.Kind.(*qdrant.Value_StringValue); ok {
				payload[k] = sv.StringValue
			}
		}
		results = append(results, SearchResult{
			ID:      r.Id.GetUuid(),
			Score:   r.Score,
			Payload: payload,
		})
	}
	return results, nil
}

func (c *Client) DeleteByDocumentID(ctx context.Context, collection, docID string) error {
	_, err := c.points.Delete(ctx, &qdrant.DeletePoints{
		CollectionName: collection,
		Points: &qdrant.PointsSelector{
			PointsSelectorOneOf: &qdrant.PointsSelector_Filter{
				Filter: &qdrant.Filter{
					Must: []*qdrant.Condition{{
						ConditionOneOf: &qdrant.Condition_Field{
							Field: &qdrant.FieldCondition{
								Key:   "document_id",
								Match: &qdrant.Match{MatchValue: &qdrant.Match_Keyword{Keyword: docID}},
							},
						},
					}},
				},
			},
		},
		Wait: boolPtr(true),
	})
	return err
}

func (c *Client) ScrollByDocumentID(ctx context.Context, collection, docID string) ([]PointWithVector, error) {
	resp, err := c.points.Scroll(ctx, &qdrant.ScrollPoints{
		CollectionName: collection,
		Filter: &qdrant.Filter{
			Must: []*qdrant.Condition{{
				ConditionOneOf: &qdrant.Condition_Field{
					Field: &qdrant.FieldCondition{
						Key:   "document_id",
						Match: &qdrant.Match{MatchValue: &qdrant.Match_Keyword{Keyword: docID}},
					},
				},
			}},
		},
		WithVectors: &qdrant.WithVectorsSelector{SelectorOptions: &qdrant.WithVectorsSelector_Enable{Enable: true}},
		WithPayload: &qdrant.WithPayloadSelector{SelectorOptions: &qdrant.WithPayloadSelector_Enable{Enable: true}},
		Limit:       uint32Ptr(10000),
	})
	if err != nil {
		return nil, err
	}
	var pts []PointWithVector
	for _, p := range resp.Result {
		payload := make(map[string]string)
		for k, v := range p.Payload {
			if sv, ok := v.Kind.(*qdrant.Value_StringValue); ok {
				payload[k] = sv.StringValue
			}
		}
		var vec []float32
		if p.Vectors != nil {
			if v := p.Vectors.GetVector(); v != nil {
				vec = v.Data
			}
		}
		pts = append(pts, PointWithVector{ID: p.Id.GetUuid(), Vector: vec, Payload: payload})
	}
	return pts, nil
}

func boolPtr(b bool) *bool       { return &b }
func uint32Ptr(n uint32) *uint32 { return &n }
