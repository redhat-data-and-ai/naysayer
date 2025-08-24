package webhook

import (
	"fmt"
	"strconv"
	"strings"

	fiber "github.com/gofiber/fiber/v2"
	"github.com/redhat-data-and-ai/naysayer/internal/config"
	"github.com/redhat-data-and-ai/naysayer/internal/gitlab"
	"github.com/redhat-data-and-ai/naysayer/internal/logging"
	"github.com/redhat-data-and-ai/naysayer/internal/rules"
	"github.com/redhat-data-and-ai/naysayer/internal/rules/shared"
	"go.uber.org/zap"
)

// DataProductConfigMrReviewHandler handles GitLab webhook requests
type DataProductConfigMrReviewHandler struct {
	gitlabClient *gitlab.Client
	ruleManager  shared.RuleManager
	config       *config.Config
}

// NewDataProductConfigMrReviewHandler creates a new webhook handler
func NewDataProductConfigMrReviewHandler(cfg *config.Config) *DataProductConfigMrReviewHandler {
	gitlabClient := gitlab.NewClientWithConfig(cfg)

	// Create rule manager for dataverse product config
	// Use the old client constructor for the rule manager since it doesn't need dry-run mode
	ruleManagerClient := gitlab.NewClient(cfg.GitLab)
	manager := rules.CreateDataverseRuleManager(ruleManagerClient)

	// Log security configuration
	logging.Info("Webhook security: %s", cfg.WebhookSecurityMode())
	if len(cfg.Webhook.AllowedIPs) > 0 {
		logging.Info("IP restrictions enabled: %v", cfg.Webhook.AllowedIPs)
	}

	// Log comments configuration
	logging.Info("MR Comments: %t (verbosity: %s)",
		cfg.Comments.EnableMRComments, cfg.Comments.CommentVerbosity)

	return &DataProductConfigMrReviewHandler{
		gitlabClient: gitlabClient,
		ruleManager:  manager,
		config:       cfg,
	}
}

// HandleWebhook processes GitLab webhook requests with security validation
func (h *DataProductConfigMrReviewHandler) HandleWebhook(c *fiber.Ctx) error {

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

	return h.handleMergeRequestEvent(c, payload)
}

// evaluateRules evaluates all rules and returns a decision with optimized error handling
func (h *DataProductConfigMrReviewHandler) evaluateRules(projectID, mrID int, mrInfo *gitlab.MRInfo) (*shared.RuleEvaluation, error) {
	// Fetch MR changes from GitLab API with timeout handling
	changes, err := h.gitlabClient.FetchMRChanges(projectID, mrID)
	if err != nil {
		logging.MRError(mrID, "Failed to fetch MR changes", err)
		// Return manual review decision if we can't fetch changes
		return &shared.RuleEvaluation{
			FinalDecision: shared.Decision{
				Type:   shared.ManualReview,
				Reason: "Could not fetch MR changes from GitLab API",
			},
			FileValidations: make(map[string]*shared.FileValidationSummary),
			ExecutionTime:   0,
		}, nil
	}

	// Create MR context for rule evaluation
	mrContext := &shared.MRContext{
		ProjectID: projectID,
		MRIID:     mrID,
		Changes:   changes,
		MRInfo:    mrInfo,
	}

	// Log rule evaluation start
	logging.MRInfo(mrID, "Starting rule evaluation", zap.Int("file_changes", len(changes)))

	// Evaluate all rules using the simple rule manager
	result := h.ruleManager.EvaluateAll(mrContext)

	// Log rule evaluation completion
	logging.MRInfo(mrID, "Rule evaluation completed",
		zap.String("decision", string(result.FinalDecision.Type)),
		zap.Int("files_evaluated", result.TotalFiles))
	return result, nil
}

