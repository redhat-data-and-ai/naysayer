package webhook

import (
	"fmt"
	"strings"

	fiber "github.com/gofiber/fiber/v2"
	"go.uber.org/zap"

	"github.com/redhat-data-and-ai/naysayer/internal/config"
	"github.com/redhat-data-and-ai/naysayer/internal/gitlab"
	"github.com/redhat-data-and-ai/naysayer/internal/logging"
	"github.com/redhat-data-and-ai/naysayer/internal/utils"
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

	// Only support MR events
	eventType, ok := payload["object_kind"].(string)
	if !ok {
		logging.Warn("Missing object_kind in payload")
		return c.Status(400).JSON(fiber.Map{
			"error": "Missing object_kind",
		})
	}

	if eventType != "merge_request" {
		logging.Warn("Skipping unsupported event: %s", eventType)
		return c.Status(400).JSON(fiber.Map{
			"error": fmt.Sprintf("Unsupported event type: %s. Only merge_request events are supported.", eventType),
		})
	}

	return h.handleMergeRequestRebase(c, payload)
}

// handleMergeRequestRebase handles the rebase operation for a merge request
func (h *FivetranTerraformRebaseHandler) handleMergeRequestRebase(c *fiber.Ctx, payload map[string]interface{}) error {
	// Extract MR information
	mrInfo, err := gitlab.ExtractMRInfo(payload)
	if err != nil {
		logging.Error("Failed to extract MR info: %v", err)
		return c.Status(400).JSON(fiber.Map{
			"error": "Missing MR information: " + err.Error(),
		})
	}

	logging.MRInfo(mrInfo.MRIID, "Processing rebase request",
		zap.Int("project_id", mrInfo.ProjectID),
		zap.String("author", mrInfo.Author),
		zap.String("state", mrInfo.State))

	// Skip rebase if MR is not open
	if mrInfo.State != utils.MRStateOpened {
		logging.MRInfo(mrInfo.MRIID, "Skipping rebase for non-open MR",
			zap.String("state", mrInfo.State))

		return c.JSON(fiber.Map{
			"webhook_response": "processed",
			"event_type":       "merge_request_rebase",
			"status":           "skipped",
			"reason":           fmt.Sprintf("MR state is '%s', only processing open MRs", mrInfo.State),
			"rebased":          false,
			"project_id":       mrInfo.ProjectID,
			"mr_iid":           mrInfo.MRIID,
		})
	}

	// Trigger the rebase operation
	logging.MRInfo(mrInfo.MRIID, "Triggering rebase operation")
	if err := h.gitlabClient.RebaseMR(mrInfo.ProjectID, mrInfo.MRIID); err != nil {
		logging.MRError(mrInfo.MRIID, "Failed to trigger rebase", err)

		// Add a comment if enabled to inform about the failure
		if h.config.Comments.EnableMRComments {
			comment := fmt.Sprintf("🔄 **Naysayer Rebase Failed**\n\n"+
				"Failed to trigger automatic rebase for this merge request.\n\n"+
				"**Error:** %s\n\n"+
				"Please manually rebase or check the merge request status.",
				err.Error())

			if addCommentErr := h.gitlabClient.AddMRComment(mrInfo.ProjectID, mrInfo.MRIID, comment); addCommentErr != nil {
				logging.MRWarn(mrInfo.MRIID, "Failed to add failure comment", zap.Error(addCommentErr))
			}
		}

		return c.Status(500).JSON(fiber.Map{
			"error":      "Failed to trigger rebase: " + err.Error(),
			"rebased":    false,
			"project_id": mrInfo.ProjectID,
			"mr_iid":     mrInfo.MRIID,
		})
	}

	logging.MRInfo(mrInfo.MRIID, "Rebase triggered successfully")

	// Add a success comment if enabled
	if h.config.Comments.EnableMRComments {
		comment := "🔄 **Naysayer Rebase Triggered**\n\n" +
			"Automatic rebase has been initiated for this merge request.\n\n" +
			"The rebase operation is running in the background. " +
			"Please wait a few moments for it to complete."

		if err := h.gitlabClient.AddMRComment(mrInfo.ProjectID, mrInfo.MRIID, comment); err != nil {
			logging.MRWarn(mrInfo.MRIID, "Failed to add success comment", zap.Error(err))
			// Continue - comment failure shouldn't block the webhook response
		} else {
			logging.MRInfo(mrInfo.MRIID, "Added rebase notification comment")
		}
	}

	// Return success response
	return c.JSON(fiber.Map{
		"webhook_response": "processed",
		"event_type":       "merge_request_rebase",
		"status":           "success",
		"rebased":          true,
		"project_id":       mrInfo.ProjectID,
		"mr_iid":           mrInfo.MRIID,
		"message":          "Rebase operation triggered successfully",
	})
}

// validateWebhookPayload performs validation on webhook payload
func (h *FivetranTerraformRebaseHandler) validateWebhookPayload(payload map[string]interface{}) error {
	// Check for required top-level fields
	if payload == nil {
		return fmt.Errorf("payload is nil")
	}

	// Validate object_attributes section
	objectAttrs, ok := payload["object_attributes"]
	if !ok {
		return fmt.Errorf("missing object_attributes")
	}

	objectAttrsMap, ok := objectAttrs.(map[string]interface{})
	if !ok {
		return fmt.Errorf("object_attributes must be an object")
	}

	// Validate required fields
	if _, exists := objectAttrsMap["iid"]; !exists {
		return fmt.Errorf("missing iid in object_attributes")
	}

	// Validate project section
	if _, ok := payload["project"]; !ok {
		return fmt.Errorf("missing project information")
	}

	return nil
}
