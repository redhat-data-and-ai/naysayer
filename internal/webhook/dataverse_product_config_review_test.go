package webhook

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/redhat-data-and-ai/naysayer/internal/config"
	"github.com/redhat-data-and-ai/naysayer/internal/gitlab"
	"github.com/redhat-data-and-ai/naysayer/internal/rules/shared"
	"github.com/stretchr/testify/assert"
)

// MockRuleManager for testing
type MockRuleManager struct {
	evaluateFunc func(*shared.MRContext) *shared.RuleEvaluation
	rules        []shared.Rule
}

func (m *MockRuleManager) AddRule(rule shared.Rule) {
	m.rules = append(m.rules, rule)
}

func (m *MockRuleManager) EvaluateAll(ctx *shared.MRContext) *shared.RuleEvaluation {
	if m.evaluateFunc != nil {
		return m.evaluateFunc(ctx)
	}
	// Default behavior
	return &shared.RuleEvaluation{
		FinalDecision: shared.Decision{
			Type:    shared.Approve,
			Reason:  "Mock approval",
			Summary: "✅ Mock test passed",
			Details: "All mock rules passed",
		},
		FileValidations: map[string]*shared.FileValidationSummary{},
		ExecutionTime:   time.Millisecond * 10,
		TotalFiles:      0,
		ApprovedFiles:   0,
		ReviewFiles:     0,
		UncoveredFiles:  0,
	}
}

func createTestApp() *fiber.App {
	return fiber.New(fiber.Config{
		DisableStartupMessage: true,
		ErrorHandler: func(c *fiber.Ctx, err error) error {
			return c.Status(500).JSON(fiber.Map{"error": err.Error()})
		},
	})
}

func createTestConfig() *config.Config {
	return &config.Config{
		GitLab: config.GitLabConfig{
			BaseURL: "https://gitlab.example.com",
			Token:   "test-token",
		},
		Webhook: config.WebhookConfig{

			AllowedIPs: []string{},
		},
	}
}

func setupTestRulesFile(t *testing.T) {
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
		t.Fatalf("Failed to create test rules file: %v", err)
	}
	t.Cleanup(func() { _ = os.Remove("rules.yaml") })
}

func TestNewWebhookHandler(t *testing.T) {
	setupTestRulesFile(t)

	cfg := createTestConfig()
	handler := NewDataProductConfigMrReviewHandler(cfg)

	assert.NotNil(t, handler)
	assert.NotNil(t, handler.gitlabClient)
	assert.NotNil(t, handler.ruleManager)
	assert.Equal(t, cfg, handler.config)
}

func TestWebhookHandler_HandleWebhook_Success(t *testing.T) {
	setupTestRulesFile(t)
	cfg := createTestConfig()
	handler := NewDataProductConfigMrReviewHandler(cfg)

	app := createTestApp()
	app.Post("/webhook", handler.HandleWebhook)

	// Create valid MR webhook payload
	payload := map[string]interface{}{
		"object_kind": "merge_request",
		"object_attributes": map[string]interface{}{
			"iid":           123,
			"title":         "Update warehouse configuration",
			"source_branch": "feature/update",
			"target_branch": "main",
			"state":         "opened",
		},
		"project": map[string]interface{}{
			"id": 456,
		},
		"user": map[string]interface{}{
			"username": "testuser",
		},
	}

	jsonData, _ := json.Marshal(payload)
	req := httptest.NewRequest("POST", "/webhook", bytes.NewReader(jsonData))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	assert.NoError(t, err)
	assert.Equal(t, 200, resp.StatusCode)

	// Parse response
	body, _ := io.ReadAll(resp.Body)
	var response map[string]interface{}
	_ = json.Unmarshal(body, &response)

	assert.Equal(t, "processed", response["webhook_response"])
	assert.NotNil(t, response["decision"])
	assert.NotNil(t, response["execution_time"])
	assert.NotNil(t, response["rules_evaluated"])

	// Since GitLab API will fail, expect manual review decision
	decision := response["decision"].(map[string]interface{})
	assert.Equal(t, "manual_review", decision["type"])
	assert.Contains(t, decision["reason"], "Could not fetch MR changes from GitLab API")
}

