package webhook

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"

	"github.com/redhat-data-and-ai/naysayer/internal/config"
	"github.com/redhat-data-and-ai/naysayer/internal/gitlab"
)

// MockStaleMRClient is a mock GitLab client for stale MR cleanup testing
type MockStaleMRClient struct {
	openMRs              []gitlab.MRDetails
	closedMRs            []int
	addedComments        []string
	commentPatternChecks map[int]bool // mrIID -> hasPattern
	listMRsError         error
	closeMRError         error
	addCommentError      error
	findPatternError     error
}

func (m *MockStaleMRClient) ListAllOpenMRsWithDetails(projectID int) ([]gitlab.MRDetails, error) {
	if m.listMRsError != nil {
		return nil, m.listMRsError
	}
	return m.openMRs, nil
}

func (m *MockStaleMRClient) CloseMR(projectID, mrIID int) error {
	if m.closeMRError != nil {
		return m.closeMRError
	}
	m.closedMRs = append(m.closedMRs, mrIID)
	return nil
}

func (m *MockStaleMRClient) AddMRComment(projectID, mrIID int, comment string) error {
	if m.addCommentError != nil {
		return m.addCommentError
	}
	m.addedComments = append(m.addedComments, comment)
	return nil
}

func (m *MockStaleMRClient) FindCommentByPattern(projectID, mrIID int, pattern string) (bool, error) {
	if m.findPatternError != nil {
		return false, m.findPatternError
	}
	if m.commentPatternChecks == nil {
		return false, nil
	}
	return m.commentPatternChecks[mrIID], nil
}

// Stub methods to satisfy GitLabClient interface
func (m *MockStaleMRClient) FetchFileContent(projectID int, filePath, ref string) (*gitlab.FileContent, error) {
	return nil, nil
}
func (m *MockStaleMRClient) GetMRTargetBranch(projectID, mrIID int) (string, error) { return "", nil }
func (m *MockStaleMRClient) GetMRDetails(projectID, mrIID int) (*gitlab.MRDetails, error) {
	return nil, nil
}
func (m *MockStaleMRClient) FetchMRChanges(projectID, mrIID int) ([]gitlab.FileChange, error) {
	return nil, nil
}
func (m *MockStaleMRClient) AddOrUpdateMRComment(projectID, mrIID int, commentBody, commentType string) error {
	return nil
}
func (m *MockStaleMRClient) ListMRComments(projectID, mrIID int) ([]gitlab.MRComment, error) {
	return nil, nil
}
func (m *MockStaleMRClient) UpdateMRComment(projectID, mrIID, commentID int, newBody string) error {
	return nil
}
func (m *MockStaleMRClient) FindLatestNaysayerComment(projectID, mrIID int, commentType ...string) (*gitlab.MRComment, error) {
	return nil, nil
}
func (m *MockStaleMRClient) ApproveMR(projectID, mrIID int) error                        { return nil }
func (m *MockStaleMRClient) ApproveMRWithMessage(projectID, mrIID int, message string) error {
	return nil
}
func (m *MockStaleMRClient) ResetNaysayerApproval(projectID, mrIID int) error { return nil }
func (m *MockStaleMRClient) GetCurrentBotUsername() (string, error)           { return "naysayer-bot", nil }
func (m *MockStaleMRClient) IsNaysayerBotAuthor(author map[string]interface{}) bool {
	return false
}
func (m *MockStaleMRClient) RebaseMR(projectID, mrIID int) error { return nil }
func (m *MockStaleMRClient) ListOpenMRs(projectID int) ([]int, error) {
	return nil, nil
}
func (m *MockStaleMRClient) ListOpenMRsWithDetails(projectID int) ([]gitlab.MRDetails, error) {
	return nil, nil
}

func (m *MockStaleMRClient) GetPipelineJobs(projectID, pipelineID int) ([]gitlab.PipelineJob, error) {
	return []gitlab.PipelineJob{}, nil
}

func (m *MockStaleMRClient) GetJobTrace(projectID, jobID int) (string, error) {
	return "", nil
}

