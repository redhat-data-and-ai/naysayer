package webhook

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/redhat-data-and-ai/naysayer/internal/config"
	"github.com/redhat-data-and-ai/naysayer/internal/gitlab"
	"github.com/redhat-data-and-ai/naysayer/internal/rules/shared"
	"github.com/stretchr/testify/assert"
)

// MockRuleManagerForApproval creates a rule manager that returns approval decisions
type MockRuleManagerForApproval struct {
	evaluateFunc func(*shared.MRContext) *shared.RuleEvaluation
}

func (m *MockRuleManagerForApproval) AddRule(rule shared.Rule) {
	// Not needed for mock
}

func (m *MockRuleManagerForApproval) EvaluateAll(ctx *shared.MRContext) *shared.RuleEvaluation {
	if m.evaluateFunc != nil {
		return m.evaluateFunc(ctx)
	}
	// Default to approval
	return &shared.RuleEvaluation{
		FinalDecision: shared.Decision{
			Type:    shared.Approve,
			Reason:  "Mock approval for testing",
			Summary: "✅ Test approval",
			Details: "Mock rule evaluation for testing approval workflow",
		},
		RuleResults: []shared.RuleResult{
			{
				RuleName: "warehouse_rule",
				Decision: shared.Decision{
					Type:   shared.Approve,
					Reason: "Mock warehouse approval",
				},
				Metadata: map[string]any{
					"analyzed_files":    []string{"dataproducts/agg/test/prod/product.yaml"},
					"warehouse_changes": []interface{}{"LARGE -> MEDIUM"},
				},
			},
		},
		ExecutionTime: time.Millisecond * 100,
	}
}

func TestHandleApprovalWithComments_Success(t *testing.T) {
	// Create test GitLab server that expects both comment and approval calls
	var commentReceived, approvalReceived bool
	gitlabServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/notes") {
			// Comment API call
			commentReceived = true
			assert.Equal(t, "POST", r.Method)
			assert.Contains(t, r.URL.Path, "/api/v4/projects/123/merge_requests/456/notes")

			w.WriteHeader(201)
			_, _ = w.Write([]byte(`{"id": 789, "body": "comment added"}`))
		} else if strings.Contains(r.URL.Path, "/approve") {
			// Approval API call
			approvalReceived = true
			assert.Equal(t, "POST", r.Method)
			assert.Contains(t, r.URL.Path, "/api/v4/projects/123/merge_requests/456/approve")

			w.WriteHeader(201)
			_, _ = w.Write([]byte(`{"id": 456, "approved": true}`))
		} else {
			w.WriteHeader(404)
		}
	}))
	defer gitlabServer.Close()

	cfg := &config.Config{
		GitLab: config.GitLabConfig{
			BaseURL: gitlabServer.URL,
			Token:   "test-token",
		},
		Comments: config.CommentsConfig{
			EnableMRComments: true,
			CommentVerbosity: "detailed",
		},
	}

	// Create webhook handler with mock rule manager that approves
	handler := &DataProductConfigMrReviewHandler{
		gitlabClient: gitlab.NewClientWithConfig(cfg),
		ruleManager: &MockRuleManagerForApproval{
			evaluateFunc: func(ctx *shared.MRContext) *shared.RuleEvaluation {
				return &shared.RuleEvaluation{
					FinalDecision: shared.Decision{
						Type:   shared.Approve,
						Reason: "Warehouse decreases detected",
					},
					RuleResults: []shared.RuleResult{
						{
							RuleName: "warehouse_rule",
							Decision: shared.Decision{
								Type:   shared.Approve,
								Reason: "Safe warehouse changes",
							},
							Metadata: map[string]any{
								"analyzed_files":    []string{"test/product.yaml"},
								"warehouse_changes": []interface{}{"LARGE->MEDIUM"},
							},
						},
					},
					ExecutionTime: time.Millisecond * 150,
				}
			},
		},
		config: cfg,
	}

	result := &shared.RuleEvaluation{
		FinalDecision: shared.Decision{
			Type:   shared.Approve,
			Reason: "Warehouse decreases detected",
		},
		RuleResults: []shared.RuleResult{
			{
				RuleName: "warehouse_rule",
				Decision: shared.Decision{
					Type:   shared.Approve,
					Reason: "Safe warehouse changes",
				},
				Metadata: map[string]any{
					"analyzed_files":    []string{"test/product.yaml"},
					"warehouse_changes": []interface{}{"LARGE->MEDIUM"},
				},
			},
		},
		ExecutionTime: time.Millisecond * 150,
	}

	mrInfo := &gitlab.MRInfo{
		ProjectID: 123,
		MRIID:     456,
		Author:    "testuser",
		Title:     "Test warehouse decrease",
	}

	err := handler.handleApprovalWithComments(result, mrInfo)

	assert.NoError(t, err)
	assert.True(t, commentReceived, "Should have posted comment to GitLab")
	assert.True(t, approvalReceived, "Should have approved MR in GitLab")
}