func TestWebhookHandler_HandleWebhook_InvalidContentType(t *testing.T) {
	setupTestRulesFile(t)
	cfg := createTestConfig()
	handler := NewDataProductConfigMrReviewHandler(cfg)

	app := createTestApp()
	app.Post("/webhook", handler.HandleWebhook)

	req := httptest.NewRequest("POST", "/webhook", bytes.NewReader([]byte("test")))
	req.Header.Set("Content-Type", "text/plain")

	resp, err := app.Test(req)
	assert.NoError(t, err)
	assert.Equal(t, 400, resp.StatusCode)

	body, _ := io.ReadAll(resp.Body)
	var response map[string]interface{}
	_ = json.Unmarshal(body, &response)

	assert.Contains(t, response["error"], "Content-Type must be application/json")
}

func TestWebhookHandler_HandleWebhook_InvalidJSON(t *testing.T) {
	setupTestRulesFile(t)
	cfg := createTestConfig()
	handler := NewDataProductConfigMrReviewHandler(cfg)

	app := createTestApp()
	app.Post("/webhook", handler.HandleWebhook)

	req := httptest.NewRequest("POST", "/webhook", bytes.NewReader([]byte("{invalid json")))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	assert.NoError(t, err)
	assert.Equal(t, 400, resp.StatusCode)

	body, _ := io.ReadAll(resp.Body)
	var response map[string]interface{}
	_ = json.Unmarshal(body, &response)

	assert.Contains(t, response["error"], "Invalid JSON payload")
}

func TestWebhookHandler_HandleWebhook_NonMREvent(t *testing.T) {
	setupTestRulesFile(t)
	cfg := createTestConfig()
	handler := NewDataProductConfigMrReviewHandler(cfg)

	app := createTestApp()
	app.Post("/webhook", handler.HandleWebhook)

	// Create non-MR event payload
	payload := map[string]interface{}{
		"object_kind": "push",
		"commits":     []interface{}{},
	}

	jsonData, _ := json.Marshal(payload)
	req := httptest.NewRequest("POST", "/webhook", bytes.NewReader(jsonData))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	assert.NoError(t, err)
	assert.Equal(t, 400, resp.StatusCode)

	body, _ := io.ReadAll(resp.Body)
	var response map[string]interface{}
	_ = json.Unmarshal(body, &response)

	assert.Contains(t, response["error"], "missing object_attributes")
}

func TestWebhookHandler_HandleWebhook_InvalidMRInfo(t *testing.T) {
	setupTestRulesFile(t)
	cfg := createTestConfig()
	handler := NewDataProductConfigMrReviewHandler(cfg)

	app := createTestApp()
	app.Post("/webhook", handler.HandleWebhook)

	// Create MR payload with missing required fields
	payload := map[string]interface{}{
		"object_kind": "merge_request",
		"object_attributes": map[string]interface{}{
			"title": "Test MR",
			// Missing iid
		},
		// Missing project
	}

	jsonData, _ := json.Marshal(payload)
	req := httptest.NewRequest("POST", "/webhook", bytes.NewReader(jsonData))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	assert.NoError(t, err)
	assert.Equal(t, 400, resp.StatusCode)

	body, _ := io.ReadAll(resp.Body)
	var response map[string]interface{}
	_ = json.Unmarshal(body, &response)

	assert.Contains(t, response["error"], "missing project")
}

func TestWebhookHandler_HandleWebhook_APIFailureHandling(t *testing.T) {
	// Test that the webhook handler correctly handles GitLab API failures
	// by returning a manual review decision when it can't fetch MR changes
	setupTestRulesFile(t)
	cfg := createTestConfig()
	handler := NewDataProductConfigMrReviewHandler(cfg)

	app := createTestApp()
	app.Post("/webhook", handler.HandleWebhook)

	payload := map[string]interface{}{
		"object_kind": "merge_request",
		"object_attributes": map[string]interface{}{
			"iid":           123,
			"title":         "Test MR",
			"source_branch": "feature/test",
			"target_branch": "main",
			"state":         "opened",
		},
		"project": map[string]interface{}{
			"id": 456,
		},
		"user": map[string]interface{}{
			"username": "testuser",
		},
	}

	jsonData, _ := json.Marshal(payload)
	req := httptest.NewRequest("POST", "/webhook", bytes.NewReader(jsonData))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	assert.NoError(t, err)
	assert.Equal(t, 200, resp.StatusCode)

	body, _ := io.ReadAll(resp.Body)
	var response map[string]interface{}
	_ = json.Unmarshal(body, &response)

	// When GitLab API fails, should return manual review decision
	decision := response["decision"].(map[string]interface{})
	assert.Equal(t, "manual_review", decision["type"])
	assert.Contains(t, decision["reason"], "Could not fetch MR changes from GitLab API")
	assert.Equal(t, "processed", response["webhook_response"])
	assert.NotNil(t, response["execution_time"])
}

