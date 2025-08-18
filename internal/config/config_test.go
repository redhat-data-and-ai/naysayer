package config

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLoad_DefaultValues(t *testing.T) {
	// Clear all relevant environment variables for clean test
	envVars := []string{
		"GITLAB_BASE_URL", "GITLAB_TOKEN", "PORT",
		"WEBHOOK_SECRET", "WEBHOOK_ALLOWED_IPS",
	}

	originalValues := make(map[string]string)
	for _, envVar := range envVars {
		originalValues[envVar] = os.Getenv(envVar)
		_ = os.Unsetenv(envVar)
	}
	defer func() {
		// Restore original environment
		for envVar, value := range originalValues {
			if value != "" {
				_ = os.Setenv(envVar, value)
			} else {
				_ = os.Unsetenv(envVar)
			}
		}
	}()

	config := Load()

	// Test default values
	assert.Equal(t, "https://gitlab.com", config.GitLab.BaseURL)
	assert.Equal(t, "", config.GitLab.Token)
	assert.Equal(t, "3000", config.Server.Port)
	assert.Equal(t, "", config.Webhook.Secret)
	assert.Empty(t, config.Webhook.AllowedIPs)
}

func TestLoad_EnvironmentOverrides(t *testing.T) {
	// Set test environment variables
	testValues := map[string]string{
		"GITLAB_BASE_URL": "https://gitlab.example.com",
		"GITLAB_TOKEN":    "test-token-123",
		"PORT":            "8080",
		"WEBHOOK_SECRET":  "secret-webhook-token",

		"WEBHOOK_ALLOWED_IPS": "192.168.1.1, 10.0.0.1,  172.16.0.1  ",
	}

	// Store original values
	originalValues := make(map[string]string)
	for key, value := range testValues {
		originalValues[key] = os.Getenv(key)
		_ = os.Setenv(key, value)
	}
	defer func() {
		// Restore original environment
		for key, value := range originalValues {
			if value != "" {
				_ = os.Setenv(key, value)
			} else {
				_ = os.Unsetenv(key)
			}
		}
	}()

	config := Load()

	// Test environment override values
	assert.Equal(t, "https://gitlab.example.com", config.GitLab.BaseURL)
	assert.Equal(t, "test-token-123", config.GitLab.Token)
	assert.Equal(t, "8080", config.Server.Port)
	assert.Equal(t, "secret-webhook-token", config.Webhook.Secret)
	assert.Equal(t, []string{"192.168.1.1", "10.0.0.1", "172.16.0.1"}, config.Webhook.AllowedIPs)
}

