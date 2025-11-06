package webhook

import (
	"fmt"
	"strings"

	fiber "github.com/gofiber/fiber/v2"
	"go.uber.org/zap"

	"github.com/redhat-data-and-ai/naysayer/internal/config"
	"github.com/redhat-data-and-ai/naysayer/internal/gitlab"
	"github.com/redhat-data-and-ai/naysayer/internal/logging"
)

// FivetranTerraformRebaseHandler handles Fivetran Terraform rebase requests
type FivetranTerraformRebaseHandler struct {
	gitlabClient gitlab.GitLabClient
	config       *config.Config
}

// NewFivetranTerraformRebaseHandler creates a new Fivetran Terraform rebase handler
func NewFivetranTerraformRebaseHandler(cfg *config.Config) *FivetranTerraformRebaseHandler {
	// Create GitLab client with Fivetran-specific token if available
	gitlabConfig := cfg.GitLab

	// Use dedicated Fivetran token if configured, otherwise fall back to main token
	token := gitlabConfig.FivetranToken
	if token == "" {
		token = gitlabConfig.Token
		logging.Info("Using main GITLAB_TOKEN for Fivetran rebase (GITLAB_TOKEN_FIVETRAN not set)")
	} else {
		logging.Info("Using dedicated GITLAB_TOKEN_FIVETRAN for Fivetran rebase")
	}

	// Create a custom config with the appropriate token
	fivetranConfig := config.GitLabConfig{
		BaseURL:     gitlabConfig.BaseURL,
		Token:       token,
		InsecureTLS: gitlabConfig.InsecureTLS,
		CACertPath:  gitlabConfig.CACertPath,
	}

	gitlabClient := gitlab.NewClient(fivetranConfig)
	return NewFivetranTerraformRebaseHandlerWithClient(cfg, gitlabClient)
}

// NewFivetranTerraformRebaseHandlerWithClient creates a handler with a custom GitLab client
// This is primarily used for testing with mock clients
func NewFivetranTerraformRebaseHandlerWithClient(cfg *config.Config, client gitlab.GitLabClient) *FivetranTerraformRebaseHandler {
	logging.Info("Fivetran Terraform Rebase handler initialized")
	return &FivetranTerraformRebaseHandler{
		gitlabClient: client,
		config:       cfg,
	}
}

// HandleWebhook handles Fivetran Terraform rebase requests
func (h *FivetranTerraformRebaseHandler) HandleWebhook(c *fiber.Ctx) error {
	c.Set("Content-Type", "application/json")

	// Quick validation of content type
	contentType := c.Get("Content-Type")
	if !strings.Contains(contentType, "application/json") {
		logging.Warn("Invalid content type: %s", contentType)
		return c.Status(400).JSON(fiber.Map{
			"error": "Content-Type must be application/json",
		})
	}

	// Parse webhook payload
	var payload map[string]interface{}
	if err := c.BodyParser(&payload); err != nil {
		logging.Error("Failed to parse payload: %v", err)
		return c.Status(400).JSON(fiber.Map{
			"error": "Invalid JSON payload",
		})
	}

	// Validate webhook payload structure
	if err := h.validateWebhookPayload(payload); err != nil {
		logging.Warn("Webhook validation failed: %v", err)
		return c.Status(400).JSON(fiber.Map{
			"error": "Invalid webhook payload: " + err.Error(),
		})
	}

	// Get event type
	eventType, ok := payload["object_kind"].(string)
	if !ok {
		logging.Warn("Missing object_kind in payload")
		return c.Status(400).JSON(fiber.Map{
			"error": "Missing object_kind",
		})
	}

	// Handle push events to main branch (rebase all open MRs)
	if eventType == "push" {
		return h.handlePushToMain(c, payload)
	}

	// Unsupported event type
	logging.Warn("Skipping unsupported event: %s", eventType)
	return c.Status(400).JSON(fiber.Map{
		"error": fmt.Sprintf("Unsupported event type: %s. Only push events are supported.", eventType),
	})
}