func TestWebhookHandler_HandleWebhook_LargePayload(t *testing.T) {
	setupTestRulesFile(t)
	cfg := createTestConfig()
	handler := NewDataProductConfigMrReviewHandler(cfg)

	app := createTestApp()
	app.Post("/webhook", handler.HandleWebhook)

	// Create a large payload with many changes
	payload := map[string]interface{}{
		"object_kind": "merge_request",
		"object_attributes": map[string]interface{}{
			"iid":           123,
			"title":         "Large MR with many changes",
			"source_branch": "feature/large-update",
			"target_branch": "main",
			"state":         "opened",
			"description":   "This is a large MR with extensive changes across multiple files and directories for testing purposes.",
		},
		"project": map[string]interface{}{
			"id":   456,
			"name": "test-project",
			"path": "test/project",
		},
		"user": map[string]interface{}{
			"username": "testuser",
			"name":     "Test User",
			"email":    "test@example.com",
		},
	}

	jsonData, _ := json.Marshal(payload)
	req := httptest.NewRequest("POST", "/webhook", bytes.NewReader(jsonData))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	assert.NoError(t, err)
	assert.Equal(t, 200, resp.StatusCode)

	// Should handle large payloads correctly and return manual review due to API failure
	body, _ := io.ReadAll(resp.Body)
	var response map[string]interface{}
	_ = json.Unmarshal(body, &response)

	assert.Equal(t, "processed", response["webhook_response"])
	decision := response["decision"].(map[string]interface{})
	assert.Equal(t, "manual_review", decision["type"])
}

func TestWebhookHandler_ContentTypeVariations(t *testing.T) {
	tests := []struct {
		name        string
		contentType string
		expectError bool
	}{
		{
			name:        "Standard JSON content type",
			contentType: "application/json",
			expectError: false,
		},
		{
			name:        "JSON with charset",
			contentType: "application/json; charset=utf-8",
			expectError: false,
		},
		{
			name:        "Plain text",
			contentType: "text/plain",
			expectError: true,
		},
		{
			name:        "Form data",
			contentType: "application/x-www-form-urlencoded",
			expectError: true,
		},
		{
			name:        "Empty content type",
			contentType: "",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setupTestRulesFile(t)
			cfg := createTestConfig()
			handler := NewDataProductConfigMrReviewHandler(cfg)

			app := createTestApp()
			app.Post("/webhook", handler.HandleWebhook)

			payload := map[string]interface{}{
				"object_kind": "merge_request",
				"object_attributes": map[string]interface{}{
					"iid": 123,
				},
				"project": map[string]interface{}{
					"id": 456,
				},
			}

			jsonData, _ := json.Marshal(payload)
			req := httptest.NewRequest("POST", "/webhook", bytes.NewReader(jsonData))
			req.Header.Set("Content-Type", tt.contentType)

			resp, err := app.Test(req)
			assert.NoError(t, err)

			if tt.expectError {
				assert.Equal(t, 400, resp.StatusCode)
			} else {
				// Should pass content type check (might fail later for other reasons)
				assert.NotEqual(t, 400, resp.StatusCode)
			}
		})
	}
}

// MockGitLabClient for testing evaluateRules with custom changes
type MockGitLabClient struct {
	changes []gitlab.FileChange
	err     error
}

func (m *MockGitLabClient) FetchFileContent(projectID int, filePath, ref string) (*gitlab.FileContent, error) {
	return nil, nil
}

