package main

import (
	"log"
	"net/http"
	"os"

	"github.com/joho/godotenv"
	"raggo/internal/api"
	"raggo/internal/config"
	"raggo/internal/database"
)

func main() {
	_ = godotenv.Load()

	cfg := config.Load()

	db, err := database.Open(cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("db open: %v", err)
	}
	defer db.Close()

	if err := database.Migrate(db); err != nil {
		log.Fatalf("migrate: %v", err)
	}

	router := api.NewRouter(cfg, db)

	addr := ":" + cfg.Port
	log.Printf("RAGgo listening on %s", addr)
	if err := http.ListenAndServe(addr, router); err != nil {
		log.Fatalf("server: %v", err)
		os.Exit(1)
	}
}