// handlePushToMain handles push events to main branch by rebasing all open MRs
func (h *FivetranTerraformRebaseHandler) handlePushToMain(c *fiber.Ctx, payload map[string]interface{}) error {
	// Extract project ID
	project, ok := payload["project"].(map[string]interface{})
	if !ok {
		logging.Error("Missing project information in push payload")
		return c.Status(400).JSON(fiber.Map{
			"error": "Missing project information",
		})
	}

	projectID, ok := project["id"].(float64)
	if !ok {
		logging.Error("Invalid project ID in push payload")
		return c.Status(400).JSON(fiber.Map{
			"error": "Invalid project ID",
		})
	}

	// Extract branch reference
	ref, ok := payload["ref"].(string)
	if !ok {
		logging.Warn("Missing ref in push payload")
		return c.Status(400).JSON(fiber.Map{
			"error": "Missing ref in payload",
		})
	}

	// Check if push is to main/master branch
	targetBranch := strings.TrimPrefix(ref, "refs/heads/")
	if targetBranch != "main" && targetBranch != "master" {
		logging.Info("Ignoring push to non-main branch: %s", targetBranch)
		return c.JSON(fiber.Map{
			"webhook_response": "processed",
			"status":           "skipped",
			"reason":           fmt.Sprintf("Push to %s branch, only main/master triggers rebase", targetBranch),
			"branch":           targetBranch,
		})
	}

	logging.Info("Push to main branch detected, rebasing all open MRs",
		zap.String("branch", targetBranch),
		zap.Int("project_id", int(projectID)))

	// Get all open MRs
	mrIIDs, err := h.gitlabClient.ListOpenMRs(int(projectID))
	if err != nil {
		logging.Error("Failed to list open MRs: %v", err)
		return c.Status(500).JSON(fiber.Map{
			"error":      "Failed to list open MRs: " + err.Error(),
			"project_id": int(projectID),
		})
	}

	if len(mrIIDs) == 0 {
		logging.Info("No open MRs found to rebase")
		return c.JSON(fiber.Map{
			"webhook_response": "processed",
			"status":           "success",
			"message":          "No open MRs to rebase",
			"project_id":       int(projectID),
			"branch":           targetBranch,
			"mrs_rebased":      0,
		})
	}

	logging.Info("Found %d open MRs to rebase", len(mrIIDs))

	// Rebase all open MRs
	successCount := 0
	failureCount := 0
	failures := make([]map[string]interface{}, 0)

	for _, mrIID := range mrIIDs {
		logging.Info("Attempting to rebase MR", zap.Int("mr_iid", mrIID))

		err := h.gitlabClient.RebaseMR(int(projectID), mrIID)
		if err != nil {
			logging.Warn("Failed to rebase MR", zap.Int("mr_iid", mrIID), zap.Error(err))
			failureCount++
			failures = append(failures, map[string]interface{}{
				"mr_iid": mrIID,
				"error":  err.Error(),
			})
		} else {
			logging.Info("Successfully triggered rebase for MR", zap.Int("mr_iid", mrIID))
			successCount++
		}
	}

	// Build response
	response := fiber.Map{
		"webhook_response": "processed",
		"status":           "completed",
		"project_id":       int(projectID),
		"branch":           targetBranch,
		"total_mrs":        len(mrIIDs),
		"successful":       successCount,
		"failed":           failureCount,
	}

	if failureCount > 0 {
		response["failures"] = failures
	}

	logging.Info("Rebase operation completed",
		zap.Int("total", len(mrIIDs)),
		zap.Int("successful", successCount),
		zap.Int("failed", failureCount))

	return c.JSON(response)
}

// validateWebhookPayload performs validation on webhook payload
func (h *FivetranTerraformRebaseHandler) validateWebhookPayload(payload map[string]interface{}) error {
	// Check for required top-level fields
	if payload == nil {
		return fmt.Errorf("payload is nil")
	}

	// Validate project section (required for both push and MR events)
	if _, ok := payload["project"]; !ok {
		return fmt.Errorf("missing project information")
	}

	return nil
}
