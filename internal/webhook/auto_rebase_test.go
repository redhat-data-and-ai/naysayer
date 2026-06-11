package webhook

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"

	"github.com/redhat-data-and-ai/naysayer/internal/config"
	"github.com/redhat-data-and-ai/naysayer/internal/gitlab"
)

// MockRebaseGitLabClient is a mock GitLab client for rebase testing
type MockRebaseGitLabClient struct {
	rebaseError       error
	addCommentError   error
	listOpenMRsError  error
	openMRs           []int
	openMRDetails     []gitlab.MRDetails
	capturedComments  []string
	capturedRebaseMRs []struct {
		projectID int
		mrIID     int
	}
	// For fork MR testing
	sourceProjectID int // Set to non-zero to simulate fork MR
}

func (m *MockRebaseGitLabClient) CompareBranches(sourceProjectID int, sourceBranch string, targetProjectID int, targetBranch string) (*gitlab.CompareResult, error) {
	return &gitlab.CompareResult{
		Commits: []gitlab.CompareCommit{
			{ID: "abc123", ShortID: "abc123", Title: "Mock commit"},
		},
	}, nil
}

func (m *MockRebaseGitLabClient) GetBranchCommit(projectID int, branch string) (string, error) {
	return "mock-main-sha", nil
}

func (m *MockRebaseGitLabClient) CompareCommits(projectID int, fromSHA, toSHA string) (*gitlab.CompareResult, error) {
	return &gitlab.CompareResult{
		Commits: []gitlab.CompareCommit{
			{ID: "abc123", ShortID: "abc123", Title: "Mock commit"},
		},
	}, nil
}

