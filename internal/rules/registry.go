package rules

import (
	"fmt"

	"github.com/redhat-data-and-ai/naysayer/internal/config"
	"github.com/redhat-data-and-ai/naysayer/internal/gitlab"
	"github.com/redhat-data-and-ai/naysayer/internal/logging"
	"github.com/redhat-data-and-ai/naysayer/internal/rules/common"
	"github.com/redhat-data-and-ai/naysayer/internal/rules/shared"
	"github.com/redhat-data-and-ai/naysayer/internal/rules/warehouse"
)

// RuleFactory is a function that creates a rule instance
type RuleFactory func(client *gitlab.Client) shared.Rule

// RuleInfo contains metadata about a rule
type RuleInfo struct {
	Name        string      // Rule identifier
	Description string      // Human-readable description
	Version     string      // Rule version
	Factory     RuleFactory // Factory function to create the rule
	Enabled     bool        // Whether the rule is enabled by default
	Category    string      // Rule category (e.g., "warehouse", "source", "security")
}

// RuleRegistry manages available rules and their creation
type RuleRegistry struct {
	rules map[string]*RuleInfo
}

// NewRuleRegistry creates a new rule registry
func NewRuleRegistry() *RuleRegistry {
	registry := &RuleRegistry{
		rules: make(map[string]*RuleInfo),
	}

	// Register built-in rules
	registry.registerBuiltInRules()

	return registry
}

// registerBuiltInRules registers all built-in rules
func (r *RuleRegistry) registerBuiltInRules() {
	// Warehouse rule
	_ = r.RegisterRule(&RuleInfo{
		Name:        "warehouse_rule",
		Description: "Auto-approves MRs with only dataverse-safe files (warehouse/sourcebinding), requires manual review for warehouse increases",
		Version:     "1.0.0",
		Factory: func(client *gitlab.Client) shared.Rule {
			return warehouse.NewRule(client)
		},
		Enabled:  true,
		Category: "warehouse",
	})

	// Section-based rules

	_ = r.RegisterRule(&RuleInfo{
		Name:        "metadata_rule",
		Description: "Auto-approves documentation and metadata file changes",
		Version:     "1.0.0",
		Factory: func(client *gitlab.Client) shared.Rule {
			return common.NewMetadataRule()
		},
		Enabled:  true,
		Category: "auto_approval",
	})

	// Service account rule
	_ = r.RegisterRule(&RuleInfo{
		Name:        "service_account_rule",
		Description: "Auto-approves Astro service account files (**_astro_<env>_appuser.yaml/yml) when name field matches filename. Other service account files require manual review.",
		Version:     "1.0.0",
		Factory: func(client *gitlab.Client) shared.Rule {
			return NewServiceAccountRule(client)
		},
		Enabled:  true,
		Category: "service_account",
	})

}

// RegisterRule registers a new rule in the registry
func (r *RuleRegistry) RegisterRule(info *RuleInfo) error {
	if info.Name == "" {
		return fmt.Errorf("rule name cannot be empty")
	}

	if info.Factory == nil {
		return fmt.Errorf("rule factory cannot be nil")
	}

	if _, exists := r.rules[info.Name]; exists {
		return fmt.Errorf("rule '%s' is already registered", info.Name)
	}

	r.rules[info.Name] = info
	logging.Info("Registered rule: %s (category: %s, enabled: %t)", info.Name, info.Category, info.Enabled)

	return nil
}

// GetRule returns rule info by name
func (r *RuleRegistry) GetRule(name string) (*RuleInfo, bool) {
	rule, exists := r.rules[name]
	return rule, exists
}

// ListRules returns all registered rules
func (r *RuleRegistry) ListRules() map[string]*RuleInfo {
	// Return a copy to prevent external modification
	result := make(map[string]*RuleInfo)
	for name, info := range r.rules {
		result[name] = info
	}
	return result
}

// ListEnabledRules returns only enabled rules
func (r *RuleRegistry) ListEnabledRules() map[string]*RuleInfo {
	result := make(map[string]*RuleInfo)
	for name, info := range r.rules {
		if info.Enabled {
			result[name] = info
		}
	}
	return result
}

