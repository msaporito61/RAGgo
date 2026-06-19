package collection

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"unicode"

	"raggo/internal/database"
)

type QdrantProvider interface {
	EnsureCollection(ctx context.Context, name string, vectorSize uint64) error
	DeleteCollection(ctx context.Context, name string) error
}

type Service struct {
	DB         *sql.DB
	Qdrant     QdrantProvider
	VectorSize uint64
}

func (s *Service) GetOrCreateDefault(ctx context.Context, username string) (*database.Collection, error) {
	col, err := database.GetCollection(s.DB, username, "default")
	if err == nil {
		return col, nil
	}
	return s.create(ctx, username, "Default", "default", true)
}

func (s *Service) Create(ctx context.Context, username, displayName string) (*database.Collection, error) {
	slug := toSlug(displayName)
	return s.create(ctx, username, displayName, slug, false)
}

func (s *Service) create(ctx context.Context, username, displayName, slug string, isDefault bool) (*database.Collection, error) {
	qdrantName := buildQdrantName(username, slug)
	if err := s.Qdrant.EnsureCollection(ctx, qdrantName, s.VectorSize); err != nil {
		return nil, fmt.Errorf("ensure qdrant collection: %w", err)
	}
	if err := database.CreateCollection(s.DB, database.Collection{
		Slug:          slug,
		DisplayName:   displayName,
		OwnerUsername: username,
		QdrantName:    qdrantName,
		IsDefault:     isDefault,
	}); err != nil {
		return nil, err
	}
	return database.GetCollection(s.DB, username, slug)
}

func (s *Service) List(username string) ([]database.Collection, error) {
	return database.ListCollectionsForUser(s.DB, username)
}

func (s *Service) ListAll() ([]database.Collection, error) {
	return database.ListAllCollections(s.DB)
}

func (s *Service) Delete(ctx context.Context, username, slug string) error {
	col, err := database.GetCollection(s.DB, username, slug)
	if err != nil {
		return err
	}
	if col.IsDefault {
		return errors.New("cannot delete the default collection")
	}
	if err := s.Qdrant.DeleteCollection(ctx, col.QdrantName); err != nil {
		return fmt.Errorf("delete qdrant collection: %w", err)
	}
	return database.DeleteCollection(s.DB, col.ID)
}

// buildQdrantName constructs a Qdrant collection name capped at 128 chars.
func buildQdrantName(username, slug string) string {
	name := fmt.Sprintf("rag_%s_%s", username, slug)
	if len(name) > 128 {
		name = name[:128]
	}
	return name
}

var nonAlpha = regexp.MustCompile(`[^a-z0-9]+`)

func toSlug(s string) string {
	s = strings.ToLower(s)
	s = strings.Map(func(r rune) rune {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			return r
		}
		return '-'
	}, s)
	s = nonAlpha.ReplaceAllString(s, "-")
	s = strings.Trim(s, "-")
	if s == "" {
		s = "collection"
	}
	return s
}
