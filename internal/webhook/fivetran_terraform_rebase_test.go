package webhook

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/redhat-data-and-ai/naysayer/internal/config"
	"github.com/redhat-data-and-ai/naysayer/internal/gitlab"
)

// MockRebaseGitLabClient is a mock GitLab client for rebase testing
type MockRebaseGitLabClient struct {
	rebaseError       error
	addCommentError   error
	capturedComments  []string
	capturedRebaseMRs []struct {
		projectID int
		mrIID     int
	}
}

func (m *MockRebaseGitLabClient) RebaseMR(projectID, mrIID int) error {
	m.capturedRebaseMRs = append(m.capturedRebaseMRs, struct {
		projectID int
		mrIID     int
	}{projectID, mrIID})
	return m.rebaseError
}

func (m *MockRebaseGitLabClient) AddMRComment(projectID, mrIID int, comment string) error {
	m.capturedComments = append(m.capturedComments, comment)
	return m.addCommentError
}

// Stub implementations for required interface methods
func (m *MockRebaseGitLabClient) FetchFileContent(projectID int, filePath, ref string) (*gitlab.FileContent, error) {
	return nil, nil
}

func (m *MockRebaseGitLabClient) GetMRTargetBranch(projectID, mrIID int) (string, error) {
	return "main", nil
}

func (m *MockRebaseGitLabClient) GetMRDetails(projectID, mrIID int) (*gitlab.MRDetails, error) {
	return nil, nil
}

func (m *MockRebaseGitLabClient) FetchMRChanges(projectID, mrIID int) ([]gitlab.FileChange, error) {
	return []gitlab.FileChange{}, nil
}

func (m *MockRebaseGitLabClient) AddOrUpdateMRComment(projectID, mrIID int, commentBody, commentType string) error {
	return nil
}

func (m *MockRebaseGitLabClient) ListMRComments(projectID, mrIID int) ([]gitlab.MRComment, error) {
	return []gitlab.MRComment{}, nil
}

func (m *MockRebaseGitLabClient) UpdateMRComment(projectID, mrIID, commentID int, newBody string) error {
	return nil
}

func (m *MockRebaseGitLabClient) FindLatestNaysayerComment(projectID, mrIID int, commentType ...string) (*gitlab.MRComment, error) {
	return nil, nil
}

func (m *MockRebaseGitLabClient) ApproveMR(projectID, mrIID int) error {
	return nil
}

func (m *MockRebaseGitLabClient) ApproveMRWithMessage(projectID, mrIID int, message string) error {
	return nil
}

func (m *MockRebaseGitLabClient) ResetNaysayerApproval(projectID, mrIID int) error {
	return nil
}

func (m *MockRebaseGitLabClient) GetCurrentBotUsername() (string, error) {
	return "naysayer-bot", nil
}

func (m *MockRebaseGitLabClient) IsNaysayerBotAuthor(author map[string]interface{}) bool {
	return false
}

func TestNewFivetranTerraformRebaseHandler(t *testing.T) {
	cfg := createTestConfig()
	handler := NewFivetranTerraformRebaseHandlerWithClient(cfg, &MockRebaseGitLabClient{})

	assert.NotNil(t, handler)
	assert.Equal(t, cfg, handler.config)
	assert.NotNil(t, handler.gitlabClient)
}

func TestFivetranTerraformRebaseHandler_HandleWebhook_Success(t *testing.T) {
	cfg := &config.Config{
		GitLab: config.GitLabConfig{
			BaseURL: "https://gitlab.example.com",
			Token:   "test-token",
		},
		Comments: config.CommentsConfig{
			EnableMRComments: true,
		},
	}

	mockClient := &MockRebaseGitLabClient{}
	handler := NewFivetranTerraformRebaseHandlerWithClient(cfg, mockClient)

	app := createTestApp()
	app.Post("/rebase", handler.HandleWebhook)

	payload := map[string]interface{}{
		"object_kind": "merge_request",
		"object_attributes": map[string]interface{}{
			"iid":   123,
			"state": "opened",
		},
		"project": map[string]interface{}{
			"id": 456,
		},
		"user": map[string]interface{}{
			"username": "testuser",
		},
	}

	payloadBytes, _ := json.Marshal(payload)
	req := httptest.NewRequest("POST", "/rebase", bytes.NewReader(payloadBytes))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	assert.NoError(t, err)
	assert.Equal(t, 200, resp.StatusCode)

	// Parse response
	body, _ := io.ReadAll(resp.Body)
	var response map[string]interface{}
	_ = json.Unmarshal(body, &response)

	assert.Equal(t, "processed", response["webhook_response"])
	assert.Equal(t, "merge_request_rebase", response["event_type"])
	assert.Equal(t, "success", response["status"])
	assert.Equal(t, true, response["rebased"])
	assert.Equal(t, float64(456), response["project_id"])
	assert.Equal(t, float64(123), response["mr_iid"])

	// Verify rebase was called
	assert.Len(t, mockClient.capturedRebaseMRs, 1)
	assert.Equal(t, 456, mockClient.capturedRebaseMRs[0].projectID)
	assert.Equal(t, 123, mockClient.capturedRebaseMRs[0].mrIID)

	// Verify comment was added
	assert.Len(t, mockClient.capturedComments, 1)
	assert.Contains(t, mockClient.capturedComments[0], "Naysayer Rebase Triggered")
}

