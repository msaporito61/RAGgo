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
	"raggo/internal/services/user"
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

	// Public routes
	r.Get("/health", handlers.Health(cfg))
	r.Post("/auth/login", authH.Login)
	r.Post("/auth/refresh", authH.Refresh)

	// Protected routes
	r.Group(func(r chi.Router) {
		r.Use(middleware.Authenticator(cfg, db))
		// handlers added in later tasks
	})

	return r
}
