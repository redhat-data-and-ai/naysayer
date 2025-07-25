package webhook

import (
	"encoding/json"
	"io"
	"net/http/httptest"
	"testing"

	"github.com/redhat-data-and-ai/naysayer/internal/config"
	"github.com/stretchr/testify/assert"
)

func TestNewManagementHandler(t *testing.T) {
	cfg := createTestConfig()
	handler := NewManagementHandler(cfg)

	assert.NotNil(t, handler)
	assert.Equal(t, cfg, handler.config)
}

func TestManagementHandler_HandleRules(t *testing.T) {
	cfg := createTestConfig()
	handler := NewManagementHandler(cfg)

	app := createTestApp()
	app.Get("/api/rules", handler.HandleRules)

	req := httptest.NewRequest("GET", "/api/rules", nil)
	resp, err := app.Test(req)

	assert.NoError(t, err)
	assert.Equal(t, 200, resp.StatusCode)

	// Parse response
	body, _ := io.ReadAll(resp.Body)
	var response map[string]interface{}
	json.Unmarshal(body, &response)

	// Verify expected fields are present
	assert.NotNil(t, response["total_rules"])
	assert.NotNil(t, response["enabled_rules"])
	assert.NotNil(t, response["rules"])
	assert.NotNil(t, response["categories"])

	// Check that rules array exists and has expected structure
	rules := response["rules"].([]interface{})
	if len(rules) > 0 {
		rule := rules[0].(map[string]interface{})
		assert.NotNil(t, rule["name"])
		assert.NotNil(t, rule["description"])
		assert.NotNil(t, rule["version"])
		assert.NotNil(t, rule["enabled"])
		assert.NotNil(t, rule["category"])
	}

	// Check categories map
	categories := response["categories"].(map[string]interface{})
	assert.IsType(t, map[string]interface{}{}, categories)
}

func TestManagementHandler_HandleRulesEnabled(t *testing.T) {
	cfg := createTestConfig()
	handler := NewManagementHandler(cfg)

	app := createTestApp()
	app.Get("/api/rules/enabled", handler.HandleRulesEnabled)

	req := httptest.NewRequest("GET", "/api/rules/enabled", nil)
	resp, err := app.Test(req)

	assert.NoError(t, err)
	assert.Equal(t, 200, resp.StatusCode)

	// Parse response
	body, _ := io.ReadAll(resp.Body)
	var response map[string]interface{}
	json.Unmarshal(body, &response)

	// Verify expected fields are present
	assert.NotNil(t, response["enabled_rules"])
	assert.NotNil(t, response["rules"])

	// Check that rules array has expected structure
	rules := response["rules"].([]interface{})
	for _, ruleInterface := range rules {
		rule := ruleInterface.(map[string]interface{})
		assert.NotNil(t, rule["name"])
		assert.NotNil(t, rule["description"])
		assert.NotNil(t, rule["version"])
		assert.NotNil(t, rule["category"])
		// Enabled rules should not have "enabled" field since they're all enabled
	}
}

func TestManagementHandler_HandleRulesByCategory_Success(t *testing.T) {
	cfg := createTestConfig()
	handler := NewManagementHandler(cfg)

	app := createTestApp()
	app.Get("/api/rules/category/:category", handler.HandleRulesByCategory)

	// Test with a known category (based on the rules system)
	req := httptest.NewRequest("GET", "/api/rules/category/warehouse", nil)
	resp, err := app.Test(req)

	assert.NoError(t, err)
	assert.Equal(t, 200, resp.StatusCode)

	// Parse response
	body, _ := io.ReadAll(resp.Body)
	var response map[string]interface{}
	json.Unmarshal(body, &response)

	// Verify expected fields are present
	assert.Equal(t, "warehouse", response["category"])
	assert.NotNil(t, response["rule_count"])
	assert.NotNil(t, response["rules"])

	// Check that rules array has expected structure
	rules := response["rules"].([]interface{})
	for _, ruleInterface := range rules {
		rule := ruleInterface.(map[string]interface{})
		assert.NotNil(t, rule["name"])
		assert.NotNil(t, rule["description"])
		assert.NotNil(t, rule["version"])
		assert.NotNil(t, rule["enabled"])
	}
}

func TestManagementHandler_HandleRulesByCategory_EmptyCategory(t *testing.T) {
	cfg := createTestConfig()
	handler := NewManagementHandler(cfg)

	app := createTestApp()
	app.Get("/api/rules/category/:category", handler.HandleRulesByCategory)

	// Test with empty string as category (which should be handled by the handler)
	// We need to use a valid route that matches the pattern but with empty category
	req := httptest.NewRequest("GET", "/api/rules/category/", nil)
	resp, err := app.Test(req)

	assert.NoError(t, err)
	// When the route doesn't match, Fiber returns 404 or 500
	// This is actually testing Fiber's routing behavior, not our handler
	// Let's just verify it doesn't crash
	assert.True(t, resp.StatusCode >= 400)
}

