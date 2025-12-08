package webhook

import (
	"fmt"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/redhat-data-and-ai/naysayer/internal/config"
	"github.com/redhat-data-and-ai/naysayer/internal/gitlab"
	"github.com/redhat-data-and-ai/naysayer/internal/logging"
)

// StaleMRCleanupHandler handles stale MR cleanup requests
type StaleMRCleanupHandler struct {
	config *config.Config
	client gitlab.GitLabClient
}

// StaleMRCleanupPayload represents the payload for stale MR cleanup webhook
type StaleMRCleanupPayload struct {
	ProjectID   int  `json:"project_id"`   // Required: GitLab project ID
	ClosureDays int  `json:"closure_days"` // Optional: Override default closure threshold
	DryRun      bool `json:"dry_run"`      // Optional: Test mode (no actual changes)
}

// StaleMRCleanupResponse represents the response from stale MR cleanup
type StaleMRCleanupResponse struct {
	WebhookResponse string `json:"webhook_response"`
	Status          string `json:"status"`
	ProjectID       int    `json:"project_id"`
	ClosureDays     int    `json:"closure_days"`
	DryRun          bool   `json:"dry_run"`
	TotalMRs        int    `json:"total_mrs"`
	Closed          int    `json:"closed"`
	Failed          int    `json:"failed"`
}

// NewStaleMRCleanupHandler creates a new stale MR cleanup handler
func NewStaleMRCleanupHandler(cfg *config.Config) *StaleMRCleanupHandler {
	return &StaleMRCleanupHandler{
		config: cfg,
		client: gitlab.NewClient(cfg.GitLab),
	}
}

// NewStaleMRCleanupHandlerWithClient creates a handler with a custom GitLab client (for testing)
func NewStaleMRCleanupHandlerWithClient(cfg *config.Config, client gitlab.GitLabClient) *StaleMRCleanupHandler {
	return &StaleMRCleanupHandler{
		config: cfg,
		client: client,
	}
}

// HandleWebhook processes stale MR cleanup webhook requests
func (h *StaleMRCleanupHandler) HandleWebhook(c *fiber.Ctx) error {
	// Validate content type
	if c.Get("Content-Type") != "application/json" {
		logging.Warn("Invalid content type for stale MR cleanup: %s", c.Get("Content-Type"))
		return c.Status(400).JSON(fiber.Map{
			"error": "Content-Type must be application/json",
		})
	}

	// Parse payload
	var payload StaleMRCleanupPayload
	if err := c.BodyParser(&payload); err != nil {
		logging.Warn("Failed to parse stale MR cleanup payload: %v", err)
		return c.Status(400).JSON(fiber.Map{
			"error": "Invalid JSON payload",
		})
	}

	// Validate payload
	if err := h.validatePayload(&payload); err != nil {
		logging.Warn("Invalid stale MR cleanup payload: %v", err)
		return c.Status(400).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	// Set defaults if not provided
	if payload.ClosureDays == 0 {
		payload.ClosureDays = h.config.StaleMR.ClosureDays
	}

	logging.Info("Starting stale MR cleanup for project %d (closure: %d days, dry_run: %t)",
		payload.ProjectID, payload.ClosureDays, payload.DryRun)

	// Process cleanup
	response, err := h.processCleanup(&payload)
	if err != nil {
		logging.Error("Stale MR cleanup failed for project %d: %v", payload.ProjectID, err)
		return c.Status(500).JSON(fiber.Map{
			"error": "Internal server error during cleanup",
		})
	}

	logging.Info("Stale MR cleanup completed for project %d: %d closed, %d failed",
		payload.ProjectID, response.Closed, response.Failed)

	return c.JSON(response)
}

// validatePayload validates the stale MR cleanup payload
func (h *StaleMRCleanupHandler) validatePayload(payload *StaleMRCleanupPayload) error {
	if payload.ProjectID == 0 {
		return fmt.Errorf("project_id is required")
	}

	if payload.ClosureDays < 0 {
		return fmt.Errorf("closure_days must be >= 0")
	}

	return nil
}

// processCleanup processes the stale MR cleanup workflow
func (h *StaleMRCleanupHandler) processCleanup(payload *StaleMRCleanupPayload) (*StaleMRCleanupResponse, error) {
	// Fetch all open MRs
	mrs, err := h.client.ListAllOpenMRsWithDetails(payload.ProjectID)
	if err != nil {
		return nil, fmt.Errorf("failed to list open MRs: %w", err)
	}

	response := &StaleMRCleanupResponse{
		WebhookResponse: "processed",
		Status:          "completed",
		ProjectID:       payload.ProjectID,
		ClosureDays:     payload.ClosureDays,
		DryRun:          payload.DryRun,
		TotalMRs:        len(mrs),
		Closed:          0,
		Failed:          0,
	}

	now := time.Now()

	// Process each MR
	for _, mr := range mrs {
		// Parse UpdatedAt timestamp
		updatedAt, err := time.Parse(time.RFC3339, mr.UpdatedAt)
		if err != nil {
			logging.Warn("Failed to parse updated_at for MR !%d: %v", mr.IID, err)
			response.Failed++
			continue
		}

		daysSinceUpdate := int(now.Sub(updatedAt).Hours() / 24)

		// Close if >= threshold
		if daysSinceUpdate >= payload.ClosureDays {
			if err := h.closeStaleMR(payload.ProjectID, mr.IID, payload.ClosureDays, daysSinceUpdate, payload.DryRun); err != nil {
				logging.Error("Failed to close MR !%d: %v", mr.IID, err)
				response.Failed++
			} else {
				response.Closed++
				logging.Info("Closed stale MR !%d (inactive for %d days)", mr.IID, daysSinceUpdate)
			}
		}
	}

	return response, nil
}

// closeStaleMR adds a closure comment and closes a stale MR
func (h *StaleMRCleanupHandler) closeStaleMR(projectID, mrIID, closureDays, daysSinceUpdate int, dryRun bool) error {
	comment := fmt.Sprintf(`**Automated Closure - Stale Merge Request**

This merge request has been automatically closed due to inactivity (%d days with no updates).

If you still want to merge this change, please:
1. Reopen this MR
2. Rebase with the latest changes
3. Address any conflicts or review comments

_This is an automated action performed by the stale MR cleanup process._`, daysSinceUpdate)

	if dryRun {
		logging.Info("[DRY RUN] Would close MR !%d", mrIID)
		return nil
	}

	// Add closure comment first
	if err := h.client.AddMRComment(projectID, mrIID, comment); err != nil {
		return fmt.Errorf("failed to add closure comment: %w", err)
	}

	// Close the MR
	if err := h.client.CloseMR(projectID, mrIID); err != nil {
		return fmt.Errorf("failed to close MR: %w", err)
	}

	return nil
}
