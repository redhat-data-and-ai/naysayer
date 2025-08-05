package webhook

import (
	"github.com/gofiber/fiber/v2"
	"github.com/redhat-data-and-ai/naysayer/internal/config"
	"github.com/redhat-data-and-ai/naysayer/internal/rules"
)

// ManagementHandler handles management and introspection endpoints
type ManagementHandler struct {
	config *config.Config
}

// NewManagementHandler creates a new management handler
func NewManagementHandler(cfg *config.Config) *ManagementHandler {
	return &ManagementHandler{
		config: cfg,
	}
}

// HandleRules returns information about available rules
func (h *ManagementHandler) HandleRules(c *fiber.Ctx) error {
	allRules := rules.ListAvailableRules()
	enabledRules := rules.ListEnabledRules()
	
	response := fiber.Map{
		"total_rules":   len(allRules),
		"enabled_rules": len(enabledRules),
		"rules":         make([]fiber.Map, 0),
		"categories":    make(map[string]int),
	}
	
	// Collect rule information
	for _, ruleInfo := range allRules {
		rule := fiber.Map{
			"name":        ruleInfo.Name,
			"description": ruleInfo.Description,
			"version":     ruleInfo.Version,
			"enabled":     ruleInfo.Enabled,
			"category":    ruleInfo.Category,
		}
		response["rules"] = append(response["rules"].([]fiber.Map), rule)
		
		// Count by category
		if categories, ok := response["categories"].(map[string]int); ok {
			categories[ruleInfo.Category]++
		}
	}
	
	return c.JSON(response)
}

// HandleRulesEnabled returns only enabled rules
func (h *ManagementHandler) HandleRulesEnabled(c *fiber.Ctx) error {
	enabledRules := rules.ListEnabledRules()
	
	response := fiber.Map{
		"enabled_rules": len(enabledRules),
		"rules":         make([]fiber.Map, 0),
	}
	
	for _, ruleInfo := range enabledRules {
		rule := fiber.Map{
			"name":        ruleInfo.Name,
			"description": ruleInfo.Description,
			"version":     ruleInfo.Version,
			"category":    ruleInfo.Category,
		}
		response["rules"] = append(response["rules"].([]fiber.Map), rule)
	}
	
	return c.JSON(response)
}

// HandleRulesByCategory returns rules in a specific category
func (h *ManagementHandler) HandleRulesByCategory(c *fiber.Ctx) error {
	category := c.Params("category")
	if category == "" {
		return c.Status(400).JSON(fiber.Map{
			"error": "Category parameter is required",
		})
	}
	
	registry := rules.GetGlobalRegistry()
	categoryRules := registry.ListRulesByCategory(category)
	
	response := fiber.Map{
		"category":    category,
		"rule_count":  len(categoryRules),
		"rules":       make([]fiber.Map, 0),
	}
	
	for _, ruleInfo := range categoryRules {
		rule := fiber.Map{
			"name":        ruleInfo.Name,
			"description": ruleInfo.Description,
			"version":     ruleInfo.Version,
			"enabled":     ruleInfo.Enabled,
		}
		response["rules"] = append(response["rules"].([]fiber.Map), rule)
	}
	
	return c.JSON(response)
}

// HandleSystemInfo returns system information and capabilities
func (h *ManagementHandler) HandleSystemInfo(c *fiber.Ctx) error {
	allRules := rules.ListAvailableRules()
	enabledRules := rules.ListEnabledRules()
	
	// Count categories
	categories := make(map[string]int)
	for _, ruleInfo := range allRules {
		categories[ruleInfo.Category]++
	}
	
	return c.JSON(fiber.Map{
		"service":           "naysayer-webhook",
		"version":           "v1.0.0",
		"analysis_mode":     h.config.AnalysisMode(),
		"security_mode":     h.config.WebhookSecurityMode(),
		"gitlab_configured": h.config.HasGitLabToken(),
		"webhook_security":  h.config.Webhook.EnableVerification,
		"rule_system": fiber.Map{
			"total_rules":      len(allRules),
			"enabled_rules":    len(enabledRules),
			"total_categories": len(categories),
			"categories":       categories,
			"extensible":       true,
			"framework":        "registry-based",
		},
		"endpoints": []string{
			"/health",
			"/ready", 
			"/webhook",
			"/api/rules",
			"/api/rules/enabled",
			"/api/rules/category/:category",
			"/api/system",
		},
	})
}