package handler

import (
	"context"
	"log"

	"github.com/gofiber/fiber/v2"
	"github.com/redhat-data-and-ai/naysayer/internal/config"
	"github.com/redhat-data-and-ai/naysayer/internal/gitlab"
	"github.com/redhat-data-and-ai/naysayer/internal/rules"
)

// WebhookHandler handles GitLab webhook requests
type WebhookHandler struct {
	gitlabClient *gitlab.Client
	ruleEngine   rules.RuleEngine
	config       *config.Config
}

// NewWebhookHandler creates a new webhook handler
func NewWebhookHandler(cfg *config.Config) *WebhookHandler {
	gitlabClient := gitlab.NewClient(cfg.GitLab)
	
	// Create rule engine and register rules
	engine := rules.NewUnanimousRuleEngine()
	
	// Register built-in rules
	warehouseRule := rules.NewWarehouseRule(gitlabClient)
	securityRule := rules.NewSecurityRule()
	
	engine.RegisterRule(warehouseRule)
	engine.RegisterRule(securityRule)
	
	return &WebhookHandler{
		gitlabClient: gitlabClient,
		ruleEngine:   engine,
		config:       cfg,
	}
}

// HandleWebhook processes GitLab webhook requests
func (h *WebhookHandler) HandleWebhook(c *fiber.Ctx) error {
	// Parse webhook payload
	var payload map[string]interface{}
	if err := c.BodyParser(&payload); err != nil {
		log.Printf("Failed to parse payload: %v", err)
		return c.Status(400).JSON(fiber.Map{
			"error": "Invalid JSON payload",
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

	log.Printf("Processing MR: Project=%d, MR=%d", mrInfo.ProjectID, mrInfo.MRIID)

	// Evaluate using rule engine
	result, err := h.evaluateRules(mrInfo.ProjectID, mrInfo.MRIID, mrInfo)
	if err != nil {
		log.Printf("Rule evaluation failed: %v", err)
		return c.Status(500).JSON(fiber.Map{
			"error": "Rule evaluation failed: " + err.Error(),
		})
	}

	// Log decision
	log.Printf("Decision: auto_approve=%t, reason=%s", result.FinalDecision.AutoApprove, result.FinalDecision.Reason)

	return c.JSON(result)
}

// evaluateRules evaluates all rules and returns unanimous decision
func (h *WebhookHandler) evaluateRules(projectID, mrIID int, mrInfo *gitlab.MRInfo) (*rules.UnanimousResult, error) {
	// Fetch MR changes from GitLab API
	changes, err := h.gitlabClient.FetchMRChanges(projectID, mrIID)
	if err != nil {
		log.Printf("Failed to fetch MR changes: %v", err)
		changes = []gitlab.FileChange{} // Continue with empty changes
	}

	// Create MR context for rule evaluation
	mrContext := &rules.MRContext{
		ProjectID: projectID,
		MRIID:     mrIID,
		Changes:   changes,
		MRInfo:    mrInfo,
	}

	// Evaluate all rules
	return h.ruleEngine.EvaluateAll(context.Background(), mrContext)
}