// handleApprovalWithComments handles the approval process with meaningful comments and messages
func (h *DataProductConfigMrReviewHandler) handleApprovalWithComments(result *shared.RuleEvaluation, mrInfo *gitlab.MRInfo) error {
	messageBuilder := NewMessageBuilder(h.config)

	// Add detailed comment to MR if enabled
	if h.config.Comments.EnableMRComments {
		comment := messageBuilder.BuildApprovalComment(result, mrInfo)

		logging.MRInfo(mrInfo.MRIID, "Adding/updating approval comment")

		// Use smart comment handling (update existing or create new)
		if h.config.Comments.UpdateExistingComments {
			if err := h.gitlabClient.AddOrUpdateMRComment(mrInfo.ProjectID, mrInfo.MRIID, comment, "approval"); err != nil {
				logging.MRError(mrInfo.MRIID, "Failed to add/update comment", err)
				// Continue with approval even if comment fails - comment is nice-to-have
			} else {
				logging.MRInfo(mrInfo.MRIID, "Added/updated approval comment")
			}
		} else {
			// Legacy behavior: always create new comment
			if err := h.gitlabClient.AddMRComment(mrInfo.ProjectID, mrInfo.MRIID, comment); err != nil {
				logging.MRError(mrInfo.MRIID, "Failed to add comment", err)
				// Continue with approval even if comment fails - comment is nice-to-have
			} else {
				logging.MRInfo(mrInfo.MRIID, "Added approval comment")
			}
		}
	} else {
		logging.MRInfo(mrInfo.MRIID, "Skipping comment (comments disabled)")
	}

	// Approve the MR with message
	approvalMessage := messageBuilder.BuildApprovalMessage(result)
	logging.MRInfo(mrInfo.MRIID, "Approving MR with message", zap.String("message", approvalMessage))

	if err := h.gitlabClient.ApproveMRWithMessage(mrInfo.ProjectID, mrInfo.MRIID, approvalMessage); err != nil {
		// Try fallback to simple approval if message approval fails
		logging.MRWarn(mrInfo.MRIID, "Failed to approve with message, trying simple approval", zap.Error(err))
		if fallbackErr := h.gitlabClient.ApproveMR(mrInfo.ProjectID, mrInfo.MRIID); fallbackErr != nil {
			return fmt.Errorf("failed to approve MR (both with message and simple): %w", fallbackErr)
		}
		logging.MRInfo(mrInfo.MRIID, "Auto-approved (fallback approval)")
	} else {
		logging.MRInfo(mrInfo.MRIID, "Auto-approved", zap.String("message", approvalMessage))
	}

	return nil
}

// handleManualReviewWithComments handles manual review decisions with informational comments
func (h *DataProductConfigMrReviewHandler) handleManualReviewWithComments(result *shared.RuleEvaluation, mrInfo *gitlab.MRInfo) error {
	messageBuilder := NewMessageBuilder(h.config)

	// Add informational comment to MR if enabled
	if h.config.Comments.EnableMRComments {
		comment := messageBuilder.BuildManualReviewComment(result, mrInfo)

		logging.MRInfo(mrInfo.MRIID, "Adding/updating manual review comment")

		// Use smart comment handling (update existing or create new)
		if h.config.Comments.UpdateExistingComments {
			if err := h.gitlabClient.AddOrUpdateMRComment(mrInfo.ProjectID, mrInfo.MRIID, comment, "manual-review"); err != nil {
				logging.MRError(mrInfo.MRIID, "Failed to add/update manual review comment", err)
				// Continue without error - comment is nice-to-have
			} else {
				logging.MRInfo(mrInfo.MRIID, "Added/updated manual review comment")
			}
		} else {
			// Legacy behavior: always create new comment
			if err := h.gitlabClient.AddMRComment(mrInfo.ProjectID, mrInfo.MRIID, comment); err != nil {
				logging.MRError(mrInfo.MRIID, "Failed to add manual review comment", err)
				// Continue without error - comment is nice-to-have
			} else {
				logging.MRInfo(mrInfo.MRIID, "Added manual review comment")
			}
		}
	} else {
		logging.MRInfo(mrInfo.MRIID, "Skipping manual review comment (comments disabled)")
	}

	return nil
}