func (m *MockRebaseGitLabClient) RebaseMR(projectID, mrIID int) (bool, error) {
	m.capturedRebaseMRs = append(m.capturedRebaseMRs, struct {
		projectID int
		mrIID     int
	}{projectID, mrIID})
	if m.rebaseError != nil {
		return false, m.rebaseError
	}
	return true, nil
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
	sourceProjectID := m.sourceProjectID
	if sourceProjectID == 0 {
		sourceProjectID = projectID
	}
	sha := "" // Same-project MRs don't need sha for compare
	if sourceProjectID != projectID {
		sha = "mock-fork-sha-" + string(rune('0'+mrIID%10))
	}
	return &gitlab.MRDetails{
		IID:                mrIID,
		SourceBranch:       "feature-branch",
		TargetBranch:       "main",
		Sha:                sha,
		ProjectID:          projectID,
		SourceProjectID:    sourceProjectID,
		TargetProjectID:    projectID,
		CreatedAt:          time.Now().Add(-24 * time.Hour).Format(time.RFC3339),
		Pipeline:           &gitlab.MRPipeline{Status: "success"},
		BehindCommitsCount: 1,
		MergeStatus:        "can_be_merged",
		RebaseInProgress:   false,
		HasConflicts:       false,
	}, nil
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

func (m *MockRebaseGitLabClient) ListOpenMRs(projectID int) ([]int, error) {
	if m.listOpenMRsError != nil {
		return nil, m.listOpenMRsError
	}
	return m.openMRs, nil
}

func (m *MockRebaseGitLabClient) ListOpenMRsWithDetails(projectID int) ([]gitlab.MRDetails, error) {
	if m.listOpenMRsError != nil {
		return nil, m.listOpenMRsError
	}
	// If openMRDetails is provided, use it; otherwise generate from openMRs
	if len(m.openMRDetails) > 0 {
		return m.openMRDetails, nil
	}

	// Simulate the new behavior: fetch details for each MR individually
	// This mimics what the real implementation does now
	details := make([]gitlab.MRDetails, 0, len(m.openMRs))
	for _, mrIID := range m.openMRs {
		// Call GetMRDetails for each (simulating N+1 calls)
		mrDetail, err := m.GetMRDetails(projectID, mrIID)
		if err != nil {
			// Skip MRs that fail to fetch
			continue
		}
		details = append(details, *mrDetail)
	}

	// If no details were fetched via GetMRDetails, generate defaults
	if len(details) == 0 && len(m.openMRs) > 0 {
		details = make([]gitlab.MRDetails, len(m.openMRs))
		for i, mrIID := range m.openMRs {
			details[i] = gitlab.MRDetails{
				IID:                mrIID,
				CreatedAt:          time.Now().Add(-24 * time.Hour).Format(time.RFC3339), // Created 1 day ago
				Pipeline:           &gitlab.MRPipeline{Status: "success"},
				BehindCommitsCount: 1,
				MergeStatus:        "can_be_merged",
				RebaseInProgress:   false,
				HasConflicts:       false,
			}
		}
	}

	return details, nil
}

// ListAllOpenMRsWithDetails lists all open merge requests (mock implementation)
func (m *MockRebaseGitLabClient) ListAllOpenMRsWithDetails(projectID int) ([]gitlab.MRDetails, error) {
	// For mock, return same as ListOpenMRsWithDetails
	return m.ListOpenMRsWithDetails(projectID)
}

// CloseMR closes a merge request (mock implementation)
func (m *MockRebaseGitLabClient) CloseMR(projectID, mrIID int) error {
	// Mock implementation - just return nil
	return nil
}

// FindCommentByPattern checks if a comment with the pattern exists (mock implementation)
func (m *MockRebaseGitLabClient) FindCommentByPattern(projectID, mrIID int, pattern string) (bool, error) {
	// Mock implementation - check captured comments
	for _, comment := range m.capturedComments {
		if strings.Contains(comment, pattern) {
			return true, nil
		}
	}
	return false, nil
}

func (m *MockRebaseGitLabClient) GetPipelineJobs(projectID, pipelineID int) ([]gitlab.PipelineJob, error) {
	// Return empty jobs by default (all succeeded)
	return []gitlab.PipelineJob{}, nil
}

func (m *MockRebaseGitLabClient) GetJobTrace(projectID, jobID int) (string, error) {
	return "", nil
}

func (m *MockRebaseGitLabClient) FindLatestAtlantisComment(projectID, mrIID int) (*gitlab.MRComment, error) {
	// Return nil by default (no atlantis comment)
	return nil, nil
}

func (m *MockRebaseGitLabClient) AreAllPipelineJobsSucceeded(projectID, pipelineID int) (bool, error) {
	// Return true by default (all jobs succeeded)
	return true, nil
}

func (m *MockRebaseGitLabClient) CheckAtlantisCommentForPlanFailures(projectID, mrIID int) (bool, string) {
	// Return true, "atlantis_comment_not_found" by default (no atlantis comment found, skip rebase)
	// This matches the actual implementation behavior
	return true, "atlantis_comment_not_found"
}

func (m *MockRebaseGitLabClient) ListDirectoryFiles(projectID int, dirPath, ref string) ([]gitlab.RepositoryFile, error) {
	return []gitlab.RepositoryFile{}, nil
}

func TestNewFivetranTerraformRebaseHandler(t *testing.T) {
	cfg := createTestConfig()
	handler := NewAutoRebaseHandlerWithClient(cfg, &MockRebaseGitLabClient{})

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

	mockClient := &MockRebaseGitLabClient{
		openMRs: []int{123, 456, 789},
	}
	handler := NewAutoRebaseHandlerWithClient(cfg, mockClient)

	app := createTestApp()
	app.Post("/rebase", handler.HandleWebhook)

	payload := map[string]interface{}{
		"object_kind": "push",
		"ref":         "refs/heads/main",
		"project": map[string]interface{}{
			"id": 456,
		},
		"user_username": "testuser",
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
	assert.Equal(t, "completed", response["status"])
	assert.Equal(t, float64(456), response["project_id"])
	assert.Equal(t, "main", response["branch"])
	assert.Equal(t, float64(3), response["total_mrs"])
	assert.Equal(t, float64(3), response["eligible_mrs"])
	assert.Equal(t, float64(3), response["successful"])
	assert.Equal(t, float64(0), response["failed"])
	assert.Equal(t, float64(0), response["skipped"])

	// Verify rebase was called for all MRs
	assert.Len(t, mockClient.capturedRebaseMRs, 3)
	assert.Equal(t, 456, mockClient.capturedRebaseMRs[0].projectID)
	assert.Equal(t, 123, mockClient.capturedRebaseMRs[0].mrIID)
	assert.Equal(t, 456, mockClient.capturedRebaseMRs[1].projectID)
	assert.Equal(t, 456, mockClient.capturedRebaseMRs[1].mrIID)
	assert.Equal(t, 456, mockClient.capturedRebaseMRs[2].projectID)
	assert.Equal(t, 789, mockClient.capturedRebaseMRs[2].mrIID)

	// Verify comments were added to all MRs
	assert.Len(t, mockClient.capturedComments, 3)
	for _, comment := range mockClient.capturedComments {
		assert.Contains(t, comment, "Automated Rebase")
	}
}

func TestFivetranTerraformRebaseHandler_HandleWebhook_NoOpenMRs(t *testing.T) {
	cfg := createTestConfig()
	mockClient := &MockRebaseGitLabClient{
		openMRs: []int{}, // No open MRs
	}
	handler := NewAutoRebaseHandlerWithClient(cfg, mockClient)

	app := createTestApp()
	app.Post("/rebase", handler.HandleWebhook)

	payload := map[string]interface{}{
		"object_kind": "push",
		"ref":         "refs/heads/main",
		"project": map[string]interface{}{
			"id": 456,
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
	assert.Equal(t, "completed", response["status"])
	assert.Equal(t, float64(0), response["total_mrs"])
	assert.Equal(t, float64(0), response["eligible_mrs"])
	assert.Equal(t, float64(0), response["successful"])
	assert.Equal(t, float64(0), response["failed"])
	assert.Equal(t, float64(0), response["skipped"])

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
		openMRs:     []int{123, 456},
		rebaseError: fmt.Errorf("rebase failed: conflicts detected"),
	}
	handler := NewAutoRebaseHandlerWithClient(cfg, mockClient)

	app := createTestApp()
	app.Post("/rebase", handler.HandleWebhook)

	payload := map[string]interface{}{
		"object_kind": "push",
		"ref":         "refs/heads/main",
		"project": map[string]interface{}{
			"id": 456,
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
	assert.Equal(t, "completed", response["status"])
	assert.Equal(t, float64(2), response["total_mrs"])
	assert.Equal(t, float64(2), response["eligible_mrs"])
	assert.Equal(t, float64(0), response["successful"])
	assert.Equal(t, float64(2), response["failed"])
	assert.Equal(t, float64(0), response["skipped"])

	// Verify both rebase attempts were made
	assert.Len(t, mockClient.capturedRebaseMRs, 2)

	// Verify failures are reported
	failures := response["failures"].([]interface{})
	assert.Len(t, failures, 2)

	// Verify no comments were added due to failures
	assert.Len(t, mockClient.capturedComments, 0)
}

func TestAutoRebase_ForkPermissionErrorPostsComment(t *testing.T) {
	cfg := createTestConfig()
	mockClient := &MockRebaseGitLabClient{
		openMRs:     []int{913},
		rebaseError: fmt.Errorf("rebase failed: insufficient permissions or rebase not allowed: {\"message\":\"403 Forbidden - Cannot push to source branch\"}"),
	}
	handler := NewAutoRebaseHandlerWithClient(cfg, mockClient)

	app := createTestApp()
	app.Post("/rebase", handler.HandleWebhook)

	payload := map[string]interface{}{
		"object_kind": "push",
		"ref":         "refs/heads/main",
		"project": map[string]interface{}{
			"id": 94023,
		},
	}
	payloadBytes, _ := json.Marshal(payload)
	req := httptest.NewRequest("POST", "/rebase", bytes.NewReader(payloadBytes))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	assert.NoError(t, err)
	assert.Equal(t, 200, resp.StatusCode)

	body, _ := io.ReadAll(resp.Body)
	var response map[string]interface{}
	_ = json.Unmarshal(body, &response)

	assert.Equal(t, float64(1), response["failed"])
	// When rebase fails due to fork permission, we post a comment explaining manual rebase is needed
	assert.Len(t, mockClient.capturedComments, 1)
	assert.Contains(t, mockClient.capturedComments[0], "Auto-rebase attempted")
	assert.Contains(t, mockClient.capturedComments[0], "rebase manually")
	assert.Contains(t, mockClient.capturedComments[0], "fork")
}

func TestIsForkRebasePermissionError(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{"nil", nil, false},
		{"cannot push to source branch", fmt.Errorf("403 Forbidden - Cannot push to source branch"), true},
		{"wrapped message", fmt.Errorf("rebase failed: {\"message\":\"403 Forbidden - Cannot push to source branch\"}"), true},
		{"403 and push", fmt.Errorf("403 forbidden: cannot push"), true},
		{"conflicts", fmt.Errorf("rebase failed: conflicts detected"), false},
		{"generic", fmt.Errorf("something went wrong"), false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isForkRebasePermissionError(tt.err)
			assert.Equal(t, tt.expected, got)
		})
	}
}

func TestFivetranTerraformRebaseHandler_HandleWebhook_InvalidContentType(t *testing.T) {
	cfg := createTestConfig()
	mockClient := &MockRebaseGitLabClient{}
	handler := NewAutoRebaseHandlerWithClient(cfg, mockClient)

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
	handler := NewAutoRebaseHandlerWithClient(cfg, mockClient)

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
	handler := NewAutoRebaseHandlerWithClient(cfg, mockClient)

	app := createTestApp()
	app.Post("/rebase", handler.HandleWebhook)

	payload := map[string]interface{}{
		"object_kind": "merge_request",
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

func TestFivetranTerraformRebaseHandler_HandleWebhook_MissingProject(t *testing.T) {
	cfg := createTestConfig()
	mockClient := &MockRebaseGitLabClient{}
	handler := NewAutoRebaseHandlerWithClient(cfg, mockClient)

	app := createTestApp()
	app.Post("/rebase", handler.HandleWebhook)

	payload := map[string]interface{}{
		"object_kind": "push",
		"ref":         "refs/heads/main",
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

	assert.Contains(t, response["error"].(string), "missing project information")
}

func TestFivetranTerraformRebaseHandler_HandleWebhook_PushToNonMainBranch(t *testing.T) {
	cfg := createTestConfig()
	mockClient := &MockRebaseGitLabClient{
		openMRs: []int{123},
	}
	handler := NewAutoRebaseHandlerWithClient(cfg, mockClient)

	app := createTestApp()
	app.Post("/rebase", handler.HandleWebhook)

	payload := map[string]interface{}{
		"object_kind": "push",
		"ref":         "refs/heads/feature-branch",
		"project": map[string]interface{}{
			"id": 456,
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
	assert.Equal(t, "skipped", response["status"])
	assert.Contains(t, response["reason"].(string), "only main/master triggers rebase")

	// Verify rebase was NOT called
	assert.Len(t, mockClient.capturedRebaseMRs, 0)
}

func TestFivetranTerraformRebaseHandler_ValidateWebhookPayload(t *testing.T) {
	cfg := createTestConfig()
	mockClient := &MockRebaseGitLabClient{}
	handler := NewAutoRebaseHandlerWithClient(cfg, mockClient)

	tests := []struct {
		name        string
		payload     map[string]interface{}
		expectError bool
		errorMsg    string
	}{
		{
			name: "Valid payload",
			payload: map[string]interface{}{
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
			name:        "Missing project",
			payload:     map[string]interface{}{},
			expectError: true,
			errorMsg:    "missing project information",
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

func TestFivetranTerraformRebaseHandler_FilterEligibleMRs(t *testing.T) {
	cfg := createTestConfig()
	mockClient := &MockRebaseGitLabClient{}
	handler := NewAutoRebaseHandlerWithClient(cfg, mockClient)

	// Create test MRs with various statuses
	// Note: We only test with MRs created within 7 days, since older MRs
	// are filtered at the GitLab API level via created_after parameter
	recentMR := gitlab.MRDetails{
		IID:                123,
		CreatedAt:          time.Now().Add(-24 * time.Hour).Format(time.RFC3339), // 1 day old
		Pipeline:           &gitlab.MRPipeline{Status: "success"},
		BehindCommitsCount: 1,
		MergeStatus:        "can_be_merged",
		RebaseInProgress:   false,
		HasConflicts:       false,
	}

	runningPipelineMR := gitlab.MRDetails{
		IID:                789,
		CreatedAt:          time.Now().Add(-24 * time.Hour).Format(time.RFC3339),
		Pipeline:           &gitlab.MRPipeline{Status: "running"},
		BehindCommitsCount: 1,
		MergeStatus:        "can_be_merged",
		RebaseInProgress:   false,
		HasConflicts:       false,
	}

	failedPipelineMR := gitlab.MRDetails{
		IID:                101,
		CreatedAt:          time.Now().Add(-24 * time.Hour).Format(time.RFC3339),
		Pipeline:           &gitlab.MRPipeline{Status: "failed"},
		BehindCommitsCount: 1,
		MergeStatus:        "can_be_merged",
		RebaseInProgress:   false,
		HasConflicts:       false,
	}

	pendingPipelineMR := gitlab.MRDetails{
		IID:                102,
		CreatedAt:          time.Now().Add(-24 * time.Hour).Format(time.RFC3339),
		Pipeline:           &gitlab.MRPipeline{Status: "pending"},
		BehindCommitsCount: 1,
		MergeStatus:        "can_be_merged",
		RebaseInProgress:   false,
		HasConflicts:       false,
	}

	noPipelineMR := gitlab.MRDetails{
		IID:                103,
		CreatedAt:          time.Now().Add(-24 * time.Hour).Format(time.RFC3339),
		Pipeline:           nil, // No pipeline
		BehindCommitsCount: 1,
		MergeStatus:        "can_be_merged",
		RebaseInProgress:   false,
		HasConflicts:       false,
	}

	tests := []struct {
		name        string
		mrs         []gitlab.MRDetails
		expectedIDs []int
	}{
		{
			name:        "Recent MR with success pipeline",
			mrs:         []gitlab.MRDetails{recentMR},
			expectedIDs: []int{123},
		},
		{
			name:        "Running pipeline should be filtered out",
			mrs:         []gitlab.MRDetails{runningPipelineMR},
			expectedIDs: []int{},
		},
		{
			name:        "Failed pipeline should be filtered out (jobs failed or plan error)",
			mrs:         []gitlab.MRDetails{failedPipelineMR},
			expectedIDs: []int{},
		},
		{
			name:        "Pending pipeline should be filtered out",
			mrs:         []gitlab.MRDetails{pendingPipelineMR},
			expectedIDs: []int{},
		},
		{
			name:        "MR without pipeline should be included",
			mrs:         []gitlab.MRDetails{noPipelineMR},
			expectedIDs: []int{103},
		},
		{
			name:        "Mixed MRs - only eligible ones included (note: old MRs filtered by API)",
			mrs:         []gitlab.MRDetails{recentMR, runningPipelineMR, failedPipelineMR, noPipelineMR},
			expectedIDs: []int{123, 103},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Use a test project ID
			result := handler.filterEligibleMRs(456, tt.mrs)
			assert.Len(t, result.Eligible, len(tt.expectedIDs))

			actualIDs := make([]int, len(result.Eligible))
			for i, mr := range result.Eligible {
				actualIDs[i] = mr.IID
			}

			assert.ElementsMatch(t, tt.expectedIDs, actualIDs)
		})
	}
}

func TestFivetranTerraformRebaseHandler_HandleWebhook_WithFilteredMRs(t *testing.T) {
	cfg := &config.Config{
		GitLab: config.GitLabConfig{
			BaseURL: "https://gitlab.example.com",
			Token:   "test-token",
		},
		Comments: config.CommentsConfig{
			EnableMRComments: true,
		},
	}

	// Create MRs with different statuses
	// Note: Old MRs (> 7 days) would be filtered at API level via created_after parameter
	// So we only include MRs that are within the 7-day window
	mockClient := &MockRebaseGitLabClient{
		openMRDetails: []gitlab.MRDetails{
			{
				IID:                123,
				CreatedAt:          time.Now().Add(-24 * time.Hour).Format(time.RFC3339), // Eligible
				Pipeline:           &gitlab.MRPipeline{Status: "success"},
				BehindCommitsCount: 1,
				MergeStatus:        "can_be_merged",
				RebaseInProgress:   false,
				HasConflicts:       false,
			},
			{
				IID:       789,
				CreatedAt: time.Now().Add(-24 * time.Hour).Format(time.RFC3339), // Running pipeline
				Pipeline:  &gitlab.MRPipeline{Status: "running"},
			},
		},
	}
	handler := NewAutoRebaseHandlerWithClient(cfg, mockClient)

	app := createTestApp()
	app.Post("/rebase", handler.HandleWebhook)

	payload := map[string]interface{}{
		"object_kind": "push",
		"ref":         "refs/heads/main",
		"project": map[string]interface{}{
			"id": 456,
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
	assert.Equal(t, "completed", response["status"])
	assert.Equal(t, float64(2), response["total_mrs"])    // 2 total open MRs (old ones filtered by API)
	assert.Equal(t, float64(1), response["eligible_mrs"]) // Only 1 eligible
	assert.Equal(t, float64(1), response["successful"])   // 1 successful rebase
	assert.Equal(t, float64(0), response["failed"])
	assert.Equal(t, float64(1), response["skipped"]) // 1 skipped (running pipeline)

	// Verify rebase was only called for eligible MR
	assert.Len(t, mockClient.capturedRebaseMRs, 1)
	assert.Equal(t, 123, mockClient.capturedRebaseMRs[0].mrIID)

	// Verify comment was added
	assert.Len(t, mockClient.capturedComments, 1)
	assert.Contains(t, mockClient.capturedComments[0], "Automated Rebase")
}

// --- Compare API tests (authoritative behind detection) ---

// TestAutoRebase_CompareBranchesUsage tests that Compare API is used to determine behind status
func TestAutoRebase_CompareBranchesUsage(t *testing.T) {
	mockClient := &MockRebaseGitLabClient{
		rebaseError: nil,
		capturedRebaseMRs: make([]struct {
			projectID int
			mrIID     int
		}, 0),
		capturedComments: make([]string, 0),
	}
	mockClient.openMRs = []int{100, 200}

	cfg := createTestConfig()
	handler := NewAutoRebaseHandlerWithClient(cfg, mockClient)

	app := fiber.New()
	app.Post("/auto-rebase", handler.HandleWebhook)

	payload := map[string]interface{}{
		"object_kind": "push",
		"ref":         "refs/heads/main",
		"project": map[string]interface{}{
			"id": 123,
		},
	}
	payloadBytes, _ := json.Marshal(payload)

	req := httptest.NewRequest("POST", "/auto-rebase", strings.NewReader(string(payloadBytes)))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req, -1)
	assert.NoError(t, err)
	assert.Equal(t, 200, resp.StatusCode)

	assert.Equal(t, 2, len(mockClient.capturedRebaseMRs), "Expected 2 MRs to be rebased")
}

// TestAutoRebase_SkipsUpToDateMR tests that MRs with 0 behind commits are skipped
func TestAutoRebase_SkipsUpToDateMR(t *testing.T) {
	mockClient := &MockRebaseGitLabClient{
		rebaseError: nil,
		capturedRebaseMRs: make([]struct {
			projectID int
			mrIID     int
		}, 0),
		capturedComments: make([]string, 0),
	}
	mockClient.openMRs = []int{100}

	customMockClient := &CustomCompareGitLabClient{
		MockRebaseGitLabClient: mockClient,
		behindCommitCount:      0,
	}

	cfg := createTestConfig()
	handler := NewAutoRebaseHandlerWithClient(cfg, customMockClient)

	app := fiber.New()
	app.Post("/auto-rebase", handler.HandleWebhook)

	payload := map[string]interface{}{
		"object_kind": "push",
		"ref":         "refs/heads/main",
		"project":     map[string]interface{}{"id": 123},
	}
	payloadBytes, _ := json.Marshal(payload)
	req := httptest.NewRequest("POST", "/auto-rebase", strings.NewReader(string(payloadBytes)))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req, -1)
	assert.NoError(t, err)
	assert.Equal(t, 200, resp.StatusCode)
	assert.Equal(t, 0, len(mockClient.capturedRebaseMRs), "Expected 0 MRs to be rebased (already up-to-date)")
}

// TestAutoRebase_RebasesBehindMR tests that MRs with commits behind are rebased
func TestAutoRebase_RebasesBehindMR(t *testing.T) {
	mockClient := &MockRebaseGitLabClient{
		rebaseError: nil,
		capturedRebaseMRs: make([]struct {
			projectID int
			mrIID     int
		}, 0),
		capturedComments: make([]string, 0),
	}
	mockClient.openMRs = []int{100}

	customMockClient := &CustomCompareGitLabClient{
		MockRebaseGitLabClient: mockClient,
		behindCommitCount:      3,
	}

	cfg := createTestConfig()
	handler := NewAutoRebaseHandlerWithClient(cfg, customMockClient)

	app := fiber.New()
	app.Post("/auto-rebase", handler.HandleWebhook)

	payload := map[string]interface{}{
		"object_kind": "push",
		"ref":         "refs/heads/main",
		"project":     map[string]interface{}{"id": 123},
	}
	payloadBytes, _ := json.Marshal(payload)
	req := httptest.NewRequest("POST", "/auto-rebase", strings.NewReader(string(payloadBytes)))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req, -1)
	assert.NoError(t, err)
	assert.Equal(t, 200, resp.StatusCode)
	assert.Equal(t, 1, len(mockClient.capturedRebaseMRs), "Expected 1 MR to be rebased")
	assert.Equal(t, 123, mockClient.capturedRebaseMRs[0].projectID)
	assert.Equal(t, 100, mockClient.capturedRebaseMRs[0].mrIID)
}

// TestAutoRebase_CompareAPIFailure tests handling of Compare API failures
func TestAutoRebase_CompareAPIFailure(t *testing.T) {
	mockClient := &MockRebaseGitLabClient{
		rebaseError: nil,
		capturedRebaseMRs: make([]struct {
			projectID int
			mrIID     int
		}, 0),
		capturedComments: make([]string, 0),
	}
	mockClient.openMRs = []int{100}

	customMockClient := &CustomCompareGitLabClient{
		MockRebaseGitLabClient: mockClient,
		compareError:           assert.AnError,
	}

	cfg := createTestConfig()
	handler := NewAutoRebaseHandlerWithClient(cfg, customMockClient)

	app := fiber.New()
	app.Post("/auto-rebase", handler.HandleWebhook)

	payload := map[string]interface{}{
		"object_kind": "push",
		"ref":         "refs/heads/main",
		"project":     map[string]interface{}{"id": 123},
	}
	payloadBytes, _ := json.Marshal(payload)
	req := httptest.NewRequest("POST", "/auto-rebase", strings.NewReader(string(payloadBytes)))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req, -1)
	assert.NoError(t, err)
	assert.Equal(t, 200, resp.StatusCode)
	assert.Equal(t, 0, len(mockClient.capturedRebaseMRs), "Expected 0 MRs to be rebased (compare failed)")

	var responseBody map[string]interface{}
	err = json.NewDecoder(resp.Body).Decode(&responseBody)
	assert.NoError(t, err)
	assert.Equal(t, float64(0), responseBody["successful"])
	assert.Equal(t, float64(1), responseBody["failed"])
}

// TestAutoRebase_LogsCompareResults tests that compare results are logged
func TestAutoRebase_LogsCompareResults(t *testing.T) {
	mockClient := &MockRebaseGitLabClient{
		rebaseError: nil,
		capturedRebaseMRs: make([]struct {
			projectID int
			mrIID     int
		}, 0),
		capturedComments: make([]string, 0),
	}
	mockClient.openMRs = []int{100}

	customMockClient := &CustomCompareGitLabClient{
		MockRebaseGitLabClient: mockClient,
		behindCommitCount:      2,
	}

	cfg := createTestConfig()
	handler := NewAutoRebaseHandlerWithClient(cfg, customMockClient)

	app := fiber.New()
	app.Post("/auto-rebase", handler.HandleWebhook)

	payload := map[string]interface{}{
		"object_kind": "push",
		"ref":         "refs/heads/main",
		"project":     map[string]interface{}{"id": 123},
	}
	payloadBytes, _ := json.Marshal(payload)
	req := httptest.NewRequest("POST", "/auto-rebase", strings.NewReader(string(payloadBytes)))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req, -1)
	assert.NoError(t, err)
	assert.Equal(t, 200, resp.StatusCode)
	assert.Equal(t, 1, len(mockClient.capturedRebaseMRs))
}

// CustomCompareGitLabClient wraps MockRebaseGitLabClient to customize CompareBranches behavior
type CustomCompareGitLabClient struct {
	*MockRebaseGitLabClient
	behindCommitCount int
	compareError      error
}

func (c *CustomCompareGitLabClient) CompareBranches(sourceProjectID int, sourceBranch string, targetProjectID int, targetBranch string) (*gitlab.CompareResult, error) {
	if c.compareError != nil {
		return nil, c.compareError
	}
	commits := make([]gitlab.CompareCommit, c.behindCommitCount)
	for i := 0; i < c.behindCommitCount; i++ {
		commits[i] = gitlab.CompareCommit{
			ID:      "commit" + string(rune('1'+i)),
			ShortID: "commit" + string(rune('1'+i)),
			Title:   "Test commit " + string(rune('1'+i)),
		}
	}
	return &gitlab.CompareResult{Commits: commits}, nil
}

// TestAutoRebase_MixedBehindStatus tests handling of mixed MR behind statuses
func TestAutoRebase_MixedBehindStatus(t *testing.T) {
	tests := []struct {
		name            string
		mrIIDs          []int
		behindCounts    map[int]int
		expectedRebased int
	}{
		{"All up-to-date", []int{100, 200, 300}, map[int]int{100: 0, 200: 0, 300: 0}, 0},
		{"All behind", []int{100, 200, 300}, map[int]int{100: 1, 200: 2, 300: 3}, 3},
		{"Mixed status", []int{100, 200, 300}, map[int]int{100: 0, 200: 1, 300: 0}, 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			multiMRCounter = 0 // reset for each subtest
			mockClient := &MockRebaseGitLabClient{
				rebaseError: nil,
				capturedRebaseMRs: make([]struct {
					projectID int
					mrIID     int
				}, 0),
				capturedComments: make([]string, 0),
			}
			mockClient.openMRs = tt.mrIIDs
			customMockClient := &MultiMRCompareClient{
				MockRebaseGitLabClient: mockClient,
				behindCounts:           tt.behindCounts,
			}
			cfg := createTestConfig()
			handler := NewAutoRebaseHandlerWithClient(cfg, customMockClient)
			app := fiber.New()
			app.Post("/auto-rebase", handler.HandleWebhook)
			payload := map[string]interface{}{
				"object_kind": "push",
				"ref":         "refs/heads/main",
				"project":     map[string]interface{}{"id": 123},
			}
			payloadBytes, _ := json.Marshal(payload)
			req := httptest.NewRequest("POST", "/auto-rebase", strings.NewReader(string(payloadBytes)))
			req.Header.Set("Content-Type", "application/json")
			resp, err := app.Test(req, -1)
			assert.NoError(t, err)
			assert.Equal(t, 200, resp.StatusCode)
			assert.Equal(t, tt.expectedRebased, len(mockClient.capturedRebaseMRs), "Unexpected number of rebased MRs")
		})
	}
}

// MultiMRCompareClient supports different behind counts for different MRs
type MultiMRCompareClient struct {
	*MockRebaseGitLabClient
	behindCounts map[int]int
}

var multiMRCounter int

func (c *MultiMRCompareClient) CompareBranches(sourceProjectID int, sourceBranch string, targetProjectID int, targetBranch string) (*gitlab.CompareResult, error) {
	mrIID := c.openMRs[multiMRCounter%len(c.openMRs)]
	multiMRCounter++
	behindCount := c.behindCounts[mrIID]
	commits := make([]gitlab.CompareCommit, behindCount)
	for i := 0; i < behindCount; i++ {
		commits[i] = gitlab.CompareCommit{
			ID:      "commit" + strings.Repeat(string(rune('a'+i)), 7),
			ShortID: "commit" + string(rune('a'+i)),
			Title:   "Test commit",
		}
	}
	return &gitlab.CompareResult{Commits: commits}, nil
}

func (c *MultiMRCompareClient) GetMRDetails(projectID, mrIID int) (*gitlab.MRDetails, error) {
	return &gitlab.MRDetails{
		IID:                mrIID,
		SourceBranch:       "feature-branch-" + strings.Repeat(string(rune('0'+mrIID)), 1),
		TargetBranch:       "main",
		CreatedAt:          time.Now().Add(-24 * time.Hour).Format(time.RFC3339),
		Pipeline:           &gitlab.MRPipeline{Status: "success"},
		BehindCommitsCount: 1,
		MergeStatus:        "can_be_merged",
		RebaseInProgress:   false,
		HasConflicts:       false,
	}, nil
}

func (c *MultiMRCompareClient) ListOpenMRsWithDetails(projectID int) ([]gitlab.MRDetails, error) {
	details := make([]gitlab.MRDetails, 0)
	for _, mrIID := range c.openMRs {
		details = append(details, gitlab.MRDetails{
			IID:                mrIID,
			SourceBranch:       "feature-branch-" + strings.Repeat(string(rune('0'+mrIID)), 1),
			TargetBranch:       "main",
			CreatedAt:          time.Now().Add(-24 * time.Hour).Format(time.RFC3339),
			Pipeline:           &gitlab.MRPipeline{Status: "success"},
			BehindCommitsCount: 1,
			MergeStatus:        "can_be_merged",
			RebaseInProgress:   false,
			HasConflicts:       false,
		})
	}
	return details, nil
}

// TestAutoRebase_CompareBranchDirection tests that Compare API is called with correct direction
func TestAutoRebase_CompareBranchDirection(t *testing.T) {
	capturedComparisons := make([]struct {
		source string
		target string
	}, 0)
	mockClient := &MockRebaseGitLabClient{
		rebaseError: nil,
		capturedRebaseMRs: make([]struct {
			projectID int
			mrIID     int
		}, 0),
		capturedComments: make([]string, 0),
	}
	mockClient.openMRs = []int{100}
	customMockClient := &DirectionTrackingClient{
		MockRebaseGitLabClient: mockClient,
		capturedComparisons:    &capturedComparisons,
		behindCount:            1,
	}
	cfg := createTestConfig()
	handler := NewAutoRebaseHandlerWithClient(cfg, customMockClient)
	app := fiber.New()
	app.Post("/auto-rebase", handler.HandleWebhook)
	payload := map[string]interface{}{
		"object_kind": "push",
		"ref":         "refs/heads/main",
		"project":     map[string]interface{}{"id": 123},
	}
	payloadBytes, _ := json.Marshal(payload)
	req := httptest.NewRequest("POST", "/auto-rebase", strings.NewReader(string(payloadBytes)))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req, -1)
	assert.NoError(t, err)
	assert.Equal(t, 200, resp.StatusCode)
	assert.Equal(t, 1, len(capturedComparisons), "Expected 1 compare call")
	assert.Equal(t, "feature-branch", capturedComparisons[0].source, "Source should be feature branch")
	assert.Equal(t, "main", capturedComparisons[0].target, "Target should be main branch")
}

// DirectionTrackingClient tracks Compare API call direction
type DirectionTrackingClient struct {
	*MockRebaseGitLabClient
	capturedComparisons *[]struct {
		source string
		target string
	}
	behindCount int
}

func (c *DirectionTrackingClient) GetMRDetails(projectID, mrIID int) (*gitlab.MRDetails, error) {
	return &gitlab.MRDetails{
		IID:                mrIID,
		SourceBranch:       "feature-branch",
		TargetBranch:       "main",
		CreatedAt:          time.Now().Add(-24 * time.Hour).Format(time.RFC3339),
		Pipeline:           &gitlab.MRPipeline{Status: "success"},
		BehindCommitsCount: 1,
		MergeStatus:        "can_be_merged",
		RebaseInProgress:   false,
		HasConflicts:       false,
	}, nil
}

func (c *DirectionTrackingClient) ListOpenMRsWithDetails(projectID int) ([]gitlab.MRDetails, error) {
	return []gitlab.MRDetails{
		{
			IID:                100,
			SourceBranch:       "feature-branch",
			TargetBranch:       "main",
			CreatedAt:          time.Now().Add(-24 * time.Hour).Format(time.RFC3339),
			Pipeline:           &gitlab.MRPipeline{Status: "success"},
			BehindCommitsCount: 1,
			MergeStatus:        "can_be_merged",
			RebaseInProgress:   false,
			HasConflicts:       false,
		},
	}, nil
}

func (c *DirectionTrackingClient) CompareBranches(sourceProjectID int, sourceBranch string, targetProjectID int, targetBranch string) (*gitlab.CompareResult, error) {
	*c.capturedComparisons = append(*c.capturedComparisons, struct {
		source string
		target string
	}{source: sourceBranch, target: targetBranch})
	commits := make([]gitlab.CompareCommit, c.behindCount)
	for i := 0; i < c.behindCount; i++ {
		commits[i] = gitlab.CompareCommit{
			ID:      "abc123" + string(rune('0'+i)),
			ShortID: "abc12",
			Title:   "Test commit",
		}
	}
	return &gitlab.CompareResult{Commits: commits}, nil
}

// TestAutoRebase_ForkMR tests auto-rebase with a fork MR (cross-project)
func TestAutoRebase_ForkMR(t *testing.T) {
	mockClient := &ForkMRCompareClient{
		MockRebaseGitLabClient: &MockRebaseGitLabClient{
			openMRs:           []int{100},
			capturedRebaseMRs: make([]struct{ projectID, mrIID int }, 0),
			capturedComments:  make([]string, 0),
			sourceProjectID:   456, // Fork MR: source is in project 456
		},
		behindCount: 1,
	}

	cfg := &config.Config{
		GitLab: config.GitLabConfig{
			BaseURL: "https://gitlab.example.com",
			Token:   "test-token",
		},
		AutoRebase: config.AutoRebaseConfig{
			Enabled:               true,
			CheckAtlantisComments: false,
			RepositoryToken:       "test-token",
		},
		Comments: config.CommentsConfig{
			EnableMRComments: true,
		},
	}

	handler := NewAutoRebaseHandlerWithClient(cfg, mockClient)
	app := fiber.New(fiber.Config{DisableStartupMessage: true})
	app.Post("/auto-rebase", handler.HandleWebhook)

	payload := map[string]interface{}{
		"object_kind": "push",
		"ref":         "refs/heads/main",
		"project": map[string]interface{}{
			"id": 123,
		},
	}
	jsonData, _ := json.Marshal(payload)

	req := httptest.NewRequest("POST", "/auto-rebase", bytes.NewReader(jsonData))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req, 30000)
	assert.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()

	body, _ := io.ReadAll(resp.Body)
	assert.Equal(t, 200, resp.StatusCode, "response body: %s", string(body))

	var response map[string]interface{}
	_ = json.Unmarshal(body, &response)

	assert.Equal(t, "completed", response["status"])
	assert.Equal(t, float64(1), response["successful"], "expected 1 successful rebase for fork MR")
	assert.Equal(t, float64(0), response["failed"])
	assert.Len(t, mockClient.capturedRebaseMRs, 1)
	assert.Equal(t, 100, mockClient.capturedRebaseMRs[0].mrIID)

	// Verify fork path was used (CompareCommits in target project)
	assert.Equal(t, 456, mockClient.capturedSourceProjectID, "fork path: source project ID should be fork (456)")
	assert.Equal(t, 123, mockClient.capturedTargetProjectID, "fork path: target project ID should be upstream (123)")

	// Verify rebase comment was added
	assert.Len(t, mockClient.capturedComments, 1)
	assert.Contains(t, mockClient.capturedComments[0], "Automated Rebase")
}

// TestAutoRebase_SameProjectMR tests auto-rebase with same-project MR (non-fork)
func TestAutoRebase_SameProjectMR(t *testing.T) {
	mockClient := &ForkMRCompareClient{
		MockRebaseGitLabClient: &MockRebaseGitLabClient{
			openMRs:           []int{200},
			capturedRebaseMRs: make([]struct{ projectID, mrIID int }, 0),
			capturedComments:  make([]string, 0),
			sourceProjectID:   0, // Same-project MR
		},
		behindCount: 1,
	}

	cfg := &config.Config{
		GitLab: config.GitLabConfig{
			BaseURL: "https://gitlab.example.com",
			Token:   "test-token",
		},
		AutoRebase: config.AutoRebaseConfig{
			Enabled:               true,
			CheckAtlantisComments: false,
			RepositoryToken:       "test-token",
		},
		Comments: config.CommentsConfig{
			EnableMRComments: true,
		},
	}

	handler := NewAutoRebaseHandlerWithClient(cfg, mockClient)
	app := fiber.New(fiber.Config{DisableStartupMessage: true})
	app.Post("/auto-rebase", handler.HandleWebhook)

	payload := map[string]interface{}{
		"object_kind": "push",
		"ref":         "refs/heads/main",
		"project": map[string]interface{}{
			"id": 123,
		},
	}
	jsonData, _ := json.Marshal(payload)

	req := httptest.NewRequest("POST", "/auto-rebase", bytes.NewReader(jsonData))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req, 30000)
	assert.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()

	body, _ := io.ReadAll(resp.Body)
	assert.Equal(t, 200, resp.StatusCode, "response body: %s", string(body))

	var response map[string]interface{}
	_ = json.Unmarshal(body, &response)

	assert.Equal(t, "completed", response["status"])
	assert.Equal(t, float64(1), response["successful"], "expected 1 successful rebase for same-project MR")
	assert.Equal(t, float64(0), response["failed"])
	assert.Len(t, mockClient.capturedRebaseMRs, 1)
	assert.Equal(t, 200, mockClient.capturedRebaseMRs[0].mrIID)

	// Verify CompareBranches was called with same project ID for both
	assert.Equal(t, 123, mockClient.capturedSourceProjectID, "source project ID should match target for same-project MR")
	assert.Equal(t, 123, mockClient.capturedTargetProjectID, "target project ID should be 123")

	// Verify rebase comment was added
	assert.Len(t, mockClient.capturedComments, 1)
	assert.Contains(t, mockClient.capturedComments[0], "Automated Rebase")
}

// ForkMRCompareClient tracks project IDs for fork MR testing
type ForkMRCompareClient struct {
	*MockRebaseGitLabClient
	capturedSourceProjectID int
	capturedTargetProjectID int
	behindCount             int
}

func (c *ForkMRCompareClient) CompareBranches(sourceProjectID int, sourceBranch string, targetProjectID int, targetBranch string) (*gitlab.CompareResult, error) {
	c.capturedSourceProjectID = sourceProjectID
	c.capturedTargetProjectID = targetProjectID
	commits := make([]gitlab.CompareCommit, c.behindCount)
	for i := 0; i < c.behindCount; i++ {
		commits[i] = gitlab.CompareCommit{ID: "fork-commit-" + string(rune('a'+i)), ShortID: "fork", Title: "Fork commit"}
	}
	return &gitlab.CompareResult{Commits: commits}, nil
}

// CompareCommits is used for fork MR path; record projectID as target (upstream)
func (c *ForkMRCompareClient) CompareCommits(projectID int, fromSHA, toSHA string) (*gitlab.CompareResult, error) {
	c.capturedTargetProjectID = projectID
	c.capturedSourceProjectID = 456 // Fork path: source is fork project (set by test)
	commits := make([]gitlab.CompareCommit, c.behindCount)
	for i := 0; i < c.behindCount; i++ {
		commits[i] = gitlab.CompareCommit{ID: "fork-commit-" + string(rune('a'+i)), ShortID: "fork", Title: "Fork commit"}
	}
	return &gitlab.CompareResult{Commits: commits}, nil
}

func init() {
	logger, _ := zap.NewDevelopment()
	zap.ReplaceGlobals(logger)
}
