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
	"raggo/internal/services/chat"
	"raggo/internal/services/collection"
	"raggo/internal/services/document"
	"raggo/internal/services/embedding"
	"raggo/internal/services/search"
	"raggo/internal/services/user"
	"raggo/internal/services/vectorstore"
)

func NewRouter(cfg *config.Config, db *sql.DB) http.Handler {
	// Initialize shared services
	qdrant, _ := vectorstore.NewClient(cfg.QdrantHost, cfg.QdrantPort)

	embedSvc := embedding.New(cfg.OpenRouterBaseURL, cfg.OpenRouterAPIKey, cfg.EmbeddingModel)

	collSvc := &collection.Service{DB: db, Qdrant: qdrant, VectorSize: embedSvc.VectorSize()}

	vsAdapter := &document.VectorStoreAdapter{Client: qdrant}
	ingestSvc := &document.IngestService{
		DB:           db,
		Embedder:     embedSvc,
		VS:           vsAdapter,
		ChunkSize:    500,
		ChunkOverlap: 50,
	}

	searchSvc := &search.Service{Embedder: embedSvc, VS: qdrant}
	chatSvc := chat.New(db, searchSvc, cfg.OpenRouterBaseURL, cfg.OpenRouterAPIKey, cfg.LLMModel)

	userSvc := &user.Service{DB: db, Cfg: cfg}
	_ = userSvc.SeedAdmin()

	r := chi.NewRouter()
	r.Use(chimw.Logger)
	r.Use(chimw.Recoverer)
	r.Use(middleware.SecurityHeaders)
	r.Use(cors.New(cors.Options{
		AllowedOrigins:   []string{"*"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Authorization", "Content-Type", "X-API-Key"},
		AllowCredentials: true,
	}).Handler)

	authH := &handlers.AuthHandler{UserSvc: userSvc}
	docH := &handlers.DocumentHandler{
		IngestSvc: ingestSvc,
		CollSvc:   collSvc,
		MaxUpload: 50 << 20,
	}
	colH := &handlers.CollectionHandler{Svc: collSvc}
	srchH := &handlers.SearchHandler{SearchSvc: searchSvc, DB: db, CollSvc: collSvc}
	chatH := &handlers.ChatHandler{ChatSvc: chatSvc, CollSvc: collSvc, DB: db}
	scrapeH := &handlers.ScrapeHandler{IngestSvc: ingestSvc, CollSvc: collSvc}
	adminH := &handlers.AdminHandler{UserSvc: userSvc, DB: db}

	// Public routes
	r.Get("/health", handlers.Health(cfg))
	r.Post("/auth/login", authH.Login)
	r.Post("/auth/refresh", authH.Refresh)

	// Protected routes
	r.Group(func(r chi.Router) {
		r.Use(middleware.Authenticator(cfg, db))

		// Collection routes
		r.Get("/collections", colH.List)
		r.Post("/collections", colH.Create)
		r.Delete("/collections/{slug}", colH.Delete)

		// Document routes
		r.Post("/documents/upload", docH.Upload)
		r.Get("/documents", docH.List)
		r.Delete("/documents/{docID}", docH.Delete)
		r.Patch("/documents/{docID}/move", docH.Move)

		// Scrape route
		r.Post("/documents/scrape", scrapeH.Scrape)

		// Search
		r.Post("/search", srchH.Search)

		// Chat session routes
		r.Post("/chat/sessions", chatH.CreateSession)
		r.Get("/chat/sessions", chatH.ListSessions)
		r.Delete("/chat/sessions/{sessionID}", chatH.DeleteSession)
		r.Post("/chat/sessions/{sessionID}/message", chatH.Chat)
		r.Get("/chat/sessions/{sessionID}/stream", chatH.ChatStream)

		// User self-service
		r.Post("/users/api-key", adminH.SetAPIKey)

		// Admin-only routes
		r.Group(func(r chi.Router) {
			r.Use(middleware.RequireRole("admin"))
			r.Get("/admin/collections", colH.ListAll)
			r.Get("/admin/users", adminH.ListUsers)
			r.Post("/admin/users", adminH.CreateUser)
			r.Delete("/admin/users/{username}", adminH.DeleteUser)
		})
	})

	return r
}
