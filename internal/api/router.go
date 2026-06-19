package api

import (
	"database/sql"
	"net/http"

	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"
	"github.com/rs/cors"
	"raggo/internal/api/handlers"
	"raggo/internal/config"
	"raggo/internal/middleware"
	"raggo/internal/services/collection"
	"raggo/internal/services/document"
	"raggo/internal/services/embedding"
	"raggo/internal/services/user"
	"raggo/internal/services/vectorstore"
)

func NewRouter(cfg *config.Config, db *sql.DB) http.Handler {
	r := chi.NewRouter()

	r.Use(chimw.Logger)
	r.Use(chimw.Recoverer)
	r.Use(middleware.SecurityHeaders)

	corsHandler := cors.New(cors.Options{
		AllowedOrigins:   []string{"*"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Authorization", "Content-Type", "X-API-Key"},
		AllowCredentials: true,
	})
	r.Use(corsHandler.Handler)

	userSvc := &user.Service{DB: db, Cfg: cfg}
	authH := &handlers.AuthHandler{UserSvc: userSvc}

	// Build shared services.
	embedSvc := embedding.New(cfg.OpenRouterBaseURL, cfg.OpenRouterAPIKey, cfg.EmbeddingModel)

	// Vectorstore client is shared between the collection service and ingest service.
	var vsClient *vectorstore.Client
	var vsAdapter document.VectorStore
	if vc, err := vectorstore.NewClient(cfg.QdrantHost, cfg.QdrantPort); err == nil {
		vsClient = vc
		vsAdapter = &document.VectorStoreAdapter{Client: vc}
	}

	collSvc := &collection.Service{DB: db, Qdrant: vsClient, VectorSize: embedSvc.VectorSize()}

	ingestSvc := &document.IngestService{
		DB:           db,
		Embedder:     embedSvc,
		VS:           vsAdapter,
		ChunkSize:    500,
		ChunkOverlap: 50,
	}

	docH := &handlers.DocumentHandler{
		IngestSvc: ingestSvc,
		CollSvc:   collSvc,
		MaxUpload: 50 << 20, // 50 MB
	}

	// Public routes
	r.Get("/health", handlers.Health(cfg))
	r.Post("/auth/login", authH.Login)
	r.Post("/auth/refresh", authH.Refresh)

	// Protected routes
	r.Group(func(r chi.Router) {
		r.Use(middleware.Authenticator(cfg, db))

		// Document routes
		r.Post("/documents/upload", docH.Upload)
		r.Get("/documents", docH.List)
		r.Delete("/documents/{docID}", docH.Delete)
		r.Post("/documents/{docID}/move", docH.Move)
	})

	return r
}
