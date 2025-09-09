package rules

import (
	"github.com/redhat-data-and-ai/naysayer/internal/config"
	"github.com/redhat-data-and-ai/naysayer/internal/gitlab"
	"github.com/redhat-data-and-ai/naysayer/internal/rules/shared"
)

// CreateDataverseRuleManager creates a rule manager with rules for dataverse product config
// This function now uses the extensible rule registry system
func CreateDataverseRuleManager(gitlabClient *gitlab.Client) shared.RuleManager {
	registry := GetGlobalRegistry()
	return registry.CreateDataverseRuleManager(gitlabClient)
}

// CreateCustomRuleManager creates a rule manager with specific rules
func CreateCustomRuleManager(gitlabClient *gitlab.Client, ruleNames []string) (shared.RuleManager, error) {
	registry := GetGlobalRegistry()
	return registry.CreateRuleManager(gitlabClient, ruleNames)
}

// LoadRuleConfigFromPath loads rule configuration from a file path
func LoadRuleConfigFromPath(configPath string) (*config.RuleConfig, error) {
	return config.LoadRuleConfig(configPath)
}

// NewSectionRuleManagerFromConfig creates a new section-based rule manager from config
func NewSectionRuleManagerFromConfig(ruleConfig *config.RuleConfig) shared.RuleManager {
	return NewSectionRuleManager(ruleConfig)
}

// CreateSectionBasedDataverseManager creates a section-aware manager for dataverse workflows
func CreateSectionBasedDataverseManager(client *gitlab.Client) shared.RuleManager {
	registry := GetGlobalRegistry()

	// Only use section-based manager - no fallbacks
	sectionManager, err := registry.CreateSectionBasedRuleManager(client, "rules.yaml")
	if err != nil {
		// Return a minimal manager that requires manual review for all files
		ruleConfig := &config.RuleConfig{
			Enabled:                 true,
			RequireFullCoverage:     false,
			ManualReviewOnUncovered: true,
			Files:                   []config.FileRuleConfig{},
		}
		return NewSectionRuleManager(ruleConfig)
	}

	return sectionManager
}

// ListAvailableRules returns information about all available rules
func ListAvailableRules() map[string]*RuleInfo {
	registry := GetGlobalRegistry()
	return registry.ListRules()
}

// ListEnabledRules returns information about enabled rules
func ListEnabledRules() map[string]*RuleInfo {
	registry := GetGlobalRegistry()
	return registry.ListEnabledRules()
}