func TestHandleApprovalWithComments_CommentsDisabled(t *testing.T) {
	// Create test GitLab server that should only receive approval call (no comment)
	var commentReceived, approvalReceived bool
	gitlabServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/notes") {
			commentReceived = true
			w.WriteHeader(201)
		} else if strings.Contains(r.URL.Path, "/approve") {
			approvalReceived = true
			w.WriteHeader(201)
			_, _ = w.Write([]byte(`{"approved": true}`))
		}
	}))
	defer gitlabServer.Close()

	cfg := &config.Config{
		GitLab: config.GitLabConfig{
			BaseURL: gitlabServer.URL,
			Token:   "test-token",
		},
		Comments: config.CommentsConfig{
			EnableMRComments: false, // Comments disabled
		},
	}

	handler := &DataProductConfigMrReviewHandler{
		gitlabClient: gitlab.NewClientWithConfig(cfg),
		config:       cfg,
	}

	result := &shared.RuleEvaluation{
		FinalDecision: shared.Decision{
			Type:   shared.Approve,
			Reason: "Test approval",
		},
		RuleResults:   []shared.RuleResult{},
		ExecutionTime: time.Millisecond * 100,
	}

	mrInfo := &gitlab.MRInfo{
		ProjectID: 123,
		MRIID:     456,
		Author:    "testuser",
		Title:     "Test MR",
	}

	err := handler.handleApprovalWithComments(result, mrInfo)

	assert.NoError(t, err)
	assert.False(t, commentReceived, "Should not have posted comment when disabled")
	assert.True(t, approvalReceived, "Should still have approved MR")
}

func TestHandleApprovalWithComments_CommentFailsContinues(t *testing.T) {
	// Create test GitLab server that fails comment but succeeds approval
	var approvalReceived bool
	gitlabServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/notes") {
			// Comment fails
			w.WriteHeader(401)
			_, _ = w.Write([]byte(`{"message": "Unauthorized"}`))
		} else if strings.Contains(r.URL.Path, "/approve") {
			// Approval succeeds
			approvalReceived = true
			w.WriteHeader(201)
			_, _ = w.Write([]byte(`{"approved": true}`))
		}
	}))
	defer gitlabServer.Close()

	cfg := &config.Config{
		GitLab: config.GitLabConfig{
			BaseURL: gitlabServer.URL,
			Token:   "test-token",
		},
		Comments: config.CommentsConfig{
			EnableMRComments: true,
		},
	}

	handler := &DataProductConfigMrReviewHandler{
		gitlabClient: gitlab.NewClientWithConfig(cfg),
		config:       cfg,
	}

	result := &shared.RuleEvaluation{
		FinalDecision: shared.Decision{
			Type:   shared.Approve,
			Reason: "Test approval",
		},
		RuleResults:   []shared.RuleResult{},
		ExecutionTime: time.Millisecond * 100,
	}

	mrInfo := &gitlab.MRInfo{
		ProjectID: 123,
		MRIID:     456,
		Author:    "testuser",
		Title:     "Test MR",
	}

	err := handler.handleApprovalWithComments(result, mrInfo)

	// Should succeed even if comment fails
	assert.NoError(t, err)
	assert.True(t, approvalReceived, "Should have approved MR despite comment failure")
}

func TestHandleApprovalWithComments_ApprovalFallback(t *testing.T) {
	// Create test GitLab server that fails approval with message but succeeds simple approval
	callCount := 0
	gitlabServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/approve") {
			callCount++
			if callCount == 1 {
				// First approval call (with message) fails
				w.WriteHeader(400)
				_, _ = w.Write([]byte(`{"message": "Bad Request"}`))
			} else {
				// Second approval call (simple) succeeds
				w.WriteHeader(201)
				_, _ = w.Write([]byte(`{"approved": true}`))
			}
		}
	}))
	defer gitlabServer.Close()

	cfg := &config.Config{
		GitLab: config.GitLabConfig{
			BaseURL: gitlabServer.URL,
			Token:   "test-token",
		},
		Comments: config.CommentsConfig{
			EnableMRComments: false, // Skip comment to focus on approval
		},
	}

	handler := &DataProductConfigMrReviewHandler{
		gitlabClient: gitlab.NewClientWithConfig(cfg),
		config:       cfg,
	}

	result := &shared.RuleEvaluation{
		FinalDecision: shared.Decision{
			Type:   shared.Approve,
			Reason: "Test approval",
		},
		RuleResults:   []shared.RuleResult{},
		ExecutionTime: time.Millisecond * 100,
	}

	mrInfo := &gitlab.MRInfo{
		ProjectID: 123,
		MRIID:     456,
		Author:    "testuser",
		Title:     "Test MR",
	}

	err := handler.handleApprovalWithComments(result, mrInfo)

	assert.NoError(t, err)
	assert.Equal(t, 2, callCount, "Should have made 2 approval attempts (with message, then fallback)")
}

