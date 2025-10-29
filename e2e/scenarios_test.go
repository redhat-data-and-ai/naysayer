package e2e

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/redhat-data-and-ai/naysayer/internal/config"
	"github.com/redhat-data-and-ai/naysayer/internal/gitlab"
	"github.com/redhat-data-and-ai/naysayer/internal/webhook"
	"github.com/stretchr/testify/assert"
)

// TestE2E_Scenarios runs all E2E test scenarios
func TestE2E_Scenarios(t *testing.T) {
	// Load all scenarios
	testdataPath := filepath.Join("testdata")
	scenarios, err := LoadScenarios(testdataPath)
	if err != nil {
		t.Skipf("No scenarios found (this is ok for initial setup): %v", err)
		return
	}

	if len(scenarios) == 0 {
		t.Skip("No scenarios found - create scenarios in testdata/scenarios/")
		return
	}

	t.Logf("Found %d scenarios to test", len(scenarios))

	// Run each scenario
	for _, scenario := range scenarios {
		t.Run(scenario.Name, func(t *testing.T) {
			runScenario(t, scenario)
		})
	}
}

// runScenario executes a single E2E scenario
func runScenario(t *testing.T, scenario ScenarioConfig) {
	// 1. Compare before/after folders to generate file changes
	changes, err := CompareFolders(scenario.BeforeDir, scenario.AfterDir)
	if err != nil {
		t.Fatalf("Failed to compare folders: %v", err)
	}

	if len(changes) == 0 {
		t.Fatal("No file changes detected - check before/after directories")
	}

	t.Logf("Detected %d file changes", len(changes))
	for _, change := range changes {
		if change.NewFile {
			t.Logf("  [NEW] %s", change.NewPath)
		} else if change.DeletedFile {
			t.Logf("  [DEL] %s", change.OldPath)
		} else {
			t.Logf("  [MOD] %s", change.NewPath)
		}
	}

	// 2. Create mock GitLab client with before/ and after/ directories
	mockGitLab := NewMockGitLabClient(scenario.BeforeDir, scenario.AfterDir)
	mockGitLab.SetMRBranches(scenario.MRMetadata.SourceBranch, scenario.MRMetadata.TargetBranch)
	mockGitLab.SetFileChanges(changes)

	// 3. Create test configuration
	testConfig := createTestConfig(t)

	// 4. Create webhook handler with mock client
	handler := createWebhookHandler(t, testConfig, mockGitLab)

	// 5. Create webhook payload
	payload := createWebhookPayload(scenario, changes)

	// 6. Call webhook endpoint
	response := callWebhookEndpoint(t, handler, payload)

	// 7. Validate decision
	validateDecision(t, scenario, response)

	// 8. Validate approval status
	validateApprovalStatus(t, scenario, response, mockGitLab)

	// 9. Validate MR comment
	validateMRComment(t, scenario, mockGitLab)

	t.Logf("✅ Scenario '%s' passed all validations", scenario.Name)
}

// createTestConfig creates a test configuration
func createTestConfig(t *testing.T) *config.Config {
	return &config.Config{
		GitLab: config.GitLabConfig{
			BaseURL: "https://gitlab.example.com",
			Token:   "test-token",
		},
		Webhook: config.WebhookConfig{
			AllowedIPs: []string{},
		},
		Comments: config.CommentsConfig{
			EnableMRComments:       true,
			UpdateExistingComments: true,
			CommentVerbosity:       "detailed",
		},
		Server: config.ServerConfig{
			Port: "3000",
		},
	}
}

// createWebhookHandler creates a webhook handler with mock GitLab client
func createWebhookHandler(t *testing.T, cfg *config.Config, mockGitLab *MockGitLabClient) *webhook.DataProductConfigMrReviewHandler {
	// Use the new constructor that accepts a GitLab client
	handler := webhook.NewDataProductConfigMrReviewHandlerWithClient(cfg, mockGitLab)
	if handler == nil {
		t.Fatal("Failed to create webhook handler")
	}
	return handler
}

// createWebhookPayload creates a GitLab webhook payload
func createWebhookPayload(scenario ScenarioConfig, changes []gitlab.FileChange) map[string]interface{} {
	return map[string]interface{}{
		"object_kind": "merge_request",
		"object_attributes": map[string]interface{}{
			"iid":              123,
			"title":            scenario.MRMetadata.Title,
			"source_branch":    scenario.MRMetadata.SourceBranch,
			"target_branch":    scenario.MRMetadata.TargetBranch,
			"state":            "opened",
			"work_in_progress": false,
		},
		"project": map[string]interface{}{
			"id":   456,
			"name": "test-project",
		},
		"user": map[string]interface{}{
			"username": scenario.MRMetadata.Author,
			"name":     "Test User",
		},
	}
}

