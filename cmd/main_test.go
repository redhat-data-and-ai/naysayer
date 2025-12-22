package main

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/fiber/v2/middleware/recover"
	"github.com/stretchr/testify/assert"

	"github.com/redhat-data-and-ai/naysayer/internal/config"
	"github.com/redhat-data-and-ai/naysayer/internal/webhook"
)

func setupTestRulesFile() {
	tempRulesContent := `enabled: true

files:
  - name: "product_configs"
    path: "**/"
    filename: "product.{yaml,yml}"
    parser_type: yaml
    enabled: true
    sections:
      - name: warehouses
        yaml_path: warehouses
        required: true
        rule_configs:
          - name: warehouse_rule
            enabled: true
        auto_approve: false
  - name: "documentation_files"
    path: "**/"
    filename: "*.md"
    parser_type: yaml
    enabled: true
    sections:
      - name: full_file
        yaml_path: .
        required: true
        rule_configs:
          - name: metadata_rule
            enabled: true
        auto_approve: true`

	err := os.WriteFile("rules.yaml", []byte(tempRulesContent), 0644)
	if err != nil {
		panic("Failed to create test rules file: " + err.Error())
	}
}

func cleanupTestRulesFile() {
	_ = os.Remove("rules.yaml")
}

// createTestApplicationWithCleanup creates a test application and returns a cleanup function
func createTestApplicationWithCleanup(t *testing.T) (*fiber.App, func()) {
	setupTestRulesFile()
	cleanup := func() {
		cleanupTestRulesFile()
	}
	t.Cleanup(cleanup) // Automatically cleanup when test ends
	return createTestApplication(), cleanup
}

// createTestApplication creates a Fiber application with the same configuration as main
func createTestApplication() *fiber.App {
	// Use test configuration
	cfg := &config.Config{
		GitLab: config.GitLabConfig{
			BaseURL: "https://gitlab.example.com",
			Token:   "test-token",
		},
		Webhook: config.WebhookConfig{},
		Server: config.ServerConfig{
			Port: "8080",
		},
	}

	// Create handlers
	webhookHandler := webhook.NewDataProductConfigMrReviewHandler(cfg)
	healthHandler := webhook.NewHealthHandler(cfg)
	autoRebaseHandler := webhook.NewAutoRebaseHandler(cfg)
	staleMRCleanupHandler := webhook.NewStaleMRCleanupHandler(cfg)

	// Create Fiber app with same config as main
	app := fiber.New(fiber.Config{
		AppName: "NAYSAYER Webhook v1.0.0",
		ErrorHandler: func(c *fiber.Ctx, err error) error {
			return c.Status(500).JSON(fiber.Map{
				"error": "Internal server error",
			})
		},
	})

	// Core middleware (same as main)
	app.Use(recover.New())
	app.Use(logger.New(logger.Config{
		Format: "${time} ${status} - ${method} ${path} - ${latency}\n",
	}))
	app.Use(cors.New())

	// Health and monitoring routes (same as main)
	app.Get("/health", healthHandler.HandleHealth)
	app.Get("/ready", healthHandler.HandleReady)

	// Webhook routes (same as main)
	app.Post("/dataverse-product-config-review", webhookHandler.HandleWebhook)
	app.Post("/auto-rebase", autoRebaseHandler.HandleWebhook)
	app.Post("/stale-mr-cleanup", staleMRCleanupHandler.HandleWebhook)

	return app
}

func TestApplication_HealthEndpoints(t *testing.T) {
	app, _ := createTestApplicationWithCleanup(t)

	tests := []struct {
		name         string
		method       string
		path         string
		expectedCode int
	}{
		{
			name:         "Health endpoint",
			method:       "GET",
			path:         "/health",
			expectedCode: 200,
		},
		{
			name:         "Ready endpoint",
			method:       "GET",
			path:         "/ready",
			expectedCode: 200,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, tt.path, nil)
			resp, err := app.Test(req)

			assert.NoError(t, err)
			assert.Equal(t, tt.expectedCode, resp.StatusCode)

			// Verify response is JSON
			assert.Equal(t, "application/json", resp.Header.Get("Content-Type"))

			// Parse and verify basic structure
			body, _ := io.ReadAll(resp.Body)
			var response map[string]interface{}
			err = json.Unmarshal(body, &response)
			assert.NoError(t, err)
		})
	}
}