func TestFivetranTerraformRebaseHandler_HandleWebhook_NonOpenMR(t *testing.T) {
	cfg := createTestConfig()
	mockClient := &MockRebaseGitLabClient{}
	handler := NewFivetranTerraformRebaseHandlerWithClient(cfg, mockClient)

	app := createTestApp()
	app.Post("/rebase", handler.HandleWebhook)

	payload := map[string]interface{}{
		"object_kind": "merge_request",
		"object_attributes": map[string]interface{}{
			"iid":   123,
			"state": "merged",
		},
		"project": map[string]interface{}{
			"id": 456,
		},
		"user": map[string]interface{}{
			"username": "testuser",
		},
	}

	payloadBytes, _ := json.Marshal(payload)
	req := httptest.NewRequest("POST", "/rebase", bytes.NewReader(payloadBytes))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	assert.NoError(t, err)
	assert.Equal(t, 200, resp.StatusCode)

	// Parse response
	body, _ := io.ReadAll(resp.Body)
	var response map[string]interface{}
	_ = json.Unmarshal(body, &response)

	assert.Equal(t, "skipped", response["status"])
	assert.Equal(t, false, response["rebased"])

	// Verify rebase was NOT called
	assert.Len(t, mockClient.capturedRebaseMRs, 0)
}

func TestFivetranTerraformRebaseHandler_HandleWebhook_RebaseError(t *testing.T) {
	cfg := &config.Config{
		GitLab: config.GitLabConfig{
			BaseURL: "https://gitlab.example.com",
			Token:   "test-token",
		},
		Comments: config.CommentsConfig{
			EnableMRComments: true,
		},
	}

	mockClient := &MockRebaseGitLabClient{
		rebaseError: fmt.Errorf("rebase failed: conflicts detected"),
	}
	handler := NewFivetranTerraformRebaseHandlerWithClient(cfg, mockClient)

	app := createTestApp()
	app.Post("/rebase", handler.HandleWebhook)

	payload := map[string]interface{}{
		"object_kind": "merge_request",
		"object_attributes": map[string]interface{}{
			"iid":   123,
			"state": "opened",
		},
		"project": map[string]interface{}{
			"id": 456,
		},
		"user": map[string]interface{}{
			"username": "testuser",
		},
	}

	payloadBytes, _ := json.Marshal(payload)
	req := httptest.NewRequest("POST", "/rebase", bytes.NewReader(payloadBytes))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	assert.NoError(t, err)
	assert.Equal(t, 500, resp.StatusCode)

	// Parse response
	body, _ := io.ReadAll(resp.Body)
	var response map[string]interface{}
	_ = json.Unmarshal(body, &response)

	assert.Equal(t, false, response["rebased"])
	assert.Contains(t, response["error"].(string), "conflicts detected")

	// Verify failure comment was added
	assert.Len(t, mockClient.capturedComments, 1)
	assert.Contains(t, mockClient.capturedComments[0], "Naysayer Rebase Failed")
}

func TestFivetranTerraformRebaseHandler_HandleWebhook_InvalidContentType(t *testing.T) {
	cfg := createTestConfig()
	mockClient := &MockRebaseGitLabClient{}
	handler := NewFivetranTerraformRebaseHandlerWithClient(cfg, mockClient)

	app := createTestApp()
	app.Post("/rebase", handler.HandleWebhook)

	req := httptest.NewRequest("POST", "/rebase", bytes.NewReader([]byte("{}")))
	req.Header.Set("Content-Type", "text/plain")

	resp, err := app.Test(req)
	assert.NoError(t, err)
	assert.Equal(t, 400, resp.StatusCode)

	body, _ := io.ReadAll(resp.Body)
	var response map[string]interface{}
	_ = json.Unmarshal(body, &response)

	assert.Contains(t, response["error"].(string), "Content-Type must be application/json")
}

func TestFivetranTerraformRebaseHandler_HandleWebhook_InvalidJSON(t *testing.T) {
	cfg := createTestConfig()
	mockClient := &MockRebaseGitLabClient{}
	handler := NewFivetranTerraformRebaseHandlerWithClient(cfg, mockClient)

	app := createTestApp()
	app.Post("/rebase", handler.HandleWebhook)

	req := httptest.NewRequest("POST", "/rebase", bytes.NewReader([]byte("invalid json")))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	assert.NoError(t, err)
	assert.Equal(t, 400, resp.StatusCode)

	body, _ := io.ReadAll(resp.Body)
	var response map[string]interface{}
	_ = json.Unmarshal(body, &response)

	assert.Contains(t, response["error"].(string), "Invalid JSON payload")
}

