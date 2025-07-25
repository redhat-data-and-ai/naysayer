package handlers

import (
	"encoding/json"
	"fmt"
	"log"

	"github.com/gofiber/fiber/v2"
	"github.com/redhat-data-and-ai/naysayer/pkg/analysis"
	"github.com/redhat-data-and-ai/naysayer/pkg/config"
)

// GitLab webhook structures
type MergeRequestWebhook struct {
	ObjectKind       string                 `json:"object_kind"`
	EventType        string                 `json:"event_type"`
	User             User                   `json:"user"`
	Project          Project                `json:"project"`
	ObjectAttributes ObjectAttributes       `json:"object_attributes"`
	Changes          map[string]interface{} `json:"changes"`
	Repository       Repository             `json:"repository"`
}

type User struct {
	ID        int    `json:"id"`
	Name      string `json:"name"`
	Username  string `json:"username"`
	Email     string `json:"email"`
	AvatarURL string `json:"avatar_url"`
}

type Project struct {
	ID                int    `json:"id"`
	Name              string `json:"name"`
	Description       string `json:"description"`
	WebURL            string `json:"web_url"`
	AvatarURL         string `json:"avatar_url"`
	GitSSHURL         string `json:"git_ssh_url"`
	GitHTTPURL        string `json:"git_http_url"`
	Namespace         string `json:"namespace"`
	VisibilityLevel   int    `json:"visibility_level"`
	PathWithNamespace string `json:"path_with_namespace"`
	DefaultBranch     string `json:"default_branch"`
	Homepage          string `json:"homepage"`
	URL               string `json:"url"`
	SSHURL            string `json:"ssh_url"`
	HTTPURL           string `json:"http_url"`
}

type ObjectAttributes struct {
	ID              int      `json:"id"`
	IID             int      `json:"iid"`
	Title           string   `json:"title"`
	Description     string   `json:"description"`
	State           string   `json:"state"`
	CreatedAt       string   `json:"created_at"`
	UpdatedAt       string   `json:"updated_at"`
	TargetBranch    string   `json:"target_branch"`
	SourceBranch    string   `json:"source_branch"`
	AuthorID        int      `json:"author_id"`
	AssigneeID      int      `json:"assignee_id"`
	AssigneeIDs     []int    `json:"assignee_ids"`
	ReviewerIDs     []int    `json:"reviewer_ids"`
	Source          Project  `json:"source"`
	Target          Project  `json:"target"`
	LastCommit      Commit   `json:"last_commit"`
	WorkInProgress  bool     `json:"work_in_progress"`
	URL             string   `json:"url"`
	Action          string   `json:"action"`
	Assignee        User     `json:"assignee"`
	DetailedMergeStatus string `json:"detailed_merge_status"`
	// Removed HeadPipelineID - no longer processing pipelines
	MergeStatus     string   `json:"merge_status"`
}

type Commit struct {
	ID        string `json:"id"`
	Message   string `json:"message"`
	Timestamp string `json:"timestamp"`
	URL       string `json:"url"`
	Author    Author `json:"author"`
}

type Author struct {
	Name  string `json:"name"`
	Email string `json:"email"`
}

type Repository struct {
	Name        string `json:"name"`
	URL         string `json:"url"`
	Description string `json:"description"`
	Homepage    string `json:"homepage"`
}

// Removed pipeline structures - focusing only on warehouse changes

// WebhookHandler handles GitLab webhook processing
type WebhookHandler struct {
	config   *config.Config
	analyzer *analysis.DiffAnalyzer
}

// NewWebhookHandler creates a new webhook handler
func NewWebhookHandler(cfg *config.Config) *WebhookHandler {
	return &WebhookHandler{
		config:   cfg,
		analyzer: analysis.NewDiffAnalyzer(),
	}
}

// ReviewMR processes GitLab MR webhook and determines approval requirements
func (h *WebhookHandler) ReviewMR() fiber.Handler {
	return func(c *fiber.Ctx) error {
		log.Printf("Received webhook: %s", c.Get("X-Gitlab-Event"))
		
		var webhook MergeRequestWebhook
		if err := c.BodyParser(&webhook); err != nil {
			log.Printf("Error parsing webhook body: %v", err)
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "Invalid webhook payload",
			})
		}

		// Validate that this is from the expected repository
		if !h.isExpectedRepository(webhook.Project.PathWithNamespace) {
			log.Printf("Webhook from unexpected repository: %s", webhook.Project.PathWithNamespace)
			return c.JSON(fiber.Map{
				"message": "Repository not monitored",
			})
		}

		// Log the received webhook for debugging (in development only)
		if h.config.LogLevel == "debug" {
			webhookJSON, _ := json.MarshalIndent(webhook, "", "  ")
			log.Printf("Webhook payload: %s", string(webhookJSON))
		}

		// Only process merge request events
		if webhook.ObjectKind != "merge_request" {
			log.Printf("Ignoring non-merge-request event: %s", webhook.ObjectKind)
			return c.JSON(fiber.Map{
				"message": "Event ignored - not a merge request",
			})
		}

		// Only process specific actions (opened, updated)
		action := webhook.ObjectAttributes.Action
		if action != "open" && action != "update" && action != "reopen" {
			log.Printf("Ignoring MR action: %s", action)
			return c.JSON(fiber.Map{
				"message": fmt.Sprintf("Action ignored: %s", action),
			})
		}

		// Analyze the MR for warehouse changes only
		decision := h.analyzer.AnalyzeMRTitle(
			webhook.ObjectAttributes.Title,
			webhook.ObjectAttributes.Description,
		)

		// Log the decision
		log.Printf("Approval decision for MR !%d (%s): requires_approval=%t, type=%s, reason=%s", 
			webhook.ObjectAttributes.IID,
			webhook.ObjectAttributes.Title,
			decision.RequiresApproval, 
			decision.ApprovalType, 
			decision.Reason)

		// In Phase 2, we'll actually interact with GitLab API here
		// Return the decision (warehouse changes only)
		response := fiber.Map{
			"mr_id":             webhook.ObjectAttributes.IID,
			"mr_title":          webhook.ObjectAttributes.Title,
			"mr_url":            webhook.ObjectAttributes.URL,
			"repository":        webhook.Project.PathWithNamespace,
			"author":            webhook.User.Name,
			"source_branch":     webhook.ObjectAttributes.SourceBranch,
			"target_branch":     webhook.ObjectAttributes.TargetBranch,
			"decision":          decision,
			"naysayer_version":  "1.0.0-phase1-warehouse-only",
		}

		return c.JSON(response)
	}
}

// Removed all pipeline-related code - NAYSAYER now focuses only on warehouse changes

// isExpectedRepository checks if the webhook is from the dataproduct-config repo
func (h *WebhookHandler) isExpectedRepository(repoPath string) bool {
	expectedRepo := h.config.DataProductRepo
	return repoPath == expectedRepo || expectedRepo == "" // Allow empty for development
}

// HealthCheck provides a health check endpoint
func HealthCheck() fiber.Handler {
	return func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{
			"status":  "healthy",
			"service": "naysayer",
			"version": "1.0.0-phase1",
		})
	}
}

// Legacy ReviewMR function for backward compatibility
func ReviewMR() fiber.Handler {
	// Use default configuration for backward compatibility
	cfg := config.LoadConfig()
	handler := NewWebhookHandler(cfg)
	return handler.ReviewMR()
}