// handleMergeRequestEvent handles traditional MR events (immediate processing)
func (h *DataProductConfigMrReviewHandler) handleMergeRequestEvent(c *fiber.Ctx, payload map[string]interface{}) error {
	// Extract MR information
	mrInfo, err := gitlab.ExtractMRInfo(payload)
	if err != nil {
		logging.Error("Failed to extract MR info: %v", err)
		return c.Status(400).JSON(fiber.Map{
			"error": "Missing MR information: " + err.Error(),
		})
	}

	logging.MRInfo(mrInfo.MRIID, "Processing MR event",
		zap.Int("project_id", mrInfo.ProjectID),
		zap.String("author", mrInfo.Author),
		zap.String("state", mrInfo.State))

	// Skip rule evaluation if MR is not open
	if mrInfo.State != "opened" {
		logging.MRInfo(mrInfo.MRIID, "Skipping rule evaluation for non-open MR", 
			zap.String("state", mrInfo.State))
		
		return c.JSON(fiber.Map{
			"webhook_response": "processed",
			"event_type":       "merge_request",
			"decision":         "skipped",
			"reason":           fmt.Sprintf("MR state is '%s', only processing open MRs", mrInfo.State),
			"mr_approved":      false,
			"project_id":       mrInfo.ProjectID,
			"mr_iid":           mrInfo.MRIID,
		})
	}

	// Fast evaluation using rule manager
	result, err := h.evaluateRules(mrInfo.ProjectID, mrInfo.MRIID, mrInfo)
	if err != nil {
		logging.MRError(mrInfo.MRIID, "Rule evaluation failed", err)
		return c.Status(500).JSON(fiber.Map{
			"error": "Rule evaluation failed: " + err.Error(),
		})
	}

	// Log decision with execution time
	logging.MRInfo(mrInfo.MRIID, "Decision",
		zap.String("type", string(result.FinalDecision.Type)),
		zap.String("reason", result.FinalDecision.Reason),
		zap.Duration("execution_time", result.ExecutionTime))

	// Handle approval with comments if decision is to approve
	approved := false
	if result.FinalDecision.Type == shared.Approve {
		if err := h.handleApprovalWithComments(result, mrInfo); err != nil {
			logging.MRError(mrInfo.MRIID, "Failed to approve", err)
			return c.Status(500).JSON(fiber.Map{
				"error": "Failed to approve MR: " + err.Error(),
			})
		}
		approved = true
	} else {
		// Handle manual review with informational comments
		if err := h.handleManualReviewWithComments(result, mrInfo); err != nil {
			logging.MRError(mrInfo.MRIID, "Failed to add manual review comment", err)
			// Continue - comment failure shouldn't block the webhook response
		}
		logging.MRInfo(mrInfo.MRIID, "Manual review required", zap.String("reason", result.FinalDecision.Reason))
	}

	// Return structured response for GitLab webhook
	return c.JSON(fiber.Map{
		"webhook_response": "processed",
		"event_type":       "merge_request",
		"decision":         result.FinalDecision,
		"execution_time":   result.ExecutionTime.String(),
		"rules_evaluated":  result.TotalFiles,
		"mr_approved":      approved,
		"project_id":       mrInfo.ProjectID,
		"mr_iid":           mrInfo.MRIID,
	})
}

// validateWebhookPayload performs security validation on webhook payload
func (h *DataProductConfigMrReviewHandler) validateWebhookPayload(payload map[string]interface{}) error {
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

	// Validate project section
	project, ok := payload["project"]
	if !ok {
		return fmt.Errorf("missing project")
	}
	
	projectMap, ok := project.(map[string]interface{})
	if !ok {
		return fmt.Errorf("project must be an object")
	}

	// Validate project ID (must be positive integer)
	if projectID, exists := projectMap["id"]; exists {
		switch v := projectID.(type) {
		case float64:
			if v <= 0 {
				return fmt.Errorf("project ID must be positive")
			}
		case int:
			if v <= 0 {
				return fmt.Errorf("project ID must be positive")
			}
		case string:
			// Try to parse string as integer
			if id, err := strconv.Atoi(v); err != nil || id <= 0 {
				return fmt.Errorf("project ID must be a positive integer")
			}
		default:
			return fmt.Errorf("project ID must be a number")
		}
	} else {
		return fmt.Errorf("project ID is required")
	}

	// Validate MR IID (must be positive integer)
	if mrIID, exists := objectAttrsMap["iid"]; exists {
		switch v := mrIID.(type) {
		case float64:
			if v <= 0 {
				return fmt.Errorf("MR IID must be positive")
			}
		case int:
			if v <= 0 {
				return fmt.Errorf("MR IID must be positive")
			}
		case string:
			// Try to parse string as integer
			if id, err := strconv.Atoi(v); err != nil || id <= 0 {
				return fmt.Errorf("MR IID must be a positive integer")
			}
		default:
			return fmt.Errorf("MR IID must be a number")
		}
	} else {
		return fmt.Errorf("MR IID is required")
	}

	// Validate title (prevent XSS and excessive length)
	if title, exists := objectAttrsMap["title"]; exists {
		if titleStr, ok := title.(string); ok {
			if len(titleStr) > 255 {
				return fmt.Errorf("title too long (max 255 characters)")
			}
			// Basic XSS prevention - reject titles with HTML/script content
			if strings.Contains(strings.ToLower(titleStr), "<script") || 
			   strings.Contains(strings.ToLower(titleStr), "javascript:") {
				return fmt.Errorf("title contains potentially malicious content")
			}
		}
	}

	// Validate author field if present
	if user, exists := payload["user"]; exists {
		if userMap, ok := user.(map[string]interface{}); ok {
			if username, exists := userMap["username"]; exists {
				if usernameStr, ok := username.(string); ok {
					if len(usernameStr) > 100 {
						return fmt.Errorf("username too long (max 100 characters)")
					}
					// Basic validation - usernames should be alphanumeric with some special chars
					if !isValidUsername(usernameStr) {
						return fmt.Errorf("invalid username format")
					}
				}
			}
		}
	}

	// Validate branch names if present
	if sourceBranch, exists := objectAttrsMap["source_branch"]; exists {
		if branchStr, ok := sourceBranch.(string); ok {
			if len(branchStr) > 255 {
				return fmt.Errorf("source branch name too long (max 255 characters)")
			}
			if !isValidBranchName(branchStr) {
				return fmt.Errorf("invalid source branch name")
			}
		}
	}

	if targetBranch, exists := objectAttrsMap["target_branch"]; exists {
		if branchStr, ok := targetBranch.(string); ok {
			if len(branchStr) > 255 {
				return fmt.Errorf("target branch name too long (max 255 characters)")
			}
			if !isValidBranchName(branchStr) {
				return fmt.Errorf("invalid target branch name")
			}
		}
	}

	// Validate state field if present
	if state, exists := objectAttrsMap["state"]; exists {
		if stateStr, ok := state.(string); ok {
			validStates := []string{"opened", "closed", "merged"}
			isValid := false
			for _, validState := range validStates {
				if stateStr == validState {
					isValid = true
					break
				}
			}
			if !isValid {
				return fmt.Errorf("invalid MR state: %s", stateStr)
			}
		} else {
			return fmt.Errorf("state must be a string")
		}
	}

	return nil
}

