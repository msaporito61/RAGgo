package config

import (
	"os"
	"strconv"
)

type Config struct {
	Port        string
	Environment string

	OpenRouterAPIKey  string
	OpenRouterBaseURL string
	LLMModel          string
	EmbeddingModel    string

	QdrantHost string
	QdrantPort int

	DatabaseURL string

	SecretKey              string
	AdminUsername          string
	AdminPassword          string
	GlobalAPIKey           string
	RateLimitPerMinute     int
}

func Load() *Config {
	return &Config{
		Port:              getenv("PORT", "8080"),
		Environment:       getenv("ENVIRONMENT", "development"),
		OpenRouterAPIKey:  getenv("OPENROUTER_API_KEY", ""),
		OpenRouterBaseURL: getenv("OPENROUTER_BASE_URL", "https://openrouter.ai/api/v1"),
		LLMModel:          getenv("LLM_MODEL", "openai/gpt-4o-mini"),
		EmbeddingModel:    getenv("EMBEDDING_MODEL", "text-embedding-3-small"),
		QdrantHost:        getenv("QDRANT_HOST", "localhost"),
		QdrantPort:        getenvInt("QDRANT_PORT", 6334),
		DatabaseURL:       getenv("DATABASE_URL", "./data/rag.db"),
		SecretKey:         getenv("SECURITY_SECRET_KEY", ""),
		AdminUsername:     getenv("SECURITY_ADMIN_USERNAME", "admin"),
		AdminPassword:     getenv("SECURITY_ADMIN_PASSWORD", ""),
		GlobalAPIKey:      getenv("SECURITY_API_KEY", ""),
		RateLimitPerMinute: getenvInt("SECURITY_RATE_LIMIT_PER_MINUTE", 60),
	}
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getenvInt(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return fallback
}