func (m *MockStaleMRClient) FindLatestAtlantisComment(projectID, mrIID int) (*gitlab.MRComment, error) {
	return nil, nil
}

func (m *MockStaleMRClient) AreAllPipelineJobsSucceeded(projectID, pipelineID int) (bool, error) {
	return true, nil
}

func (m *MockStaleMRClient) CheckAtlantisCommentForPlanFailures(projectID, mrIID int) (bool, string) {
	return false, ""
}

func createStaleMRTestConfig() *config.Config {
	return &config.Config{
		GitLab: config.GitLabConfig{
			BaseURL: "https://gitlab.example.com",
			Token:   "test-token",
		},
		StaleMR: config.StaleMRConfig{
			ClosureDays: 30,
		},
	}
}

func TestNewStaleMRCleanupHandler(t *testing.T) {
	cfg := createStaleMRTestConfig()
	handler := NewStaleMRCleanupHandler(cfg)

	assert.NotNil(t, handler)
	assert.Equal(t, cfg, handler.config)
}

func TestStaleMRCleanupHandler_HandleWebhook_Success(t *testing.T) {
	cfg := createStaleMRTestConfig()

	// Create MRs with different ages
	now := time.Now()
	mockClient := &MockStaleMRClient{
		openMRs: []gitlab.MRDetails{
			{IID: 1, UpdatedAt: now.AddDate(0, 0, -35).Format(time.RFC3339)}, // 35 days old - should close
			{IID: 2, UpdatedAt: now.AddDate(0, 0, -28).Format(time.RFC3339)}, // 28 days old - skip (under 30)
			{IID: 3, UpdatedAt: now.AddDate(0, 0, -10).Format(time.RFC3339)}, // 10 days old - skip
		},
		commentPatternChecks: map[int]bool{
			1: false,
			2: false,
			3: false,
		},
	}

	handler := NewStaleMRCleanupHandlerWithClient(cfg, mockClient)

	app := fiber.New()
	app.Post("/stale-mr-cleanup", handler.HandleWebhook)

	payload := map[string]interface{}{
		"project_id":   123,
		"closure_days": 30,
		"dry_run":      false,
	}
	payloadBytes, _ := json.Marshal(payload)

	req := httptest.NewRequest("POST", "/stale-mr-cleanup", bytes.NewBuffer(payloadBytes))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	assert.NoError(t, err)
	assert.Equal(t, 200, resp.StatusCode)

	var response StaleMRCleanupResponse
	err = json.NewDecoder(resp.Body).Decode(&response)
	assert.NoError(t, err)

	assert.Equal(t, "processed", response.WebhookResponse)
	assert.Equal(t, "completed", response.Status)
	assert.Equal(t, 123, response.ProjectID)
	assert.Equal(t, 3, response.TotalMRs)
	assert.Equal(t, 1, response.Closed) // MR !1
	assert.Equal(t, 0, response.Failed)

	// Verify MR was closed
	assert.Contains(t, mockClient.closedMRs, 1)
	assert.NotContains(t, mockClient.closedMRs, 2)
	assert.NotContains(t, mockClient.closedMRs, 3)

	// Verify only one closure comment was added
	assert.Equal(t, 1, len(mockClient.addedComments))
}

func TestStaleMRCleanupHandler_HandleWebhook_DryRun(t *testing.T) {
	cfg := createStaleMRTestConfig()

	now := time.Now()
	mockClient := &MockStaleMRClient{
		openMRs: []gitlab.MRDetails{
			{IID: 1, UpdatedAt: now.AddDate(0, 0, -35).Format(time.RFC3339)}, // Should close (>= 30 days)
			{IID: 2, UpdatedAt: now.AddDate(0, 0, -28).Format(time.RFC3339)}, // Skip (< 30 days)
		},
		commentPatternChecks: map[int]bool{1: false, 2: false},
	}

	handler := NewStaleMRCleanupHandlerWithClient(cfg, mockClient)

	app := fiber.New()
	app.Post("/stale-mr-cleanup", handler.HandleWebhook)

	payload := map[string]interface{}{
		"project_id": 123,
		"dry_run":    true,
	}
	payloadBytes, _ := json.Marshal(payload)

	req := httptest.NewRequest("POST", "/stale-mr-cleanup", bytes.NewBuffer(payloadBytes))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	assert.NoError(t, err)
	assert.Equal(t, 200, resp.StatusCode)

	var response StaleMRCleanupResponse
	err = json.NewDecoder(resp.Body).Decode(&response)
	assert.NoError(t, err)

	assert.Equal(t, true, response.DryRun)
	assert.Equal(t, 1, response.Closed)

	// Verify nothing was actually changed
	assert.Equal(t, 0, len(mockClient.closedMRs))
	assert.Equal(t, 0, len(mockClient.addedComments))
}