// callWebhookEndpoint calls the webhook endpoint and returns the response
func callWebhookEndpoint(t *testing.T, handler *webhook.DataProductConfigMrReviewHandler, payload map[string]interface{}) map[string]interface{} {
	// Create Fiber app
	app := fiber.New(fiber.Config{
		DisableStartupMessage: true,
		ErrorHandler: func(c *fiber.Ctx, err error) error {
			return c.Status(500).JSON(fiber.Map{"error": err.Error()})
		},
	})

	// Setup route
	app.Post("/webhook", handler.HandleWebhook)

	// Marshal payload
	jsonData, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("Failed to marshal payload: %v", err)
	}

	// Create HTTP request
	req := httptest.NewRequest("POST", "/webhook", bytes.NewReader(jsonData))
	req.Header.Set("Content-Type", "application/json")

	// Execute request
	resp, err := app.Test(req, 30000) // 30 second timeout
	if err != nil {
		t.Fatalf("Failed to execute webhook request: %v", err)
	}

	// Read response
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("Failed to read response body: %v", err)
	}

	// Parse response
	var response map[string]interface{}
	err = json.Unmarshal(body, &response)
	if err != nil {
		t.Fatalf("Failed to unmarshal response: %s - %v", string(body), err)
	}

	return response
}

// validateDecision validates the decision matches expected
func validateDecision(t *testing.T, scenario ScenarioConfig, response map[string]interface{}) {
	// Extract decision
	decision, ok := response["decision"].(map[string]interface{})
	if !ok {
		t.Fatalf("Expected 'decision' to be a map, got %T", response["decision"])
	}

	// Validate decision type
	decisionType, ok := decision["type"].(string)
	if !ok {
		t.Fatalf("Expected 'decision.type' to be a string, got %T", decision["type"])
	}

	expectedType := string(scenario.Expected.Decision)
	assert.Equal(t, expectedType, decisionType,
		"Expected decision type %s, got %s", expectedType, decisionType)

	// Validate decision reason (if specified)
	if scenario.Expected.Reason != "" {
		reason, ok := decision["reason"].(string)
		if !ok {
			t.Fatalf("Expected 'decision.reason' to be a string, got %T", decision["reason"])
		}

		assert.Contains(t, reason, scenario.Expected.Reason,
			"Expected reason to contain '%s', got '%s'", scenario.Expected.Reason, reason)
	}

	t.Logf("✅ Decision: %s (%s)", decisionType, decision["reason"])
}

// validateApprovalStatus validates the approval status
func validateApprovalStatus(t *testing.T, scenario ScenarioConfig, response map[string]interface{}, mockGitLab *MockGitLabClient) {
	// Check response mr_approved field
	mrApproved, ok := response["mr_approved"].(bool)
	if !ok {
		t.Fatalf("Expected 'mr_approved' to be a bool, got %T", response["mr_approved"])
	}

	assert.Equal(t, scenario.Expected.Approved, mrApproved,
		"Expected approval status %v, got %v", scenario.Expected.Approved, mrApproved)

	// Verify mock GitLab client captured approval correctly
	assert.Equal(t, scenario.Expected.Approved, mockGitLab.WasApproved(),
		"Mock GitLab approval status mismatch")

	if scenario.Expected.Approved {
		t.Logf("✅ MR was approved")
	} else {
		t.Logf("✅ MR was not approved (manual review required)")
	}
}

// validateMRComment validates the MR comment
func validateMRComment(t *testing.T, scenario ScenarioConfig, mockGitLab *MockGitLabClient) {
	// Get captured comments
	comments := mockGitLab.GetAllComments()
	if len(comments) == 0 {
		t.Fatal("Expected at least one comment to be posted")
	}

	actualComment := comments[0]

	// If expected_comment.txt exists, validate exact match
	if scenario.Expected.CommentFile != "" {
		expectedComment, err := LoadExpectedComment(scenario.Expected.CommentFile)
		if err == nil && expectedComment != "" {
			// Normalize whitespace for comparison
			normalizedExpected := normalizeWhitespace(expectedComment)
			normalizedActual := normalizeWhitespace(actualComment)

			assert.Equal(t, normalizedExpected, normalizedActual,
				"Comment mismatch.\nExpected:\n%s\n\nGot:\n%s",
				expectedComment, actualComment)

			t.Logf("✅ Comment matches expected_comment.txt")
			return
		}
	}

	// Otherwise, validate that comment contains expected phrases
	if len(scenario.Expected.CommentContains) > 0 {
		for _, phrase := range scenario.Expected.CommentContains {
			assert.Contains(t, actualComment, phrase,
				"Expected comment to contain '%s'", phrase)
		}
		t.Logf("✅ Comment contains all expected phrases (%d)", len(scenario.Expected.CommentContains))
	}
}

// normalizeWhitespace normalizes whitespace for comment comparison
func normalizeWhitespace(s string) string {
	// Trim leading/trailing whitespace
	s = strings.TrimSpace(s)

	// Normalize line endings
	s = strings.ReplaceAll(s, "\r\n", "\n")

	// Remove trailing whitespace from each line
	lines := strings.Split(s, "\n")
	for i, line := range lines {
		lines[i] = strings.TrimRight(line, " \t")
	}

	return strings.Join(lines, "\n")
}
