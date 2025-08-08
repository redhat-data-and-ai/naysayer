package webhook

import (
	"encoding/json"
	"io"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/redhat-data-and-ai/naysayer/internal/config"
	"github.com/stretchr/testify/assert"
)

func TestNewHealthHandler(t *testing.T) {
	cfg := createTestConfig()
	handler := NewHealthHandler(cfg)

	assert.NotNil(t, handler)
	assert.Equal(t, cfg, handler.config)
	assert.False(t, handler.startTime.IsZero())
}

func TestHealthHandler_HandleHealth(t *testing.T) {
	cfg := createTestConfig()
	handler := NewHealthHandler(cfg)

	app := createTestApp()
	app.Get("/health", handler.HandleHealth)

	req := httptest.NewRequest("GET", "/health", nil)
	resp, err := app.Test(req)

	assert.NoError(t, err)
	assert.Equal(t, 200, resp.StatusCode)

	// Parse response
	body, _ := io.ReadAll(resp.Body)
	var health map[string]interface{}
	json.Unmarshal(body, &health)

	// Verify all expected fields are present
	assert.Equal(t, "healthy", health["status"])
	assert.Equal(t, "naysayer-webhook", health["service"])
	assert.Equal(t, "v1.0.0", health["version"])
	assert.NotNil(t, health["uptime_seconds"])
	assert.NotNil(t, health["timestamp"])
	assert.NotNil(t, health["analysis_mode"])
	assert.NotNil(t, health["security_mode"])
	assert.NotNil(t, health["gitlab_token"])
	assert.NotNil(t, health["webhook_secret"])

	// Verify uptime is reasonable
	uptime := health["uptime_seconds"].(float64)
	assert.True(t, uptime >= 0)
	assert.True(t, uptime < 10) // Should be very small for new handler
}

func TestHealthHandler_HandleHealth_UptimeProgression(t *testing.T) {
	cfg := createTestConfig()
	handler := NewHealthHandler(cfg)

	app := createTestApp()
	app.Get("/health", handler.HandleHealth)

	// First health check
	req1 := httptest.NewRequest("GET", "/health", nil)
	resp1, err := app.Test(req1)
	assert.NoError(t, err)

	body1, _ := io.ReadAll(resp1.Body)
	var health1 map[string]interface{}
	json.Unmarshal(body1, &health1)
	uptime1 := health1["uptime_seconds"].(float64)

	// Wait a more noticeable amount
	time.Sleep(time.Millisecond * 200)

	// Second health check
	req2 := httptest.NewRequest("GET", "/health", nil)
	resp2, err := app.Test(req2)
	assert.NoError(t, err)

	body2, _ := io.ReadAll(resp2.Body)
	var health2 map[string]interface{}
	json.Unmarshal(body2, &health2)
	uptime2 := health2["uptime_seconds"].(float64)

	// Uptime should have increased (allow for small measurement variations)
	assert.True(t, uptime2 >= uptime1, "Uptime should not decrease: %f >= %f", uptime2, uptime1)
}

func TestHealthHandler_HandleReady_Success(t *testing.T) {
	cfg := &config.Config{
		GitLab: config.GitLabConfig{
			BaseURL: "https://gitlab.example.com",
			Token:   "test-token",
		},
		Webhook: config.WebhookConfig{},
	}
	handler := NewHealthHandler(cfg)

	app := createTestApp()
	app.Get("/ready", handler.HandleReady)

	req := httptest.NewRequest("GET", "/ready", nil)
	resp, err := app.Test(req)

	assert.NoError(t, err)
	assert.Equal(t, 200, resp.StatusCode)

	body, _ := io.ReadAll(resp.Body)
	var ready map[string]interface{}
	json.Unmarshal(body, &ready)

	assert.Equal(t, true, ready["ready"])
	assert.Equal(t, "naysayer-webhook", ready["service"])
	assert.NotNil(t, ready["timestamp"])
	assert.NotNil(t, ready["gitlab_token"])
	assert.NotNil(t, ready["webhook_secret"])
}

func TestHealthHandler_HandleReady_MissingGitLabToken(t *testing.T) {
	cfg := &config.Config{
		GitLab: config.GitLabConfig{
			BaseURL: "https://gitlab.example.com",
			Token:   "", // No token
		},
		Webhook: config.WebhookConfig{
		},
	}
	handler := NewHealthHandler(cfg)

	app := createTestApp()
	app.Get("/ready", handler.HandleReady)

	req := httptest.NewRequest("GET", "/ready", nil)
	resp, err := app.Test(req)

	assert.NoError(t, err)
	assert.Equal(t, 503, resp.StatusCode)

	body, _ := io.ReadAll(resp.Body)
	var ready map[string]interface{}
	json.Unmarshal(body, &ready)

	assert.Equal(t, false, ready["ready"])
	assert.Equal(t, "GitLab token not configured", ready["reason"])
}