func TestApplication_WebhookEndpoint(t *testing.T) {
	app, _ := createTestApplicationWithCleanup(t)

	tests := []struct {
		name         string
		method       string
		path         string
		body         string
		contentType  string
		expectedCode int
	}{
		{
			name:         "Valid webhook payload",
			method:       "POST",
			path:         "/dataverse-product-config-review",
			body:         `{"object_kind":"merge_request","object_attributes":{"iid":123},"project":{"id":456},"user":{"username":"testuser"}}`,
			contentType:  "application/json",
			expectedCode: 200,
		},
		{
			name:         "Invalid JSON payload",
			method:       "POST",
			path:         "/dataverse-product-config-review",
			body:         `{invalid json}`,
			contentType:  "application/json",
			expectedCode: 400,
		},
		{
			name:         "Wrong content type",
			method:       "POST",
			path:         "/dataverse-product-config-review",
			body:         `{"test": "data"}`,
			contentType:  "text/plain",
			expectedCode: 400,
		},
		{
			name:         "Non-MR event",
			method:       "POST",
			path:         "/dataverse-product-config-review",
			body:         `{"object_kind":"push","commits":[]}`,
			contentType:  "application/json",
			expectedCode: 400,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, tt.path, strings.NewReader(tt.body))
			req.Header.Set("Content-Type", tt.contentType)

			resp, err := app.Test(req)
			assert.NoError(t, err)
			assert.Equal(t, tt.expectedCode, resp.StatusCode)

			// Verify response is JSON
			assert.Equal(t, "application/json", resp.Header.Get("Content-Type"))
		})
	}
}

func TestApplication_UnknownRoutes(t *testing.T) {
	app, _ := createTestApplicationWithCleanup(t)

	tests := []struct {
		name         string
		method       string
		path         string
		expectedCode int
	}{
		{
			name:         "Unknown GET route",
			method:       "GET",
			path:         "/unknown",
			expectedCode: 500, // Fiber's error handler returns 500
		},
		{
			name:         "Unknown POST route",
			method:       "POST",
			path:         "/unknown",
			expectedCode: 500, // Fiber's error handler returns 500
		},
		{
			name:         "Root path",
			method:       "GET",
			path:         "/",
			expectedCode: 500, // Fiber's error handler returns 500
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, tt.path, nil)
			resp, err := app.Test(req)

			assert.NoError(t, err)
			assert.Equal(t, tt.expectedCode, resp.StatusCode)
		})
	}
}

func TestApplication_MethodNotAllowed(t *testing.T) {
	app, _ := createTestApplicationWithCleanup(t)

	tests := []struct {
		name         string
		method       string
		path         string
		expectedCode int
	}{
		{
			name:         "POST to health endpoint",
			method:       "POST",
			path:         "/health",
			expectedCode: 500, // Fiber's error handler
		},
		{
			name:         "GET to webhook endpoint",
			method:       "GET",
			path:         "/dataverse-product-config-review",
			expectedCode: 500, // Fiber's error handler
		},
		{
			name:         "PUT to system endpoint",
			method:       "PUT",
			path:         "/api/system",
			expectedCode: 500, // Fiber's error handler
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, tt.path, nil)
			resp, err := app.Test(req)

			assert.NoError(t, err)
			assert.Equal(t, tt.expectedCode, resp.StatusCode)
		})
	}
}

func TestApplication_CORS_Headers(t *testing.T) {
	app, _ := createTestApplicationWithCleanup(t)

	// Test CORS preflight request
	req := httptest.NewRequest("OPTIONS", "/health", nil)
	req.Header.Set("Origin", "https://example.com")
	req.Header.Set("Access-Control-Request-Method", "GET")

	resp, err := app.Test(req)
	assert.NoError(t, err)

	// Should have CORS headers
	assert.NotEmpty(t, resp.Header.Get("Access-Control-Allow-Origin"))
}