func TestStaleMRCleanupHandler_HandleWebhook_InvalidContentType(t *testing.T) {
	cfg := createStaleMRTestConfig()
	handler := NewStaleMRCleanupHandler(cfg)

	app := fiber.New()
	app.Post("/stale-mr-cleanup", handler.HandleWebhook)

	req := httptest.NewRequest("POST", "/stale-mr-cleanup", bytes.NewBufferString("{}"))
	req.Header.Set("Content-Type", "text/plain")

	resp, err := app.Test(req)
	assert.NoError(t, err)
	assert.Equal(t, 400, resp.StatusCode)
}

func TestStaleMRCleanupHandler_HandleWebhook_InvalidJSON(t *testing.T) {
	cfg := createStaleMRTestConfig()
	handler := NewStaleMRCleanupHandler(cfg)

	app := fiber.New()
	app.Post("/stale-mr-cleanup", handler.HandleWebhook)

	req := httptest.NewRequest("POST", "/stale-mr-cleanup", bytes.NewBufferString("invalid json"))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	assert.NoError(t, err)
	assert.Equal(t, 400, resp.StatusCode)
}

func TestStaleMRCleanupHandler_HandleWebhook_MissingProjectID(t *testing.T) {
	cfg := createStaleMRTestConfig()
	handler := NewStaleMRCleanupHandler(cfg)

	app := fiber.New()
	app.Post("/stale-mr-cleanup", handler.HandleWebhook)

	payload := map[string]interface{}{
		"dry_run": false,
	}
	payloadBytes, _ := json.Marshal(payload)

	req := httptest.NewRequest("POST", "/stale-mr-cleanup", bytes.NewBuffer(payloadBytes))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	assert.NoError(t, err)
	assert.Equal(t, 400, resp.StatusCode)
}

