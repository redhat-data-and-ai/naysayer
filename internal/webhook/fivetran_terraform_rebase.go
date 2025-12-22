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

// AutoRebaseHandler handles auto-rebase requests (generic, reusable across repositories)
type AutoRebaseHandler struct {
	gitlabClient gitlab.GitLabClient
	config       *config.Config
}

// FivetranTerraformRebaseHandler is an alias for backward compatibility
// Deprecated: Use AutoRebaseHandler instead
type FivetranTerraformRebaseHandler = AutoRebaseHandler

// NewAutoRebaseHandler creates a new auto-rebase handler
func NewAutoRebaseHandler(cfg *config.Config) *AutoRebaseHandler {
	// Use repository-specific token if configured, otherwise use main token
	token := cfg.AutoRebase.RepositoryToken
	if token == "" {
		token = cfg.GitLab.Token
		logging.Info("Using main GITLAB_TOKEN for auto-rebase")
	} else {
		logging.Info("Using repository-specific token for auto-rebase")
	}

	// Create a custom config with the appropriate token
	gitlabConfig := config.GitLabConfig{
		BaseURL:     cfg.GitLab.BaseURL,
		Token:       token,
		InsecureTLS: cfg.GitLab.InsecureTLS,
		CACertPath:  cfg.GitLab.CACertPath,
	}

	gitlabClient := gitlab.NewClient(gitlabConfig)
	return NewAutoRebaseHandlerWithClient(cfg, gitlabClient)
}

// NewAutoRebaseHandlerWithClient creates a handler with a custom GitLab client
// This is primarily used for testing with mock clients
func NewAutoRebaseHandlerWithClient(cfg *config.Config, client gitlab.GitLabClient) *AutoRebaseHandler {
	atlantisCheckStatus := "disabled"
	if cfg.AutoRebase.CheckAtlantisComments {
		atlantisCheckStatus = "enabled"
	}
	logging.Info("Auto-rebase handler initialized",
		zap.Bool("atlantis_comment_check_enabled", cfg.AutoRebase.CheckAtlantisComments),
		zap.String("atlantis_check_status", atlantisCheckStatus),
		zap.Bool("auto_rebase_enabled", cfg.AutoRebase.Enabled))
	return &AutoRebaseHandler{
		gitlabClient: client,
		config:       cfg,
	}
}

// NewFivetranTerraformRebaseHandler creates a new handler (backward compatibility)
// Deprecated: Use NewAutoRebaseHandler instead
func NewFivetranTerraformRebaseHandler(cfg *config.Config) *AutoRebaseHandler {
	return NewAutoRebaseHandler(cfg)
}

// NewFivetranTerraformRebaseHandlerWithClient creates a handler with a custom GitLab client (backward compatibility)
// Deprecated: Use NewAutoRebaseHandlerWithClient instead
func NewFivetranTerraformRebaseHandlerWithClient(cfg *config.Config, client gitlab.GitLabClient) *AutoRebaseHandler {
	return NewAutoRebaseHandlerWithClient(cfg, client)
}

