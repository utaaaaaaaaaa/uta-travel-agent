package main

import (
	"fmt"
	"os"
)

// Config holds application configuration
type Config struct {
	// Server
	HTTPPort string

	// Embedding Service (gRPC)
	EmbeddingAddr string

	// Qdrant Vector DB
	QdrantHost string
	QdrantPort int

	// LLM
	LLMAPIKey   string
	LLMBBaseURL string
	LLMModel    string

	// Database
	DatabaseURL  string
	DatabaseHost string
	DatabasePort int
	DatabaseUser string
	DatabasePass string
	DatabaseName string
	DatabaseSSL  string
}

// Load loads configuration from environment
func Load() *Config {
	return &Config{
		HTTPPort:      getEnv("HTTP_PORT", "8080"),
		EmbeddingAddr: getEnv("EMBEDDING_ADDR", "localhost:50052"),
		QdrantHost:    getEnv("QDRANT_HOST", "localhost"),
		QdrantPort:    getEnvInt("QDRANT_PORT", 6334),
		LLMAPIKey:     getEnv("LLM_API_KEY", ""),
		LLMBBaseURL:   getEnv("LLM_BASE_URL", "https://open.bigmodel.cn/api/paas/v4"),
		LLMModel:      getEnv("LLM_MODEL", "glm-4-flash"),
		DatabaseURL:   getEnv("DATABASE_URL", ""),
		DatabaseHost:  getEnv("DATABASE_HOST", "localhost"),
		DatabasePort:  getEnvInt("DATABASE_PORT", 5432),
		DatabaseUser:  getEnv("DATABASE_USER", "postgres"),
		DatabasePass:  getEnv("DATABASE_PASSWORD", "postgres"),
		DatabaseName:  getEnv("DATABASE_NAME", "uta_travel"),
		DatabaseSSL:   getEnv("DATABASE_SSLMODE", "disable"),
	}
}

func getEnv(key, defaultValue string) string {
	val := os.Getenv(key)
	if val == "" {
		return defaultValue
	}
	return val
}

func getEnvInt(key string, defaultValue int) int {
	val := os.Getenv(key)
	if val == "" {
		return defaultValue
	}
	var result int
	_, err := fmt.Sscanf(val, "%d", &result)
	if err != nil {
		return defaultValue
	}
	return result
}