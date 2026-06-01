package config

import (
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	HTTPAddress        string
	SessionTTL         time.Duration
	CORSAllowedOrigins []string
	MongoURI           string
	MongoDatabase      string
	ClassifierBaseURL  string
	ClassifierAPIKey   string
	ClassifierTimeout  time.Duration
}

func Load() Config {
	return Config{
		HTTPAddress:        getEnv("HTTP_ADDRESS", ":8001"),
		SessionTTL:         getDurationSeconds("SESSION_TTL_SECONDS", 60*60*24*7),
		CORSAllowedOrigins: getCSVEnv("CORS_ALLOWED_ORIGINS", []string{"http://localhost:3000", "http://localhost:5173", "http://localhost:4000"}),
		MongoURI:           getEnv("MONGO_URI", "mongodb://localhost:27017"),
		MongoDatabase:      getEnv("MONGO_DATABASE", "sentir-mais"),
		ClassifierBaseURL:  getEnv("CLASSIFIER_BASE_URL", ""),
		ClassifierAPIKey:   getEnv("CLASSIFIER_API_KEY", ""),
		ClassifierTimeout:  getDurationSeconds("CLASSIFIER_TIMEOUT_SECONDS", 10),
	}
}

func getEnv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}

	return fallback
}

func getDurationSeconds(key string, fallbackSeconds int) time.Duration {
	raw := os.Getenv(key)
	if raw == "" {
		return time.Duration(fallbackSeconds) * time.Second
	}

	seconds, err := strconv.Atoi(raw)
	if err != nil || seconds <= 0 {
		return time.Duration(fallbackSeconds) * time.Second
	}

	return time.Duration(seconds) * time.Second
}

func getCSVEnv(key string, fallback []string) []string {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return fallback
	}

	parts := strings.Split(raw, ",")
	values := make([]string, 0, len(parts))
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed != "" {
			values = append(values, trimmed)
		}
	}

	if len(values) == 0 {
		return fallback
	}

	return values
}