// HandleWebhook handles auto-rebase requests
func (h *AutoRebaseHandler) HandleWebhook(c *fiber.Ctx) error {
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
func (h *AutoRebaseHandler) handlePushToMain(c *fiber.Ctx, payload map[string]interface{}, targetBranch string) error {
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
	filterResult := h.filterEligibleMRs(projectID, allMRs)
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
		logging.Info("Attempting to rebase MR", zap.Int("mr_iid", mr.IID))

		err := h.gitlabClient.RebaseMR(projectID, mr.IID)
		if err != nil {
			logging.Warn("Failed to rebase MR", zap.Int("mr_iid", mr.IID), zap.Error(err))
			failureCount++
			failures = append(failures, map[string]interface{}{
				"mr_iid": mr.IID,
				"error":  err.Error(),
			})
		} else {
			logging.Info("Successfully triggered rebase for MR", zap.Int("mr_iid", mr.IID))
			successCount++

			// Add comment to MR informing users about the automated rebase
			commentBody := "🤖 **Automated Rebase**\n\nThis merge request has been automatically rebased with the latest changes from the target branch.\n\n_This is an automated action triggered by a push to the main branch._"
			if commentErr := h.gitlabClient.AddMRComment(projectID, mr.IID, commentBody); commentErr != nil {
				logging.Warn("Failed to add rebase comment to MR", zap.Int("mr_iid", mr.IID), zap.Error(commentErr))
				// Don't fail the rebase if comment fails
			}
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

// filterEligibleMRs filters MRs based on pipeline status, jobs, and optionally atlantis comments
// Returns both eligible MRs and detailed skip information
// Note: MRs are already filtered by creation date at the API level (last 7 days)
func (h *AutoRebaseHandler) filterEligibleMRs(projectID int, mrs []gitlab.MRDetails) MRFilterResult {
	result := MRFilterResult{
		Eligible: make([]gitlab.MRDetails, 0),
		Skipped:  make([]MRSkipInfo, 0),
	}

	for _, mr := range mrs {
		// Check pipeline status
		if mr.Pipeline != nil {
			status := strings.ToLower(mr.Pipeline.Status)

			// Skip MRs with running or pending pipelines
			if status == "running" || status == "pending" {
				logging.Info("Skipping MR with %s pipeline", status, zap.Int("mr_iid", mr.IID))
				result.Skipped = append(result.Skipped, MRSkipInfo{
					MRIID:      mr.IID,
					Reason:     fmt.Sprintf("pipeline_%s", status),
					PipelineID: mr.Pipeline.ID,
				})
				continue
			}

			// For successful pipelines, proceed with rebase directly
			// Pipeline status = "success" means no failures, so no need to check jobs or atlantis comments
			// Just continue to add MR to eligible list (no special handling needed)

			// For failed pipelines, check all jobs first
			// If all jobs succeeded, optionally check atlantis comment for plan failures
			if status == "failed" {
				allJobsSucceeded, err := h.gitlabClient.AreAllPipelineJobsSucceeded(projectID, mr.Pipeline.ID)
				if err != nil {
					logging.Warn("Failed to check pipeline jobs for MR, skipping", zap.Int("mr_iid", mr.IID), zap.Error(err))
					result.Skipped = append(result.Skipped, MRSkipInfo{
						MRIID:      mr.IID,
						Reason:     "failed_to_check_jobs",
						PipelineID: mr.Pipeline.ID,
					})
					continue
				}

				if !allJobsSucceeded {
					logging.Info("Skipping MR with failed jobs", zap.Int("mr_iid", mr.IID))
					result.Skipped = append(result.Skipped, MRSkipInfo{
						MRIID:      mr.IID,
						Reason:     "pipeline_jobs_failed",
						PipelineID: mr.Pipeline.ID,
					})
					continue
				}

				// All jobs succeeded but pipeline is marked as failed
				// Check if we should check atlantis comments (configurable)
				logging.Info("Checking atlantis comment configuration",
					zap.Int("mr_iid", mr.IID),
					zap.Bool("check_atlantis_enabled", h.config.AutoRebase.CheckAtlantisComments))

				if h.config.AutoRebase.CheckAtlantisComments {
					logging.Info("Atlantis comment check enabled, checking for atlantis comments", zap.Int("mr_iid", mr.IID))
					// Check for atlantis comments
					atlantisComment, err := h.gitlabClient.FindLatestAtlantisComment(projectID, mr.IID)
					if err != nil {
						logging.Warn("Failed to find atlantis comment", zap.Int("mr_iid", mr.IID), zap.Error(err))
					}
					if err != nil || atlantisComment == nil {
						// No atlantis comment found - skip rebase (safe default)
						logging.Info("Skipping MR with failed pipeline (no atlantis comment found)", zap.Int("mr_iid", mr.IID))
						result.Skipped = append(result.Skipped, MRSkipInfo{
							MRIID:      mr.IID,
							Reason:     "pipeline_failed_atlantis_comment_not_found",
							PipelineID: mr.Pipeline.ID,
						})
						continue
					}

					logging.Info("Found atlantis comment, checking for plan failures", zap.Int("mr_iid", mr.IID))
					// Check atlantis comment to determine if it's a state lock (allow rebase) or plan error (skip rebase)
					shouldSkip, skipReason := h.gitlabClient.CheckAtlantisCommentForPlanFailures(projectID, mr.IID)
					logging.Info("Atlantis comment check result",
						zap.Int("mr_iid", mr.IID),
						zap.Bool("should_skip", shouldSkip),
						zap.String("skip_reason", skipReason))

					if shouldSkip && skipReason != "atlantis_plan_locked" {
						logging.Info("Skipping MR with failed pipeline due to plan error", zap.Int("mr_iid", mr.IID), zap.String("reason", skipReason))
						result.Skipped = append(result.Skipped, MRSkipInfo{
							MRIID:      mr.IID,
							Reason:     fmt.Sprintf("pipeline_failed_%s", skipReason),
							PipelineID: mr.Pipeline.ID,
						})
						continue
					}
					// If skipReason is "atlantis_plan_locked", we allow rebase (continue to eligible)
					logging.Info("Allowing rebase (state lock detected or no plan failure)", zap.Int("mr_iid", mr.IID), zap.String("skip_reason", skipReason))
				} else {
					// CheckAtlantisComments = false: Simple behavior - skip failed pipelines
					logging.Info("Skipping MR with failed pipeline (atlantis check disabled)", zap.Int("mr_iid", mr.IID))
					result.Skipped = append(result.Skipped, MRSkipInfo{
						MRIID:      mr.IID,
						Reason:     "pipeline_failed",
						PipelineID: mr.Pipeline.ID,
					})
					continue
				}
			}
		}

		// MR is eligible
		result.Eligible = append(result.Eligible, mr)
	}

	return result
}

// validateWebhookPayload performs validation on webhook payload
func (h *AutoRebaseHandler) validateWebhookPayload(payload map[string]interface{}) error {
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