func TestHealthHandler_HandleReady_MissingWebhookSecret(t *testing.T) {
	cfg := &config.Config{
		GitLab: config.GitLabConfig{
			BaseURL: "https://gitlab.example.com",
			Token:   "test-token",
		},
		Webhook: config.WebhookConfig{
			Secret:             "",   // No secret
		},
	}
	handler := NewHealthHandler(cfg)

	app := createTestApp()
	app.Get("/ready", handler.HandleReady)

	req := httptest.NewRequest("GET", "/ready", nil)
	resp, err := app.Test(req)

	assert.NoError(t, err)
	assert.Equal(t, 200, resp.StatusCode)

	body, _ := io.ReadAll(resp.Body)
	var ready map[string]interface{}
	json.Unmarshal(body, &ready)

	assert.Equal(t, true, ready["ready"])
	assert.Nil(t, ready["reason"])
}

func TestHealthHandler_HandleReady_VerificationDisabled(t *testing.T) {
	cfg := &config.Config{
		GitLab: config.GitLabConfig{
			BaseURL: "https://gitlab.example.com",
			Token:   "", // No token, should fail readiness
		},
		Webhook: config.WebhookConfig{

		},
	}
	handler := NewHealthHandler(cfg)

	app := createTestApp()
	app.Get("/ready", handler.HandleReady)

	req := httptest.NewRequest("GET", "/ready", nil)
	resp, err := app.Test(req)

	assert.NoError(t, err)
	assert.Equal(t, 503, resp.StatusCode)

	body, _ := io.ReadAll(resp.Body)
	var ready map[string]interface{}
	json.Unmarshal(body, &ready)

	assert.Equal(t, false, ready["ready"])
	assert.Equal(t, "GitLab token not configured", ready["reason"])
}

func TestHealthHandler_HandleHealth_WithSecureConfig(t *testing.T) {
	cfg := &config.Config{
		GitLab: config.GitLabConfig{
			BaseURL: "https://gitlab.example.com",
			Token:   "secure-token",
		},
		Webhook: config.WebhookConfig{

			Secret:             "webhook-secret",
			AllowedIPs:         []string{"192.168.1.0/24", "10.0.0.0/8"},
		},
	}
	handler := NewHealthHandler(cfg)

	app := createTestApp()
	app.Get("/health", handler.HandleHealth)

	req := httptest.NewRequest("GET", "/health", nil)
	resp, err := app.Test(req)

	assert.NoError(t, err)
	assert.Equal(t, 200, resp.StatusCode)

	body, _ := io.ReadAll(resp.Body)
	var health map[string]interface{}
	json.Unmarshal(body, &health)

	assert.Equal(t, "healthy", health["status"])
	assert.Equal(t, true, health["gitlab_token"])
	assert.Equal(t, true, health["webhook_secret"])
}

func TestHealthHandler_HandleHealth_ContentType(t *testing.T) {
	cfg := createTestConfig()
	handler := NewHealthHandler(cfg)

	app := createTestApp()
	app.Get("/health", handler.HandleHealth)

	req := httptest.NewRequest("GET", "/health", nil)
	resp, err := app.Test(req)

	assert.NoError(t, err)
	assert.Equal(t, "application/json", resp.Header.Get("Content-Type"))
}

func TestHealthHandler_HandleReady_ContentType(t *testing.T) {
	cfg := createTestConfig()
	handler := NewHealthHandler(cfg)

	app := createTestApp()
	app.Get("/ready", handler.HandleReady)

	req := httptest.NewRequest("GET", "/ready", nil)
	resp, err := app.Test(req)

	assert.NoError(t, err)
	assert.Equal(t, "application/json", resp.Header.Get("Content-Type"))
}

func TestHealthHandler_StartTimeImmutable(t *testing.T) {
	cfg := createTestConfig()
	handler := NewHealthHandler(cfg)

	startTime1 := handler.startTime

	// Wait a bit
	time.Sleep(time.Millisecond * 50)

	// Create another health check - start time should be the same
	app := createTestApp()
	app.Get("/health", handler.HandleHealth)

	req := httptest.NewRequest("GET", "/health", nil)
	app.Test(req)

	startTime2 := handler.startTime
	assert.Equal(t, startTime1, startTime2)
}

func TestHealthHandler_TimestampFormat(t *testing.T) {
	cfg := createTestConfig()
	handler := NewHealthHandler(cfg)

	app := createTestApp()
	app.Get("/health", handler.HandleHealth)

	req := httptest.NewRequest("GET", "/health", nil)
	resp, err := app.Test(req)

	assert.NoError(t, err)

	body, _ := io.ReadAll(resp.Body)
	var health map[string]interface{}
	json.Unmarshal(body, &health)

	timestamp := health["timestamp"].(string)

	// Verify timestamp is in RFC3339 format
	_, err = time.Parse(time.RFC3339, timestamp)
	assert.NoError(t, err)
}

func TestHealthHandler_HTTPMethods(t *testing.T) {
	tests := []struct {
		name     string
		method   string
		endpoint string
		expected int
	}{
		{
			name:     "Health GET",
			method:   "GET",
			endpoint: "/health",
			expected: 200,
		},
		{
			name:     "Ready GET",
			method:   "GET",
			endpoint: "/ready",
			expected: 200,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := createTestConfig()
			handler := NewHealthHandler(cfg)

			app := createTestApp()
			app.Get("/health", handler.HandleHealth)
			app.Get("/ready", handler.HandleReady)

			req := httptest.NewRequest(tt.method, tt.endpoint, nil)
			resp, err := app.Test(req)

			assert.NoError(t, err)
			assert.Equal(t, tt.expected, resp.StatusCode)
		})
	}
}