// isValidUsername validates username format
func isValidUsername(username string) bool {
	if username == "" || len(username) > 100 {
		return false
	}
	// Allow alphanumeric, underscore, hyphen, dot
	for _, r := range username {
		if !((r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || 
			 (r >= '0' && r <= '9') || r == '_' || r == '-' || r == '.') {
			return false
		}
	}
	return true
}

// isValidBranchName validates Git branch name format
func isValidBranchName(branch string) bool {
	if branch == "" || len(branch) > 255 {
		return false
	}
	
	// Git branch names have specific rules
	// Cannot start with -, ., or contain ..
	if strings.HasPrefix(branch, "-") || strings.HasPrefix(branch, ".") || 
	   strings.Contains(branch, "..") {
		return false
	}
	
	// Cannot contain control characters, space, ~, ^, :, ?, *, [
	invalidChars := []string{" ", "~", "^", ":", "?", "*", "[", "]", "\\"}
	for _, invalid := range invalidChars {
		if strings.Contains(branch, invalid) {
			return false
		}
	}
	
	return true
}

// validateMRContext performs additional validation on the MR context
func (h *DataProductConfigMrReviewHandler) validateMRContext(mrCtx *shared.MRContext) error {
	if mrCtx == nil {
		return fmt.Errorf("MR context is nil")
	}
	
	if mrCtx.ProjectID <= 0 {
		return fmt.Errorf("invalid project ID: %d", mrCtx.ProjectID)
	}
	
	if mrCtx.MRIID <= 0 {
		return fmt.Errorf("invalid MR IID: %d", mrCtx.MRIID)
	}
	
	if mrCtx.MRInfo == nil {
		return fmt.Errorf("MR info is nil")
	}
	
	// Validate file paths in changes
	for i, change := range mrCtx.Changes {
		if err := h.validateFileChange(change); err != nil {
			return fmt.Errorf("invalid file change at index %d: %w", i, err)
		}
	}
	
	return nil
}

// validateFileChange validates individual file changes
func (h *DataProductConfigMrReviewHandler) validateFileChange(change gitlab.FileChange) error {
	// Validate file paths
	if change.NewPath != "" && !isValidFilePath(change.NewPath) {
		return fmt.Errorf("invalid new file path: %s", change.NewPath)
	}
	
	if change.OldPath != "" && !isValidFilePath(change.OldPath) {
		return fmt.Errorf("invalid old file path: %s", change.OldPath)
	}
	
	return nil
}

// isValidFilePath validates file path format
func isValidFilePath(path string) bool {
	if path == "" {
		return true // Empty paths are allowed for new/deleted files
	}
	
	if len(path) > 4096 {
		return false // Path too long
	}
	
	// Check for directory traversal attempts
	if strings.Contains(path, "..") || strings.HasPrefix(path, "/") {
		return false
	}
	
	// Check for control characters
	for _, r := range path {
		if r < 32 || r == 127 {
			return false
		}
	}
	
	return true
}