func (m *MockGitLabClient) GetMRTargetBranch(projectID, mrIID int) (string, error) {
	return "main", nil
}

func (m *MockGitLabClient) GetMRDetails(projectID, mrIID int) (*gitlab.MRDetails, error) {
	return nil, nil
}

func (m *MockGitLabClient) FetchMRChanges(projectID, mrIID int) ([]gitlab.FileChange, error) {
	return m.changes, m.err
}

func (m *MockGitLabClient) AddMRComment(projectID, mrIID int, comment string) error {
	return nil
}

func (m *MockGitLabClient) AddOrUpdateMRComment(projectID, mrIID int, commentBody, commentType string) error {
	return nil
}

func (m *MockGitLabClient) ListMRComments(projectID, mrIID int) ([]gitlab.MRComment, error) {
	return nil, nil
}

func (m *MockGitLabClient) UpdateMRComment(projectID, mrIID, commentID int, newBody string) error {
	return nil
}

func (m *MockGitLabClient) FindLatestNaysayerComment(projectID, mrIID int, commentType ...string) (*gitlab.MRComment, error) {
	return nil, nil
}

func (m *MockGitLabClient) ApproveMR(projectID, mrIID int) error {
	return nil
}

func (m *MockGitLabClient) ApproveMRWithMessage(projectID, mrIID int, message string) error {
	return nil
}

func (m *MockGitLabClient) ResetNaysayerApproval(projectID, mrIID int) error {
	return nil
}

func (m *MockGitLabClient) GetCurrentBotUsername() (string, error) {
	return "naysayer-bot", nil
}

func (m *MockGitLabClient) IsNaysayerBotAuthor(author map[string]interface{}) bool {
	return false
}

func (m *MockGitLabClient) RebaseMR(projectID, mrIID int) error {
	return nil
}

func (m *MockGitLabClient) ListOpenMRs(projectID int) ([]int, error) {
	return nil, nil
}

func (m *MockGitLabClient) ListOpenMRsWithDetails(projectID int) ([]gitlab.MRDetails, error) {
	return nil, nil
}

// Test empty MR detection
func TestEvaluateRules_EmptyMR(t *testing.T) {
	setupTestRulesFile(t)
	cfg := createTestConfig()

	// Mock client returns empty changes array
	mockClient := &MockGitLabClient{
		changes: []gitlab.FileChange{}, // Empty - no files changed
		err:     nil,
	}

	handler := NewDataProductConfigMrReviewHandlerWithClient(cfg, mockClient)

	mrInfo := &gitlab.MRInfo{
		ProjectID:    456,
		MRIID:        123,
		Title:        "Test Empty MR",
		Author:       "testuser",
		SourceBranch: "feature/test",
		TargetBranch: "main",
		State:        "opened",
	}

	result, err := handler.evaluateRules(456, 123, mrInfo)

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, shared.ManualReview, result.FinalDecision.Type)
	assert.Contains(t, result.FinalDecision.Reason, "no file changes")
	assert.Equal(t, "Empty MR", result.FinalDecision.Summary)
}

// Test net-zero changes detection
func TestEvaluateRules_NetZeroChanges(t *testing.T) {
	setupTestRulesFile(t)
	cfg := createTestConfig()

	// Mock client returns file changes but all diffs are empty
	mockClient := &MockGitLabClient{
		changes: []gitlab.FileChange{
			{NewPath: "file1.txt", Diff: ""},
			{NewPath: "file2.txt", Diff: ""},
		},
		err: nil,
	}

	handler := NewDataProductConfigMrReviewHandlerWithClient(cfg, mockClient)

	mrInfo := &gitlab.MRInfo{
		ProjectID:    456,
		MRIID:        124,
		Title:        "Test Net-Zero MR",
		Author:       "testuser",
		SourceBranch: "feature/net-zero",
		TargetBranch: "main",
		State:        "opened",
	}

	result, err := handler.evaluateRules(456, 124, mrInfo)

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, shared.ManualReview, result.FinalDecision.Type)
	assert.Contains(t, result.FinalDecision.Reason, "no substantive changes")
	assert.Equal(t, "Net-zero changes", result.FinalDecision.Summary)
}