func TestManagementHandler_HandleRulesByCategory_AdditionalEdgeCases(t *testing.T) {
	cfg := createTestConfig()
	handler := NewManagementHandler(cfg)

	app := createTestApp()
	app.Get("/api/rules/category/:category", handler.HandleRulesByCategory)

	// Additional test cases to improve coverage
	tests := []struct {
		name           string
		path           string
		expectedStatus int
		description    string
	}{
		{
			name:           "case sensitive category test",
			path:           "/api/rules/category/Warehouse",
			expectedStatus: 200,
			description:    "Test case sensitivity",
		},
		{
			name:           "category with numbers",
			path:           "/api/rules/category/test123",
			expectedStatus: 200,
			description:    "Test category with numbers",
		},
		{
			name:           "very long category name",
			path:           "/api/rules/category/very_long_category_name_that_does_not_exist_anywhere",
			expectedStatus: 200,
			description:    "Test very long category name",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", tt.path, nil)
			resp, err := app.Test(req)

			assert.NoError(t, err)
			assert.Equal(t, tt.expectedStatus, resp.StatusCode)

			if resp.StatusCode == 200 {
				body, _ := io.ReadAll(resp.Body)
				var response map[string]interface{}
				json.Unmarshal(body, &response)

				assert.NotNil(t, response["category"])
				assert.NotNil(t, response["rule_count"])
				assert.NotNil(t, response["rules"])
			}
		})
	}
}

func TestManagementHandler_HandleRulesByCategory_NonexistentCategory(t *testing.T) {
	cfg := createTestConfig()
	handler := NewManagementHandler(cfg)

	app := createTestApp()
	app.Get("/api/rules/category/:category", handler.HandleRulesByCategory)

	// Test with nonexistent category
	req := httptest.NewRequest("GET", "/api/rules/category/nonexistent", nil)
	resp, err := app.Test(req)

	assert.NoError(t, err)
	assert.Equal(t, 200, resp.StatusCode) // Should still return 200 with empty results

	body, _ := io.ReadAll(resp.Body)
	var response map[string]interface{}
	json.Unmarshal(body, &response)

	assert.Equal(t, "nonexistent", response["category"])
	assert.Equal(t, float64(0), response["rule_count"])
	assert.Empty(t, response["rules"])
}

func TestManagementHandler_HandleSystemInfo(t *testing.T) {
	cfg := createTestConfig()
	handler := NewManagementHandler(cfg)

	app := createTestApp()
	app.Get("/api/system", handler.HandleSystemInfo)

	req := httptest.NewRequest("GET", "/api/system", nil)
	resp, err := app.Test(req)

	assert.NoError(t, err)
	assert.Equal(t, 200, resp.StatusCode)

	// Parse response
	body, _ := io.ReadAll(resp.Body)
	var response map[string]interface{}
	json.Unmarshal(body, &response)

	// Verify expected top-level fields
	assert.Equal(t, "naysayer-webhook", response["service"])
	assert.Equal(t, "v1.0.0", response["version"])
	assert.NotNil(t, response["analysis_mode"])
	assert.NotNil(t, response["security_mode"])
	assert.NotNil(t, response["gitlab_configured"])
	assert.NotNil(t, response["webhook_security"])

	// Verify rule system information
	ruleSystem := response["rule_system"].(map[string]interface{})
	assert.NotNil(t, ruleSystem["total_rules"])
	assert.NotNil(t, ruleSystem["enabled_rules"])
	assert.NotNil(t, ruleSystem["total_categories"])
	assert.NotNil(t, ruleSystem["categories"])
	assert.Equal(t, true, ruleSystem["extensible"])
	assert.Equal(t, "registry-based", ruleSystem["framework"])

	// Verify endpoints list
	endpoints := response["endpoints"].([]interface{})
	expectedEndpoints := []string{
		"/health",
		"/ready",
		"/webhook",
		"/api/rules",
		"/api/rules/enabled",
		"/api/rules/category/:category",
		"/api/system",
	}

	assert.Len(t, endpoints, len(expectedEndpoints))
	for i, endpoint := range endpoints {
		assert.Equal(t, expectedEndpoints[i], endpoint)
	}
}

func TestManagementHandler_ContentTypes(t *testing.T) {
	cfg := createTestConfig()
	handler := NewManagementHandler(cfg)

	app := createTestApp()
	app.Get("/api/rules", handler.HandleRules)
	app.Get("/api/rules/enabled", handler.HandleRulesEnabled)
	app.Get("/api/rules/category/:category", handler.HandleRulesByCategory)
	app.Get("/api/system", handler.HandleSystemInfo)

	tests := []struct {
		name string
		path string
	}{
		{"Rules endpoint", "/api/rules"},
		{"Enabled rules endpoint", "/api/rules/enabled"},
		{"Category rules endpoint", "/api/rules/category/warehouse"},
		{"System info endpoint", "/api/system"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", tt.path, nil)
			resp, err := app.Test(req)

			assert.NoError(t, err)
			assert.Equal(t, "application/json", resp.Header.Get("Content-Type"))
		})
	}
}

