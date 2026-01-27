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
	token := gitlabConfig.GitlabFivetranRepositoryToken
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
	if !c.Is("json") {
		contentType := c.Get("Content-Type")
		logging.Warn("Invalid content type: %s", contentType)
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": fmt.Sprintf("Content-Type must be application/json, got: %s", contentType),
		})
	}

	// Parse webhook payload
	var payload map[string]interface{}
	if err := c.BodyParser(&payload); err != nil {
		logging.Error("Failed to parse payload: %v", err)
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": fmt.Sprintf("Invalid JSON payload: %v", err),
		})
	}

	// Validate webhook payload structure
	if err := h.validateWebhookPayload(payload); err != nil {
		logging.Warn("Webhook validation failed: %v", err)
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": fmt.Sprintf("Invalid webhook payload: %v", err),
		})
	}

	// Get event type
	eventType, ok := payload["object_kind"].(string)
	if !ok {
		logging.Warn("Missing object_kind in payload")
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Missing object_kind in payload",
		})
	}

	// Handle push events to main branch (rebase all open MRs)
	if eventType == "push" {
		// Extract branch reference
		ref, ok := payload["ref"].(string)
		if !ok {
			logging.Warn("Missing ref in push payload")
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
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

		return h.handlePushToMain(c, payload, targetBranch)
	}

	// Unsupported event type
	logging.Warn("Skipping unsupported event: %s", eventType)
	return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
		"error": fmt.Sprintf("Unsupported event type: %s. Only push events are supported.", eventType),
	})
}

// handlePushToMain handles push events to main branch by rebasing all open MRs
// targetBranch is already validated to be "main" or "master" by the caller
func (h *FivetranTerraformRebaseHandler) handlePushToMain(c *fiber.Ctx, payload map[string]interface{}, targetBranch string) error {
	// Extract project ID
	project, ok := payload["project"].(map[string]interface{})
	if !ok {
		logging.Error("Missing project information in push payload")
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Missing project information",
		})
	}

	projectIDFloat, ok := project["id"].(float64)
	if !ok {
		logging.Error("Invalid project ID in push payload")
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid project ID",
		})
	}

	// Convert projectID to int once and reuse throughout
	projectID := int(projectIDFloat)

	logging.Info("Push to main branch detected, rebasing eligible open MRs",
		zap.String("branch", targetBranch),
		zap.Int("project_id", projectID))

	// Get all open MRs with details (already filtered by created_after at API level)
	allMRs, err := h.gitlabClient.ListOpenMRsWithDetails(projectID)
	if err != nil {
		logging.Error("Failed to list open MRs: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error":      fmt.Sprintf("Failed to list open MRs: %v", err),
			"project_id": projectID,
		})
	}

	// Filter MRs based on pipeline status
	// Note: Date filtering is already done at API level via created_after parameter
	filterResult := h.filterEligibleMRs(allMRs)
	eligibleMRs := filterResult.Eligible

	if len(eligibleMRs) == 0 {
		logging.Info("No eligible MRs found to rebase")
		return c.JSON(fiber.Map{
			"webhook_response": "processed",
			"status":           "completed",
			"project_id":       projectID,
			"branch":           targetBranch,
			"total_mrs":        len(allMRs),
			"eligible_mrs":     0,
			"successful":       0,
			"failed":           0,
			"skipped":          len(allMRs),
			"skip_details":     filterResult.Skipped,
		})
	}

	logging.Info("Found %d eligible MRs to rebase out of %d total open MRs", len(eligibleMRs), len(allMRs))

	// Rebase all eligible MRs
	successCount := 0
	failureCount := 0
	failures := make([]map[string]interface{}, 0)

	for _, mr := range eligibleMRs {
		logging.Info("Attempting to rebase MR", zap.Int("mr_iid", mr.IID), zap.Int("behind_commits", mr.BehindCommitsCount))

		// Pre-check: Skip if already up-to-date
		if mr.BehindCommitsCount == 0 {
			logging.Info("MR is already up-to-date, skipping rebase", zap.Int("mr_iid", mr.IID))
			continue
		}

		// Pre-check: Skip if has conflicts
		if mr.HasConflicts || mr.MergeStatus == "cannot_be_merged" {
			logging.Warn("MR has conflicts, skipping rebase",
				zap.Int("mr_iid", mr.IID),
				zap.String("merge_status", mr.MergeStatus))
			failureCount++
			failures = append(failures, map[string]interface{}{
				"mr_iid": mr.IID,
				"error":  fmt.Sprintf("rebase skipped: MR has merge conflicts (merge_status: %s)", mr.MergeStatus),
			})
			continue
		}

		success, actuallyRebased, err := h.gitlabClient.RebaseMR(projectID, mr.IID)
		if err != nil {
			logging.Warn("Failed to rebase MR", zap.Int("mr_iid", mr.IID), zap.Error(err))
			failureCount++
			failures = append(failures, map[string]interface{}{
				"mr_iid": mr.IID,
				"error":  err.Error(),
			})
		} else if success && actuallyRebased {
			logging.Info("Successfully rebased MR", zap.Int("mr_iid", mr.IID))
			successCount++

			// Only add comment if rebase was actually performed
			commentBody := "🤖 **Automated Rebase**\n\nThis merge request has been automatically rebased with the latest changes from the target branch.\n\n_This is an automated action triggered by a push to the main branch._"
			if commentErr := h.gitlabClient.AddMRComment(projectID, mr.IID, commentBody); commentErr != nil {
				logging.Warn("Failed to add rebase comment to MR", zap.Int("mr_iid", mr.IID), zap.Error(commentErr))
			}
		} else if success && !actuallyRebased {
			// Rebase API succeeded but no rebase was needed (already up-to-date)
			logging.Info("Rebase not needed for MR (already up-to-date)", zap.Int("mr_iid", mr.IID))
			// Don't count as success or failure, just skip
		}
	}

	// Build response
	response := fiber.Map{
		"webhook_response": "processed",
		"status":           "completed",
		"project_id":       projectID,
		"branch":           targetBranch,
		"total_mrs":        len(allMRs),
		"eligible_mrs":     len(eligibleMRs),
		"successful":       successCount,
		"failed":           failureCount,
		"skipped":          len(allMRs) - len(eligibleMRs),
		"skip_details":     filterResult.Skipped,
	}

	if failureCount > 0 {
		response["failures"] = failures
	}

	logging.Info("Rebase operation completed",
		zap.Int("total", len(allMRs)),
		zap.Int("eligible", len(eligibleMRs)),
		zap.Int("successful", successCount),
		zap.Int("failed", failureCount))

	return c.JSON(response)
}

