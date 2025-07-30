package handler

import (
	"log"

	"github.com/gofiber/fiber/v2"
	"github.com/redhat-data-and-ai/naysayer/internal/analyzer"
	"github.com/redhat-data-and-ai/naysayer/internal/config"
	"github.com/redhat-data-and-ai/naysayer/internal/decision"
	"github.com/redhat-data-and-ai/naysayer/internal/gitlab"
)

// WebhookHandler handles GitLab webhook requests
type WebhookHandler struct {
	gitlabClient  *gitlab.Client
	analyzer      *analyzer.YAMLAnalyzer
	decisionMaker *decision.Maker
	config        *config.Config
}

// NewWebhookHandler creates a new webhook handler
func NewWebhookHandler(cfg *config.Config) *WebhookHandler {
	gitlabClient := gitlab.NewClient(cfg.GitLab)
	return &WebhookHandler{
		gitlabClient:  gitlabClient,
		analyzer:      analyzer.NewYAMLAnalyzer(gitlabClient),
		decisionMaker: decision.NewMaker(),
		config:        cfg,
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

	// Analyze file changes
	decision := h.analyzeFileChanges(mrInfo.ProjectID, mrInfo.MRIID)

	// Log decision
	log.Printf("Decision: auto_approve=%t, reason=%s", decision.AutoApprove, decision.Reason)

	return c.JSON(decision)
}

// analyzeFileChanges analyzes file changes and makes approval decision
func (h *WebhookHandler) analyzeFileChanges(projectID, mrIID int) decision.Decision {
	if !h.config.HasGitLabToken() {
		return h.decisionMaker.NoTokenDecision()
	}

	// Fetch MR changes from GitLab API
	changes, err := h.gitlabClient.FetchMRChanges(projectID, mrIID)
	if err != nil {
		log.Printf("Failed to fetch MR changes: %v", err)
		return h.decisionMaker.APIErrorDecision(err)
	}

	// Analyze warehouse changes using proper YAML parsing
	warehouseChanges, err := h.analyzer.AnalyzeChanges(projectID, mrIID, changes)
	if err != nil {
		log.Printf("YAML analysis failed: %v", err)
		return h.decisionMaker.AnalysisErrorDecision(err)
	}

	// Make decision based on changes
	return h.decisionMaker.Decide(warehouseChanges)
}