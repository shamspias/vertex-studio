package config

import (
	"fmt"
	"os"

	"github.com/joho/godotenv"
)

// Config holds all the application configuration
type Config struct {
	GoogleGenAIKey string
	ProjectID      string
	Location       string
}

// LoadConfig reads .env and validates required variables
func LoadConfig() (*Config, error) {
	// Load .env file (ignore error if file is missing, e.g., in production)
	_ = godotenv.Load()

	cfg := &Config{
		GoogleGenAIKey: os.Getenv("GOOGLE_GENAI_API_KEY"),
		ProjectID:      os.Getenv("GCLOUD_PROJECT_ID"),
		Location:       os.Getenv("GCLOUD_LOCATION"),
	}

	// Validate critical config
	if cfg.GoogleGenAIKey == "" {
		return nil, fmt.Errorf("CRITICAL: GOOGLE_GENAI_API_KEY is missing")
	}
	if cfg.ProjectID == "" {
		return nil, fmt.Errorf("CRITICAL: GCLOUD_PROJECT_ID is missing")
	}
	if cfg.Location == "" {
		return nil, fmt.Errorf("CRITICAL: GCLOUD_LOCATION is missing")
	}

	return cfg, nil
}
