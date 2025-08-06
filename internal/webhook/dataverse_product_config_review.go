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

// WebhookHandler handles GitLab webhook requests
type WebhookHandler struct {
	gitlabClient *gitlab.Client
	ruleManager  shared.RuleManager
	config       *config.Config
}

// NewWebhookHandler creates a new webhook handler
func NewWebhookHandler(cfg *config.Config) *WebhookHandler {
	gitlabClient := gitlab.NewClient(cfg.GitLab)

	// Create rule manager for dataverse product config
	manager := rules.CreateDataverseRuleManager(gitlabClient)

	// Log security configuration
	log.Printf("Webhook security: %s", cfg.WebhookSecurityMode())
	if len(cfg.Webhook.AllowedIPs) > 0 {
		log.Printf("IP restrictions enabled: %v", cfg.Webhook.AllowedIPs)
	}

	return &WebhookHandler{
		gitlabClient: gitlabClient,
		ruleManager:  manager,
		config:       cfg,
	}
}

// HandleWebhook processes GitLab webhook requests with security validation
func (h *WebhookHandler) HandleWebhook(c *fiber.Ctx) error {

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

	// Fast event type filtering - only process merge request events
	eventType, ok := payload["object_kind"].(string)
	if !ok || eventType != "merge_request" {
		log.Printf("Skipping non-MR event: %s", eventType)
		return c.JSON(fiber.Map{
			"message": "Event processed",
			"action":  "skipped",
			"reason":  "Not a merge request event",
		})
	}

	// Extract MR information
	mrInfo, err := gitlab.ExtractMRInfo(payload)
	if err != nil {
		log.Printf("Failed to extract MR info: %v", err)
		return c.Status(400).JSON(fiber.Map{
			"error": "Missing MR information: " + err.Error(),
		})
	}

	log.Printf("Processing MR: Project=%d, MR=%d, Author=%s", mrInfo.ProjectID, mrInfo.MRIID, mrInfo.Author)

	// Fast evaluation using rule manager
	result, err := h.evaluateRules(mrInfo.ProjectID, mrInfo.MRIID, mrInfo)
	if err != nil {
		log.Printf("Rule evaluation failed: %v", err)
		return c.Status(500).JSON(fiber.Map{
			"error": "Rule evaluation failed: " + err.Error(),
		})
	}

	// Log decision with execution time
	log.Printf("Decision: type=%s, reason=%s, execution_time=%v",
		result.FinalDecision.Type, result.FinalDecision.Reason, result.ExecutionTime)

	// Return structured response for GitLab webhook
	return c.JSON(fiber.Map{
		"webhook_response": "processed",
		"decision":         result.FinalDecision,
		"execution_time":   result.ExecutionTime.String(),
		"rules_evaluated":  len(result.RuleResults),
	})
}

// evaluateRules evaluates all rules and returns a decision with optimized error handling
func (h *WebhookHandler) evaluateRules(projectID, mrIID int, mrInfo *gitlab.MRInfo) (*shared.RuleEvaluation, error) {
	// Fetch MR changes from GitLab API with timeout handling
	changes, err := h.gitlabClient.FetchMRChanges(projectID, mrIID)
	if err != nil {
		log.Printf("Failed to fetch MR changes: %v", err)
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
		MRIID:     mrIID,
		Changes:   changes,
		MRInfo:    mrInfo,
	}

	// Evaluate all rules using the simple rule manager
	result := h.ruleManager.EvaluateAll(mrContext)
	return result, nil
}