func TestFivetranTerraformRebaseHandler_HandleWebhook_UnsupportedEventType(t *testing.T) {
	cfg := createTestConfig()
	mockClient := &MockRebaseGitLabClient{}
	handler := NewFivetranTerraformRebaseHandlerWithClient(cfg, mockClient)

	app := createTestApp()
	app.Post("/rebase", handler.HandleWebhook)

	payload := map[string]interface{}{
		"object_kind": "push",
		"object_attributes": map[string]interface{}{
			"iid": 123,
		},
		"project": map[string]interface{}{
			"id": 456,
		},
	}

	payloadBytes, _ := json.Marshal(payload)
	req := httptest.NewRequest("POST", "/rebase", bytes.NewReader(payloadBytes))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	assert.NoError(t, err)
	assert.Equal(t, 400, resp.StatusCode)

	body, _ := io.ReadAll(resp.Body)
	var response map[string]interface{}
	_ = json.Unmarshal(body, &response)

	assert.Contains(t, response["error"].(string), "Unsupported event type")
}

func TestFivetranTerraformRebaseHandler_HandleWebhook_MissingObjectAttributes(t *testing.T) {
	cfg := createTestConfig()
	mockClient := &MockRebaseGitLabClient{}
	handler := NewFivetranTerraformRebaseHandlerWithClient(cfg, mockClient)

	app := createTestApp()
	app.Post("/rebase", handler.HandleWebhook)

	payload := map[string]interface{}{
		"object_kind": "merge_request",
	}

	payloadBytes, _ := json.Marshal(payload)
	req := httptest.NewRequest("POST", "/rebase", bytes.NewReader(payloadBytes))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	assert.NoError(t, err)
	assert.Equal(t, 400, resp.StatusCode)

	body, _ := io.ReadAll(resp.Body)
	var response map[string]interface{}
	_ = json.Unmarshal(body, &response)

	assert.Contains(t, response["error"].(string), "missing object_attributes")
}

func TestFivetranTerraformRebaseHandler_HandleWebhook_CommentsDisabled(t *testing.T) {
	cfg := &config.Config{
		GitLab: config.GitLabConfig{
			BaseURL: "https://gitlab.example.com",
			Token:   "test-token",
		},
		Comments: config.CommentsConfig{
			EnableMRComments: false, // Comments disabled
		},
	}

	mockClient := &MockRebaseGitLabClient{}
	handler := NewFivetranTerraformRebaseHandlerWithClient(cfg, mockClient)

	app := createTestApp()
	app.Post("/rebase", handler.HandleWebhook)

	payload := map[string]interface{}{
		"object_kind": "merge_request",
		"object_attributes": map[string]interface{}{
			"iid":   123,
			"state": "opened",
		},
		"project": map[string]interface{}{
			"id": 456,
		},
		"user": map[string]interface{}{
			"username": "testuser",
		},
	}

	payloadBytes, _ := json.Marshal(payload)
	req := httptest.NewRequest("POST", "/rebase", bytes.NewReader(payloadBytes))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	assert.NoError(t, err)
	assert.Equal(t, 200, resp.StatusCode)

	// Verify no comment was added
	assert.Len(t, mockClient.capturedComments, 0)

	// But rebase should still be called
	assert.Len(t, mockClient.capturedRebaseMRs, 1)
}

func TestFivetranTerraformRebaseHandler_ValidateWebhookPayload(t *testing.T) {
	cfg := createTestConfig()
	mockClient := &MockRebaseGitLabClient{}
	handler := NewFivetranTerraformRebaseHandlerWithClient(cfg, mockClient)

	tests := []struct {
		name        string
		payload     map[string]interface{}
		expectError bool
		errorMsg    string
	}{
		{
			name: "Valid payload",
			payload: map[string]interface{}{
				"object_attributes": map[string]interface{}{
					"iid": 123,
				},
				"project": map[string]interface{}{
					"id": 456,
				},
			},
			expectError: false,
		},
		{
			name:        "Nil payload",
			payload:     nil,
			expectError: true,
			errorMsg:    "payload is nil",
		},
		{
			name:        "Missing object_attributes",
			payload:     map[string]interface{}{},
			expectError: true,
			errorMsg:    "missing object_attributes",
		},
		{
			name: "Missing iid",
			payload: map[string]interface{}{
				"object_attributes": map[string]interface{}{},
				"project": map[string]interface{}{
					"id": 456,
				},
			},
			expectError: true,
			errorMsg:    "missing iid",
		},
		{
			name: "Missing project",
			payload: map[string]interface{}{
				"object_attributes": map[string]interface{}{
					"iid": 123,
				},
			},
			expectError: true,
			errorMsg:    "missing project",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := handler.validateWebhookPayload(tt.payload)
			if tt.expectError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMsg)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
