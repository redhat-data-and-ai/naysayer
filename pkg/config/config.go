package config

import (
	"os"
	"strconv"
)

type Config struct {
	// Server configuration
	Port string

	// GitLab configuration
	GitLabToken        string
	GitLabBaseURL      string
	DataProductRepo    string // e.g., "dataverse/dataverse-config/dataproduct-config"
	
	// Webhook configuration
	WebhookSecret      string
	
	// Removed pipeline configuration - focusing only on warehouse changes
	
	// Feature flags
	EnableActualGitLabAPI bool // For Phase 2 - when false, uses mock responses
	LogLevel              string
}

// LoadConfig loads configuration from environment variables
func LoadConfig() *Config {
	return &Config{
		Port:                   getEnv("PORT", "3000"),
		GitLabToken:            getEnv("GITLAB_TOKEN", ""),
		GitLabBaseURL:          getEnv("GITLAB_BASE_URL", "https://gitlab.cee.redhat.com"),
		DataProductRepo:        getEnv("DATAPRODUCT_REPO", "dataverse/dataverse-config/dataproduct-config"),
		WebhookSecret:          getEnv("WEBHOOK_SECRET", ""),

		EnableActualGitLabAPI:  getEnvBool("ENABLE_GITLAB_API", false),
		LogLevel:               getEnv("LOG_LEVEL", "info"),
	}
}

// getEnv gets an environment variable with a default value
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// getEnvBool gets a boolean environment variable with a default value
func getEnvBool(key string, defaultValue bool) bool {
	if value := os.Getenv(key); value != "" {
		if parsed, err := strconv.ParseBool(value); err == nil {
			return parsed
		}
	}
	return defaultValue
}

// getEnvInt gets an integer environment variable with a default value
func getEnvInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if parsed, err := strconv.Atoi(value); err == nil {
			return parsed
		}
	}
	return defaultValue
} 