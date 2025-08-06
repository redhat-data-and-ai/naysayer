package rules

import (
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

// CreateRuleManagerByCategory creates a rule manager with rules from a specific category
func CreateRuleManagerByCategory(gitlabClient *gitlab.Client, category string) shared.RuleManager {
	registry := GetGlobalRegistry()
	manager := NewSimpleRuleManager()

	rules := registry.ListRulesByCategory(category)
	for _, info := range rules {
		if info.Enabled {
			rule := info.Factory(gitlabClient)
			manager.AddRule(rule)
		}
	}

	return manager
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
