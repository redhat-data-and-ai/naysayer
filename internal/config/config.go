package config

import (
	"os"
	"strings"
)

// Config holds application configuration
type Config struct {
	GitLab  GitLabConfig
	Server  ServerConfig
	Webhook WebhookConfig
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

// WebhookConfig holds webhook security configuration
type WebhookConfig struct {
	Secret             string   // GitLab webhook secret token
	EnableVerification bool     // Enable signature verification
	AllowedIPs         []string // Optional: restrict webhook calls to specific IPs
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
		Webhook: WebhookConfig{
			Secret:             getEnv("WEBHOOK_SECRET", ""),
			EnableVerification: getEnv("WEBHOOK_VERIFY", "true") == "true",
			AllowedIPs:         parseIPList(getEnv("WEBHOOK_ALLOWED_IPS", "")),
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

// HasWebhookSecret returns true if webhook secret is configured
func (c *Config) HasWebhookSecret() bool {
	return c.Webhook.Secret != ""
}

// WebhookSecurityMode returns a description of the current webhook security mode
func (c *Config) WebhookSecurityMode() string {
	if !c.Webhook.EnableVerification {
		return "Disabled (INSECURE)"
	}
	if c.HasWebhookSecret() {
		return "Token verification enabled"
	}
	return "Verification enabled but no secret configured"
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// parseIPList parses a comma-separated list of IP addresses
func parseIPList(ipString string) []string {
	if ipString == "" {
		return []string{}
	}
	ips := strings.Split(ipString, ",")
	result := make([]string, 0) // Initialize to empty slice, not nil
	for _, ip := range ips {
		if trimmed := strings.TrimSpace(ip); trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}