func TestHasGitLabToken(t *testing.T) {
	tests := []struct {
		name     string
		token    string
		expected bool
	}{
		{
			name:     "with valid token",
			token:    "glpat-mock-token-for-testing",
			expected: true,
		},
		{
			name:     "with empty token",
			token:    "",
			expected: false,
		},
		{
			name:     "with whitespace token",
			token:    "   ",
			expected: true, // getEnv doesn't trim whitespace
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := &Config{
				GitLab: GitLabConfig{
					Token: tt.token,
				},
			}

			result := config.HasGitLabToken()
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestAnalysisMode(t *testing.T) {
	tests := []struct {
		name     string
		token    string
		expected string
	}{
		{
			name:     "with GitLab token",
			token:    "glpat-mock-token-for-testing",
			expected: "Full YAML analysis",
		},
		{
			name:     "without GitLab token",
			token:    "",
			expected: "Limited (no GitLab token)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := &Config{
				GitLab: GitLabConfig{
					Token: tt.token,
				},
			}

			result := config.AnalysisMode()
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestHasWebhookSecret(t *testing.T) {
	tests := []struct {
		name     string
		secret   string
		expected bool
	}{
		{
			name:     "with valid secret",
			secret:   "webhook-secret-123",
			expected: true,
		},
		{
			name:     "with empty secret",
			secret:   "",
			expected: false,
		},
		{
			name:     "with whitespace secret",
			secret:   "   ",
			expected: true, // getEnv doesn't trim whitespace
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := &Config{
				Webhook: WebhookConfig{
					Secret: tt.secret,
				},
			}

			result := config.HasWebhookSecret()
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestWebhookSecurityMode(t *testing.T) {
	tests := []struct {
		name     string
		secret   string
		expected string
	}{
		{
			name:     "with secret",
			secret:   "webhook-secret-123",
			expected: "Token verification available",
		},
		{
			name:     "without secret",
			secret:   "",
			expected: "No secret configured",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := &Config{
				Webhook: WebhookConfig{
					Secret: tt.secret,
				},
			}

			result := config.WebhookSecurityMode()
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGetEnv(t *testing.T) {
	tests := []struct {
		name         string
		key          string
		defaultValue string
		envValue     string
		setEnv       bool
		expected     string
	}{
		{
			name:         "environment variable set",
			key:          "TEST_ENV_VAR",
			defaultValue: "default",
			envValue:     "environment-value",
			setEnv:       true,
			expected:     "environment-value",
		},
		{
			name:         "environment variable not set",
			key:          "UNSET_ENV_VAR",
			defaultValue: "default",
			envValue:     "",
			setEnv:       false,
			expected:     "default",
		},
		{
			name:         "environment variable set to empty string",
			key:          "EMPTY_ENV_VAR",
			defaultValue: "default",
			envValue:     "",
			setEnv:       true,
			expected:     "default", // Empty env var should use default
		},
		{
			name:         "environment variable with whitespace",
			key:          "WHITESPACE_ENV_VAR",
			defaultValue: "default",
			envValue:     "  value  ",
			setEnv:       true,
			expected:     "  value  ", // getEnv doesn't trim
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Save original value
			originalValue := os.Getenv(tt.key)
			defer func() {
				if originalValue != "" {
					_ = os.Setenv(tt.key, originalValue)
				} else {
					_ = os.Unsetenv(tt.key)
				}
			}()

			if tt.setEnv {
				_ = os.Setenv(tt.key, tt.envValue)
			} else {
				_ = os.Unsetenv(tt.key)
			}

			result := getEnv(tt.key, tt.defaultValue)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestParseIPList(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []string
	}{
		{
			name:     "empty string",
			input:    "",
			expected: []string{},
		},
		{
			name:     "single IP",
			input:    "192.168.1.1",
			expected: []string{"192.168.1.1"},
		},
		{
			name:     "multiple IPs",
			input:    "192.168.1.1,10.0.0.1,172.16.0.1",
			expected: []string{"192.168.1.1", "10.0.0.1", "172.16.0.1"},
		},
		{
			name:     "IPs with whitespace",
			input:    " 192.168.1.1 , 10.0.0.1  ,  172.16.0.1 ",
			expected: []string{"192.168.1.1", "10.0.0.1", "172.16.0.1"},
		},
		{
			name:     "empty entries",
			input:    "192.168.1.1,,10.0.0.1, ,172.16.0.1",
			expected: []string{"192.168.1.1", "10.0.0.1", "172.16.0.1"},
		},
		{
			name:     "only commas and whitespace",
			input:    " , , , ",
			expected: []string{},
		},
		{
			name:     "IPv6 addresses",
			input:    "::1,2001:db8::1,fe80::1",
			expected: []string{"::1", "2001:db8::1", "fe80::1"},
		},
		{
			name:     "mixed IPv4 and IPv6",
			input:    "127.0.0.1,::1,192.168.1.1",
			expected: []string{"127.0.0.1", "::1", "192.168.1.1"},
		},
		{
			name:     "trailing and leading commas",
			input:    ",192.168.1.1,10.0.0.1,",
			expected: []string{"192.168.1.1", "10.0.0.1"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseIPList(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestConfigStructs_FieldsExist(t *testing.T) {
	// Test that all expected fields exist and have correct types
	config := &Config{}

	// GitLab config
	assert.IsType(t, GitLabConfig{}, config.GitLab)
	assert.IsType(t, "", config.GitLab.BaseURL)
	assert.IsType(t, "", config.GitLab.Token)

	// Server config
	assert.IsType(t, ServerConfig{}, config.Server)
	assert.IsType(t, "", config.Server.Port)

	// Webhook config
	assert.IsType(t, WebhookConfig{}, config.Webhook)
	assert.IsType(t, "", config.Webhook.Secret)
	assert.IsType(t, []string{}, config.Webhook.AllowedIPs)
}

func TestConfigIntegration_RealWorldScenarios(t *testing.T) {
	tests := []struct {
		name        string
		envVars     map[string]string
		unsetVars   []string
		description string
		checkFunc   func(*testing.T, *Config)
	}{
		{
			name: "production configuration",
			envVars: map[string]string{
				"GITLAB_BASE_URL": "https://gitlab.company.com",
				"GITLAB_TOKEN":    "glpat-production-token",
				"PORT":            "8080",
				"WEBHOOK_SECRET":  "super-secure-webhook-secret",

				"WEBHOOK_ALLOWED_IPS": "10.0.0.0/8,172.16.0.0/12",
			},
			description: "Typical production setup with all security features enabled",
			checkFunc: func(t *testing.T, c *Config) {
				assert.True(t, c.HasGitLabToken())
				assert.True(t, c.HasWebhookSecret())
				assert.Equal(t, "Full YAML analysis", c.AnalysisMode())
				assert.Equal(t, "Token verification available", c.WebhookSecurityMode())
				assert.Len(t, c.Webhook.AllowedIPs, 2)
			},
		},
		{
			name: "development configuration",
			envVars: map[string]string{
				"GITLAB_BASE_URL": "https://gitlab.com",
				"GITLAB_TOKEN":    "glpat-mock-token-for-testing",
				"PORT":            "3000",
			},
			unsetVars:   []string{"WEBHOOK_SECRET", "WEBHOOK_ALLOWED_IPS"},
			description: "Development setup with relaxed security",
			checkFunc: func(t *testing.T, c *Config) {
				assert.True(t, c.HasGitLabToken())
				assert.False(t, c.HasWebhookSecret())
				assert.Equal(t, "Full YAML analysis", c.AnalysisMode())
				assert.Equal(t, "No secret configured", c.WebhookSecurityMode())
				assert.Empty(t, c.Webhook.AllowedIPs)
			},
		},
		{
			name:        "minimal configuration",
			envVars:     map[string]string{},
			unsetVars:   []string{"GITLAB_TOKEN", "WEBHOOK_SECRET", "WEBHOOK_ALLOWED_IPS"},
			description: "Minimal setup with defaults",
			checkFunc: func(t *testing.T, c *Config) {
				assert.False(t, c.HasGitLabToken())
				assert.False(t, c.HasWebhookSecret())
				assert.Equal(t, "Limited (no GitLab token)", c.AnalysisMode())
				assert.Equal(t, "No secret configured", c.WebhookSecurityMode())
				assert.Empty(t, c.Webhook.AllowedIPs)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Store original environment
			originalEnv := make(map[string]string)
			allVars := make(map[string]bool)

			// Collect all variables that might be affected
			for key := range tt.envVars {
				allVars[key] = true
				originalEnv[key] = os.Getenv(key)
			}
			for _, key := range tt.unsetVars {
				allVars[key] = true
				originalEnv[key] = os.Getenv(key)
			}

			defer func() {
				// Restore original environment
				for key, value := range originalEnv {
					if value != "" {
						_ = os.Setenv(key, value)
					} else {
						_ = os.Unsetenv(key)
					}
				}
			}()

			// Set test environment
			for key, value := range tt.envVars {
				_ = os.Setenv(key, value)
			}
			for _, key := range tt.unsetVars {
				_ = os.Unsetenv(key)
			}

			// Load configuration and test
			config := Load()
			tt.checkFunc(t, config)
		})
	}
}