// MRSkipInfo holds information about why an MR was skipped
type MRSkipInfo struct {
	MRIID      int    `json:"mr_iid"`
	Reason     string `json:"reason"`
	PipelineID int    `json:"pipeline_id,omitempty"`
	CreatedAt  string `json:"created_at,omitempty"`
}

// MRFilterResult contains both eligible MRs and skip information
type MRFilterResult struct {
	Eligible []gitlab.MRDetails
	Skipped  []MRSkipInfo
}

// filterEligibleMRs filters MRs based on pipeline status
// Returns both eligible MRs and detailed skip information
// Note: MRs are already filtered by creation date at the API level (last 7 days)
func (h *FivetranTerraformRebaseHandler) filterEligibleMRs(mrs []gitlab.MRDetails) MRFilterResult {
	result := MRFilterResult{
		Eligible: make([]gitlab.MRDetails, 0),
		Skipped:  make([]MRSkipInfo, 0),
	}

	for _, mr := range mrs {
		// Check pipeline status
		if mr.Pipeline != nil {
			status := strings.ToLower(mr.Pipeline.Status)
			if status == "running" || status == "pending" || status == "failed" {
				logging.Info("Skipping MR with %s pipeline", status, zap.Int("mr_iid", mr.IID))
				result.Skipped = append(result.Skipped, MRSkipInfo{
					MRIID:      mr.IID,
					Reason:     fmt.Sprintf("pipeline_%s", status),
					PipelineID: mr.Pipeline.ID,
				})
				continue
			}
		}

		// MR is eligible
		result.Eligible = append(result.Eligible, mr)
	}

	return result
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