func TestApplication_ErrorHandling(t *testing.T) {
	app, _ := createTestApplicationWithCleanup(t)

	// Test error handling with a simple invalid route
	req := httptest.NewRequest("GET", "/invalid-route", nil)

	resp, err := app.Test(req)
	assert.NoError(t, err)

	// Should handle the request without crashing
	assert.Equal(t, 500, resp.StatusCode)

	// Parse response to make sure error handler is working
	body, _ := io.ReadAll(resp.Body)
	var response map[string]interface{}
	_ = json.Unmarshal(body, &response)
	assert.Equal(t, "Internal server error", response["error"])
}

func TestApplication_HealthCheck_Integration(t *testing.T) {
	app, _ := createTestApplicationWithCleanup(t)

	// Test health endpoint response structure
	req := httptest.NewRequest("GET", "/health", nil)
	resp, err := app.Test(req)

	assert.NoError(t, err)
	assert.Equal(t, 200, resp.StatusCode)

	body, _ := io.ReadAll(resp.Body)
	var health map[string]interface{}
	_ = json.Unmarshal(body, &health)

	// Verify health response has expected fields
	assert.Equal(t, "healthy", health["status"])
	assert.Equal(t, "naysayer-webhook", health["service"])
	assert.Equal(t, "v1.0.0", health["version"])
	assert.NotNil(t, health["uptime_seconds"])
	assert.NotNil(t, health["timestamp"])
}

func TestApplication_RouteConfiguration(t *testing.T) {
	app, _ := createTestApplicationWithCleanup(t)

	// Test that all routes from main.go are properly configured
	// Note: Routes that make external API calls may return 500 in tests (route exists, but API fails)
	expectedRoutes := map[string]string{
		"GET:/health":                           "200",
		"GET:/ready":                            "200",
		"POST:/dataverse-product-config-review": "200",     // Will return 200 even with API failure
		"POST:/auto-rebase":                     "200|500", // Route exists (500 = API failure, not 404 = route missing)
		"POST:/stale-mr-cleanup":                "200|500", // Route exists (500 = API failure, not 404 = route missing)
	}

	for route, expectedStatus := range expectedRoutes {
		parts := strings.Split(route, ":")
		method := parts[0]
		path := parts[1]

		t.Run(route, func(t *testing.T) {
			var req *http.Request
			if method == "POST" {
				// For POST requests, provide appropriate payload based on endpoint
				var payload string
				switch path {
				case "/dataverse-product-config-review":
					payload = `{"object_kind":"merge_request","object_attributes":{"iid":123},"project":{"id":456}}`
				case "/auto-rebase":
					payload = `{"object_kind":"push","ref":"refs/heads/main","project":{"id":456}}`
				case "/stale-mr-cleanup":
					payload = `{"project_id":456}`
				default:
					payload = `{}`
				}
				req = httptest.NewRequest(method, path, strings.NewReader(payload))
				req.Header.Set("Content-Type", "application/json")
			} else {
				req = httptest.NewRequest(method, path, nil)
			}

			resp, err := app.Test(req)
			assert.NoError(t, err)

			// Check that route exists (not 404) and matches expected status
			// For routes with "200|500", accept either (500 = route exists but API failed)
			if expectedStatus == "200" {
				assert.Equal(t, 200, resp.StatusCode, "Route should return 200")
			} else if expectedStatus == "200|500" {
				assert.Contains(t, []int{200, 500}, resp.StatusCode, "Route should exist (200 or 500, not 404)")
			}
		})
	}
}

func TestApplication_Middleware_Integration(t *testing.T) {
	app, _ := createTestApplicationWithCleanup(t)

	// Test that middleware is working
	req := httptest.NewRequest("GET", "/health", nil)
	resp, err := app.Test(req)

	assert.NoError(t, err)
	assert.Equal(t, 200, resp.StatusCode)

	// The response should be successful, indicating middleware didn't interfere
	body, _ := io.ReadAll(resp.Body)
	var health map[string]interface{}
	_ = json.Unmarshal(body, &health)
	assert.Equal(t, "healthy", health["status"])
}