func TestStaleMRCleanupHandler_ValidatePayload(t *testing.T) {
	cfg := createStaleMRTestConfig()
	handler := NewStaleMRCleanupHandler(cfg)

	tests := []struct {
		name    string
		payload StaleMRCleanupPayload
		wantErr bool
		errMsg  string
	}{
		{
			name:    "valid payload",
			payload: StaleMRCleanupPayload{ProjectID: 123, ClosureDays: 30},
			wantErr: false,
		},
		{
			name:    "missing project_id",
			payload: StaleMRCleanupPayload{ClosureDays: 30},
			wantErr: true,
			errMsg:  "project_id is required",
		},
		{
			name:    "negative closure_days",
			payload: StaleMRCleanupPayload{ProjectID: 123, ClosureDays: -1},
			wantErr: true,
			errMsg:  "closure_days must be >= 0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := handler.validatePayload(&tt.payload)
			if tt.wantErr {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestStaleMRCleanupHandler_DefaultThresholds(t *testing.T) {
	cfg := createStaleMRTestConfig()
	mockClient := &MockStaleMRClient{
		openMRs: []gitlab.MRDetails{},
	}

	handler := NewStaleMRCleanupHandlerWithClient(cfg, mockClient)

	app := fiber.New()
	app.Post("/stale-mr-cleanup", handler.HandleWebhook)

	// Don't specify closure_days
	payload := map[string]interface{}{
		"project_id": 123,
	}
	payloadBytes, _ := json.Marshal(payload)

	req := httptest.NewRequest("POST", "/stale-mr-cleanup", bytes.NewBuffer(payloadBytes))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	assert.NoError(t, err)

	var response StaleMRCleanupResponse
	err = json.NewDecoder(resp.Body).Decode(&response)
	assert.NoError(t, err)

	// Should use defaults from config
	assert.Equal(t, 30, response.ClosureDays)
}

func TestStaleMRCleanupHandler_GitLabAPIError(t *testing.T) {
	cfg := createStaleMRTestConfig()
	mockClient := &MockStaleMRClient{
		listMRsError: fmt.Errorf("GitLab API error"),
	}

	handler := NewStaleMRCleanupHandlerWithClient(cfg, mockClient)

	app := fiber.New()
	app.Post("/stale-mr-cleanup", handler.HandleWebhook)

	payload := map[string]interface{}{
		"project_id": 123,
	}
	payloadBytes, _ := json.Marshal(payload)

	req := httptest.NewRequest("POST", "/stale-mr-cleanup", bytes.NewBuffer(payloadBytes))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	assert.NoError(t, err)
	assert.Equal(t, 500, resp.StatusCode)
}

func TestStaleMRCleanupHandler_CommentTemplates(t *testing.T) {
	cfg := createStaleMRTestConfig()

	now := time.Now()
	mockClient := &MockStaleMRClient{
		openMRs: []gitlab.MRDetails{
			{IID: 1, UpdatedAt: now.AddDate(0, 0, -35).Format(time.RFC3339)}, // Close (35 days old)
			{IID: 2, UpdatedAt: now.AddDate(0, 0, -28).Format(time.RFC3339)}, // Skip (28 days old, under threshold)
		},
		commentPatternChecks: map[int]bool{1: false, 2: false},
	}

	handler := NewStaleMRCleanupHandlerWithClient(cfg, mockClient)

	app := fiber.New()
	app.Post("/stale-mr-cleanup", handler.HandleWebhook)

	payload := map[string]interface{}{
		"project_id": 123,
	}
	payloadBytes, _ := json.Marshal(payload)

	req := httptest.NewRequest("POST", "/stale-mr-cleanup", bytes.NewBuffer(payloadBytes))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	assert.NoError(t, err)
	assert.Equal(t, 200, resp.StatusCode)

	// Verify comment templates contain expected text
	assert.Equal(t, 1, len(mockClient.addedComments))

	// Check closure comment
	closureComment := mockClient.addedComments[0]
	assert.Contains(t, closureComment, "Automated Closure")
	assert.Contains(t, closureComment, "35 days") // Actual days since update
}

func TestStaleMRCleanupHandler_CustomThresholds(t *testing.T) {
	cfg := createStaleMRTestConfig()

	now := time.Now()
	mockClient := &MockStaleMRClient{
		openMRs: []gitlab.MRDetails{
			{IID: 1, UpdatedAt: now.AddDate(0, 0, -45).Format(time.RFC3339)}, // Should close with custom threshold (45 days)
			{IID: 2, UpdatedAt: now.AddDate(0, 0, -38).Format(time.RFC3339)}, // Skip (38 days, under 40 threshold)
		},
		commentPatternChecks: map[int]bool{1: false, 2: false},
	}

	handler := NewStaleMRCleanupHandlerWithClient(cfg, mockClient)

	app := fiber.New()
	app.Post("/stale-mr-cleanup", handler.HandleWebhook)

	payload := map[string]interface{}{
		"project_id":   123,
		"closure_days": 40,
	}
	payloadBytes, _ := json.Marshal(payload)

	req := httptest.NewRequest("POST", "/stale-mr-cleanup", bytes.NewBuffer(payloadBytes))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	assert.NoError(t, err)

	var response StaleMRCleanupResponse
	err = json.NewDecoder(resp.Body).Decode(&response)
	assert.NoError(t, err)

	assert.Equal(t, 40, response.ClosureDays)
	assert.Equal(t, 1, response.Closed)
	assert.Equal(t, 0, response.Failed)
}