// ListRulesByCategory returns rules in a specific category
func (r *RuleRegistry) ListRulesByCategory(category string) map[string]*RuleInfo {
	result := make(map[string]*RuleInfo)
	for name, info := range r.rules {
		if info.Category == category {
			result[name] = info
		}
	}
	return result
}

// CreateRuleManager creates a rule manager with specified rules
func (r *RuleRegistry) CreateRuleManager(client *gitlab.Client, ruleNames []string) (shared.RuleManager, error) {
	// Load default rule configuration for section-based validation
	ruleConfig, err := config.LoadRuleConfig("rules.yaml")
	if err != nil {
		logging.Warn("Failed to load rule config, using minimal configuration: %v", err)
		// Create minimal config for fallback
		ruleConfig = &config.RuleConfig{
			Enabled:                 true,
			RequireFullCoverage:     false,
			ManualReviewOnUncovered: true,
			Files:                   []config.FileRuleConfig{},
		}
	}

	manager := NewSectionRuleManager(ruleConfig)

	// If no specific rules requested, use all enabled rules
	if len(ruleNames) == 0 {
		for _, info := range r.ListEnabledRules() {
			rule := info.Factory(client)
			manager.AddRule(rule)
			logging.Info("Added enabled rule: %s", info.Name)
		}
	} else {
		// Add only specified rules from the list
		for _, ruleName := range ruleNames {
			info, ok := r.rules[ruleName]
			if !ok {
				return nil, fmt.Errorf("rule not found: %s", ruleName)
			}
			rule := info.Factory(client)
			manager.AddRule(rule)
			logging.Info("Added requested rule: %s", info.Name)
		}
	}

	return manager, nil
}

// CreateDataverseRuleManager creates a rule manager specifically for dataverse workflows
func (r *RuleRegistry) CreateDataverseRuleManager(client *gitlab.Client) shared.RuleManager {
	// Include warehouse rule and auto-approval rules for complete dataverse workflow support
	dataverseRules := []string{
		"warehouse_rule",
		// Note: service_account_comment_rule is registered but disabled in production
		// because it requires GitLab API access to compare old vs new file content
		// "service_account_comment_rule",
		// TODO: Add back when implemented:
		// "migrations_rule", 
		// "naming_conventions_rule",
	}

	manager, err := r.CreateRuleManager(client, dataverseRules)
	if err != nil {
		logging.Error("Error creating dataverse rule manager: %v", err)
		// Fallback to empty section manager
		ruleConfig := &config.RuleConfig{
			Enabled:                 true,
			RequireFullCoverage:     false,
			ManualReviewOnUncovered: true,
			Files:                   []config.FileRuleConfig{},
		}
		return NewSectionRuleManager(ruleConfig)
	}

	return manager
}

// CreateSectionBasedRuleManager creates a section-aware rule manager
func (r *RuleRegistry) CreateSectionBasedRuleManager(client *gitlab.Client, ruleConfigPath string) (shared.RuleManager, error) {
	// Load rule configuration
	ruleConfig, err := config.LoadRuleConfig(ruleConfigPath)
	if err != nil {
		logging.Warn("Failed to load rule config, falling back to dataverse manager: %v", err)
		return r.CreateDataverseRuleManager(client), nil
	}

	// If section-based validation is disabled, use dataverse manager
	if !ruleConfig.Enabled {
		logging.Info("Section-based validation disabled, using dataverse rule manager")
		return r.CreateDataverseRuleManager(client), nil
	}

	// Create section-based manager
	sectionManager := NewSectionRuleManager(ruleConfig)

	// Add all enabled rules to the section manager
	for _, info := range r.ListEnabledRules() {
		rule := info.Factory(client)
		sectionManager.AddRule(rule)
		logging.Info("Added rule to section manager: %s", info.Name)
	}

	logging.Info("Created section-based rule manager with %d file configurations", len(ruleConfig.Files))
	return sectionManager, nil
}

// Global registry instance
var globalRegistry *RuleRegistry

// GetGlobalRegistry returns the global rule registry
func GetGlobalRegistry() *RuleRegistry {
	if globalRegistry == nil {
		globalRegistry = NewRuleRegistry()
	}
	return globalRegistry
}

// RegisterGlobalRule registers a rule in the global registry
func RegisterGlobalRule(info *RuleInfo) error {
	return GetGlobalRegistry().RegisterRule(info)
}
