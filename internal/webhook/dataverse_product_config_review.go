package webhook

import (
	"fmt"
	"log"
	"strings"

	fiber "github.com/gofiber/fiber/v2"
	"github.com/redhat-data-and-ai/naysayer/internal/config"
	"github.com/redhat-data-and-ai/naysayer/internal/gitlab"
	"github.com/redhat-data-and-ai/naysayer/internal/rules"
	"github.com/redhat-data-and-ai/naysayer/internal/rules/shared"
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
	log.Printf("Webhook security: %s", cfg.WebhookSecurityMode())
	if len(cfg.Webhook.AllowedIPs) > 0 {
		log.Printf("IP restrictions enabled: %v", cfg.Webhook.AllowedIPs)
	}

	// Log comments configuration
	log.Printf("MR Comments: %t (verbosity: %s)", 
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
		log.Printf("Invalid content type: %s", contentType)
		return c.Status(400).JSON(fiber.Map{
			"error": "Content-Type must be application/json",
		})
	}

	// Parse webhook payload
	var payload map[string]interface{}
	if err := c.BodyParser(&payload); err != nil {
		log.Printf("Failed to parse payload: %v", err)
		return c.Status(400).JSON(fiber.Map{
			"error": "Invalid JSON payload",
		})
	}

	// Only support MR events
	eventType, ok := payload["object_kind"].(string)
	if !ok {
		log.Printf("Missing object_kind in payload")
		return c.Status(400).JSON(fiber.Map{
			"error": "Missing object_kind",
		})
	}

	if eventType != "merge_request" {
		log.Printf("Skipping unsupported event: %s", eventType)
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
		log.Printf("MR %d: Failed to fetch MR changes: %v", mrID, err)
		// Return manual review decision if we can't fetch changes
		return &shared.RuleEvaluation{
			FinalDecision: shared.Decision{
				Type:    shared.ManualReview,
				Reason:  "Could not fetch MR changes from GitLab API",
				Summary: "🚫 API error - manual review required",
				Details: fmt.Sprintf("GitLab API error: %v", err),
			},
			RuleResults:   []shared.RuleResult{},
			ExecutionTime: 0,
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
	log.Printf("MR %d: Starting rule evaluation with %d file changes", mrID, len(changes))

	// Evaluate all rules using the simple rule manager
	result := h.ruleManager.EvaluateAll(mrContext)
	
	// Log rule evaluation completion
	log.Printf("MR %d: Rule evaluation completed: %d rules evaluated, decision=%s", 
		mrID, len(result.RuleResults), result.FinalDecision.Type)
	
	return result, nil
}

// handleApprovalWithComments handles the approval process with meaningful comments and messages
func (h *DataProductConfigMrReviewHandler) handleApprovalWithComments(result *shared.RuleEvaluation, mrInfo *gitlab.MRInfo) error {
	messageBuilder := NewMessageBuilder(h.config)

	// Add detailed comment to MR if enabled
	if h.config.Comments.EnableMRComments {
		comment := messageBuilder.BuildApprovalComment(result, mrInfo)
		
		if err := h.gitlabClient.AddMRComment(mrInfo.ProjectID, mrInfo.MRIID, comment); err != nil {
			log.Printf("MR %d: Failed to add comment: %v", mrInfo.MRIID, err)
			// Continue with approval even if comment fails - comment is nice-to-have
		} else {
			log.Printf("MR %d: 📝 Added approval comment", mrInfo.MRIID)
		}
	} else {
		log.Printf("MR %d: Skipping comment (comments disabled)", mrInfo.MRIID)
	}

	// Approve with meaningful message
	approvalMessage := messageBuilder.BuildApprovalMessage(result)
	
	if err := h.gitlabClient.ApproveMRWithMessage(mrInfo.ProjectID, mrInfo.MRIID, approvalMessage); err != nil {
		// Try fallback to simple approval if message approval fails
		log.Printf("MR %d: Failed to approve with message, trying simple approval: %v", mrInfo.MRIID, err)
		if fallbackErr := h.gitlabClient.ApproveMR(mrInfo.ProjectID, mrInfo.MRIID); fallbackErr != nil {
			return fmt.Errorf("failed to approve MR (both with message and simple): %w", fallbackErr)
		}
		log.Printf("MR %d: ✅ Auto-approved (fallback approval)", mrInfo.MRIID)
	} else {
		log.Printf("MR %d: ✅ Auto-approved: %s", mrInfo.MRIID, approvalMessage)
	}

	return nil
}

// handleMergeRequestEvent handles traditional MR events (immediate processing)
func (h *DataProductConfigMrReviewHandler) handleMergeRequestEvent(c *fiber.Ctx, payload map[string]interface{}) error {
	// Extract MR information
	mrInfo, err := gitlab.ExtractMRInfo(payload)
	if err != nil {
		log.Printf("Failed to extract MR info: %v", err)
		return c.Status(400).JSON(fiber.Map{
			"error": "Missing MR information: " + err.Error(),
		})
	}

	log.Printf("MR %d: Processing MR event: Project=%d, Author=%s", mrInfo.MRIID, mrInfo.ProjectID, mrInfo.Author)

	// Fast evaluation using rule manager
	result, err := h.evaluateRules(mrInfo.ProjectID, mrInfo.MRIID, mrInfo)
	if err != nil {
		log.Printf("MR %d: Rule evaluation failed: %v", mrInfo.MRIID, err)
		return c.Status(500).JSON(fiber.Map{
			"error": "Rule evaluation failed: " + err.Error(),
		})
	}

	// Log decision with execution time
	log.Printf("MR %d: Decision: type=%s, reason=%s, execution_time=%v",
		mrInfo.MRIID, result.FinalDecision.Type, result.FinalDecision.Reason, result.ExecutionTime)

	// Handle approval with comments if decision is to approve
	approved := false
	if result.FinalDecision.Type == shared.Approve {
		if err := h.handleApprovalWithComments(result, mrInfo); err != nil {
			log.Printf("MR %d: Failed to approve: %v", mrInfo.MRIID, err)
			return c.Status(500).JSON(fiber.Map{
				"error": "Failed to approve MR: " + err.Error(),
			})
		}
		approved = true
	} else {
		log.Printf("MR %d: 🚫 Manual review required: %s", mrInfo.MRIID, result.FinalDecision.Reason)
	}

	// Return structured response for GitLab webhook
	return c.JSON(fiber.Map{
		"webhook_response": "processed",
		"event_type":       "merge_request",
		"decision":         result.FinalDecision,
		"execution_time":   result.ExecutionTime.String(),
		"rules_evaluated":  len(result.RuleResults),
		"mr_approved":      approved,
		"project_id":       mrInfo.ProjectID,
		"mr_iid":           mrInfo.MRIID,
	})
}