func TestHandleApprovalWithComments_BothApprovalsFail(t *testing.T) {
	// Create test GitLab server that fails both approval attempts
	gitlabServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/approve") {
			w.WriteHeader(401)
			_, _ = w.Write([]byte(`{"message": "Unauthorized"}`))
		}
	}))
	defer gitlabServer.Close()

	cfg := &config.Config{
		GitLab: config.GitLabConfig{
			BaseURL: gitlabServer.URL,
			Token:   "invalid-token",
		},
		Comments: config.CommentsConfig{
			EnableMRComments: false,
		},
	}

	handler := &DataProductConfigMrReviewHandler{
		gitlabClient: gitlab.NewClientWithConfig(cfg),
		config:       cfg,
	}

	result := &shared.RuleEvaluation{
		FinalDecision: shared.Decision{
			Type:   shared.Approve,
			Reason: "Test approval",
		},
		RuleResults:   []shared.RuleResult{},
		ExecutionTime: time.Millisecond * 100,
	}

	mrInfo := &gitlab.MRInfo{
		ProjectID: 123,
		MRIID:     456,
		Author:    "testuser",
		Title:     "Test MR",
	}

	err := handler.handleApprovalWithComments(result, mrInfo)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to approve MR (both with message and simple)")
}

func TestWebhookHandler_FullApprovalWorkflow(t *testing.T) {
	// Integration test for the full approval workflow

	// Create mock GitLab server for changes API (to avoid manual review due to API failure)
	changesServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/changes") {
			// Return mock changes that should trigger approval
			w.WriteHeader(200)
			_, _ = w.Write([]byte(`{
				"changes": [
					{
						"old_path": "dataproducts/agg/test/prod/product.yaml",
						"new_path": "dataproducts/agg/test/prod/product.yaml",
						"new_file": false,
						"renamed_file": false,
						"deleted_file": false
					}
				]
			}`))
		} else if strings.Contains(r.URL.Path, "/notes") {
			// Mock comment creation
			w.WriteHeader(201)
			_, _ = w.Write([]byte(`{"id": 123}`))
		} else if strings.Contains(r.URL.Path, "/approve") {
			// Mock approval
			w.WriteHeader(201)
			_, _ = w.Write([]byte(`{"approved": true}`))
		}
	}))
	defer changesServer.Close()

	cfg := &config.Config{
		GitLab: config.GitLabConfig{
			BaseURL: changesServer.URL,
			Token:   "test-token",
		},
		Comments: config.CommentsConfig{
			EnableMRComments: true,
			CommentVerbosity: "detailed",
		},
	}

	// Create webhook handler with real GitLab client but mock rule manager
	gitlabClient := gitlab.NewClientWithConfig(cfg)
	handler := &DataProductConfigMrReviewHandler{
		gitlabClient: gitlabClient,
		ruleManager: &MockRuleManagerForApproval{
			evaluateFunc: func(ctx *shared.MRContext) *shared.RuleEvaluation {
				// Return approval decision to trigger approval workflow
				return &shared.RuleEvaluation{
					FinalDecision: shared.Decision{
						Type:   shared.Approve,
						Reason: "Safe warehouse changes detected",
					},
					RuleResults: []shared.RuleResult{
						{
							RuleName: "warehouse_rule",
							Decision: shared.Decision{
								Type:   shared.Approve,
								Reason: "All warehouse changes are decreases",
							},
							Metadata: map[string]any{
								"analyzed_files":    []string{"dataproducts/agg/test/prod/product.yaml"},
								"warehouse_changes": []interface{}{"LARGE->MEDIUM"},
							},
						},
					},
					ExecutionTime: time.Millisecond * 200,
				}
			},
		},
		config: cfg,
	}

	// Create Fiber app and test the full webhook
	app := fiber.New()
	app.Post("/webhook", handler.HandleWebhook)

	// Create test request
	payload := map[string]interface{}{
		"object_kind": "merge_request",
		"object_attributes": map[string]interface{}{
			"iid":   123,
			"title": "Test warehouse decrease",
		},
		"project": map[string]interface{}{
			"id": 456,
		},
		"user": map[string]interface{}{
			"username": "testuser",
		},
	}

	jsonPayload, _ := json.Marshal(payload)
	req := httptest.NewRequest("POST", "/webhook", bytes.NewReader(jsonPayload))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	assert.NoError(t, err)
	assert.Equal(t, 200, resp.StatusCode)

	// Parse response
	body := new(bytes.Buffer)
	_, _ = body.ReadFrom(resp.Body)
	var response map[string]interface{}
	_ = json.Unmarshal(body.Bytes(), &response)

	// Verify approval workflow executed
	assert.Equal(t, "processed", response["webhook_response"])
	assert.Equal(t, true, response["mr_approved"])
	assert.Equal(t, float64(456), response["project_id"])
	assert.Equal(t, float64(123), response["mr_iid"])

	decision := response["decision"].(map[string]interface{})
	assert.Equal(t, "approve", decision["type"])
}
