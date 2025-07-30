package config

import "os"

// Config holds application configuration
type Config struct {
	GitLab GitLabConfig
	Server ServerConfig
}

// GitLabConfig holds GitLab API configuration
type GitLabConfig struct {
	BaseURL string
	Token   string
}

// ServerConfig holds server configuration
type ServerConfig struct {
	Port string
}

// Load loads configuration from environment variables
func Load() *Config {
	return &Config{
		GitLab: GitLabConfig{
			BaseURL: getEnv("GITLAB_BASE_URL", "https://gitlab.com"),
			Token:   getEnv("GITLAB_TOKEN", ""),
		},
		Server: ServerConfig{
			Port: getEnv("PORT", "3000"),
		},
	}
}

// HasGitLabToken returns true if GitLab token is configured
func (c *Config) HasGitLabToken() bool {
	return c.GitLab.Token != ""
}

// AnalysisMode returns a description of the current analysis mode
func (c *Config) AnalysisMode() string {
	if c.HasGitLabToken() {
		return "Full YAML analysis"
	}
	return "Limited (no GitLab token)"
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}