func TestManagementHandler_RuleSystemConsistency(t *testing.T) {
	cfg := createTestConfig()
	handler := NewManagementHandler(cfg)

	app := createTestApp()
	app.Get("/api/rules", handler.HandleRules)
	app.Get("/api/rules/enabled", handler.HandleRulesEnabled)
	app.Get("/api/system", handler.HandleSystemInfo)

	// Get all rules
	req1 := httptest.NewRequest("GET", "/api/rules", nil)
	resp1, err := app.Test(req1)
	assert.NoError(t, err)

	body1, _ := io.ReadAll(resp1.Body)
	var allRulesResponse map[string]interface{}
	json.Unmarshal(body1, &allRulesResponse)

	// Get enabled rules
	req2 := httptest.NewRequest("GET", "/api/rules/enabled", nil)
	resp2, err := app.Test(req2)
	assert.NoError(t, err)

	body2, _ := io.ReadAll(resp2.Body)
	var enabledRulesResponse map[string]interface{}
	json.Unmarshal(body2, &enabledRulesResponse)

	// Get system info
	req3 := httptest.NewRequest("GET", "/api/system", nil)
	resp3, err := app.Test(req3)
	assert.NoError(t, err)

	body3, _ := io.ReadAll(resp3.Body)
	var systemInfoResponse map[string]interface{}
	json.Unmarshal(body3, &systemInfoResponse)

	// Check consistency between responses
	totalRules := allRulesResponse["total_rules"].(float64)
	enabledRules := allRulesResponse["enabled_rules"].(float64)
	enabledRulesFromEnabledEndpoint := enabledRulesResponse["enabled_rules"].(float64)

	ruleSystem := systemInfoResponse["rule_system"].(map[string]interface{})
	systemTotalRules := ruleSystem["total_rules"].(float64)
	systemEnabledRules := ruleSystem["enabled_rules"].(float64)

	// All should report the same numbers
	assert.Equal(t, totalRules, systemTotalRules)
	assert.Equal(t, enabledRules, enabledRulesFromEnabledEndpoint)
	assert.Equal(t, enabledRules, systemEnabledRules)

	// Enabled rules should be <= total rules
	assert.True(t, enabledRules <= totalRules)
}

func TestManagementHandler_ErrorHandling(t *testing.T) {
	cfg := createTestConfig()
	handler := NewManagementHandler(cfg)

	app := createTestApp()
	app.Get("/api/rules/category/:category", handler.HandleRulesByCategory)

	// Test invalid route patterns - these will be handled by Fiber's router
	tests := []struct {
		name     string
		path     string
		expected int
	}{
		{
			name:     "Malformed category route",
			path:     "/api/rules/category/",
			expected: 500, // Fiber routing behavior
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", tt.path, nil)
			resp, err := app.Test(req)

			assert.NoError(t, err)
			// Just verify we get an error status code
			assert.True(t, resp.StatusCode >= 400)
		})
	}
}

func TestManagementHandler_ConfigurationReflection(t *testing.T) {
	// Test that system info correctly reflects different configurations
	tests := []struct {
		name   string
		config *config.Config
		checks func(t *testing.T, response map[string]interface{})
	}{
		{
			name: "Secure configuration",
			config: &config.Config{
				GitLab: config.GitLabConfig{
					BaseURL: "https://gitlab.example.com",
					Token:   "secure-token",
				},
				Webhook: config.WebhookConfig{
					EnableVerification: true,
					Secret:             "webhook-secret",
				},
			},
			checks: func(t *testing.T, response map[string]interface{}) {
				assert.Equal(t, true, response["gitlab_configured"])
				assert.Equal(t, true, response["webhook_security"])
			},
		},
		{
			name: "Insecure configuration",
			config: &config.Config{
				GitLab: config.GitLabConfig{
					BaseURL: "https://gitlab.example.com",
					Token:   "",
				},
				Webhook: config.WebhookConfig{
					EnableVerification: false,
				},
			},
			checks: func(t *testing.T, response map[string]interface{}) {
				assert.Equal(t, false, response["gitlab_configured"])
				assert.Equal(t, false, response["webhook_security"])
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := NewManagementHandler(tt.config)

			app := createTestApp()
			app.Get("/api/system", handler.HandleSystemInfo)

			req := httptest.NewRequest("GET", "/api/system", nil)
			resp, err := app.Test(req)

			assert.NoError(t, err)
			assert.Equal(t, 200, resp.StatusCode)

			body, _ := io.ReadAll(resp.Body)
			var response map[string]interface{}
			json.Unmarshal(body, &response)

			tt.checks(t, response)
		})
	}
}
