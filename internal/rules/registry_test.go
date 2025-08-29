package rules

import (
	"testing"

	"github.com/redhat-data-and-ai/naysayer/internal/gitlab"
	"github.com/redhat-data-and-ai/naysayer/internal/rules/shared"
	"github.com/stretchr/testify/assert"
)

// MockRule is a simple mock rule for testing
type MockRule struct {
	name string
}

func (r *MockRule) Name() string {
	return r.name
}

func (r *MockRule) Description() string {
	return "Mock rule for testing"
}

func (r *MockRule) GetCoveredLines(filePath string, fileContent string) []shared.LineRange {
	return nil
}

func (r *MockRule) ValidateLines(filePath string, fileContent string, lineRanges []shared.LineRange) (shared.DecisionType, string) {
	return shared.Approve, "Mock validation"
}

func TestNewRuleRegistry(t *testing.T) {
	registry := NewRuleRegistry()

	assert.NotNil(t, registry)
	assert.NotNil(t, registry.rules)

	// Should have built-in rules registered
	rules := registry.ListRules()
	assert.Greater(t, len(rules), 0, "Should have built-in rules registered")

	// Check that specific built-in rules exist
	warehouseRule, exists := registry.GetRule("warehouse_rule")
	assert.True(t, exists, "Warehouse rule should be registered")
	assert.Equal(t, "warehouse_rule", warehouseRule.Name)
	assert.Equal(t, "warehouse", warehouseRule.Category)
	assert.True(t, warehouseRule.Enabled)
}

func TestRuleRegistry_RegisterRule(t *testing.T) {
	registry := NewRuleRegistry()

	tests := []struct {
		name          string
		ruleInfo      *RuleInfo
		expectedError string
	}{
		{
			name: "valid rule registration",
			ruleInfo: &RuleInfo{
				Name:        "test_rule",
				Description: "Test rule for unit testing",
				Version:     "1.0.0",
				Factory:     func(client *gitlab.Client) shared.Rule { return &MockRule{name: "test_rule"} },
				Enabled:     true,
				Category:    "test",
			},
			expectedError: "",
		},
		{
			name: "empty rule name",
			ruleInfo: &RuleInfo{
				Name:        "",
				Description: "Test rule with empty name",
				Factory:     func(client *gitlab.Client) shared.Rule { return &MockRule{} },
			},
			expectedError: "rule name cannot be empty",
		},
		{
			name: "nil factory",
			ruleInfo: &RuleInfo{
				Name:        "nil_factory_rule",
				Description: "Test rule with nil factory",
				Factory:     nil,
			},
			expectedError: "rule factory cannot be nil",
		},
		{
			name: "duplicate rule name",
			ruleInfo: &RuleInfo{
				Name:        "warehouse_rule", // Already registered in built-in rules
				Description: "Duplicate warehouse rule",
				Factory:     func(client *gitlab.Client) shared.Rule { return &MockRule{name: "duplicate"} },
			},
			expectedError: "rule 'warehouse_rule' is already registered",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := registry.RegisterRule(tt.ruleInfo)

			if tt.expectedError != "" {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedError)
			} else {
				assert.NoError(t, err)

				// Verify rule was registered
				registeredRule, exists := registry.GetRule(tt.ruleInfo.Name)
				assert.True(t, exists)
				assert.Equal(t, tt.ruleInfo.Name, registeredRule.Name)
				assert.Equal(t, tt.ruleInfo.Description, registeredRule.Description)
				assert.Equal(t, tt.ruleInfo.Version, registeredRule.Version)
				assert.Equal(t, tt.ruleInfo.Enabled, registeredRule.Enabled)
				assert.Equal(t, tt.ruleInfo.Category, registeredRule.Category)
				assert.NotNil(t, registeredRule.Factory)
			}
		})
	}
}

func TestRuleRegistry_GetRule(t *testing.T) {
	registry := NewRuleRegistry()

	// Test getting existing rule
	rule, exists := registry.GetRule("warehouse_rule")
	assert.True(t, exists)
	assert.NotNil(t, rule)
	assert.Equal(t, "warehouse_rule", rule.Name)

	// Test getting non-existent rule
	rule, exists = registry.GetRule("non_existent_rule")
	assert.False(t, exists)
	assert.Nil(t, rule)
}

func TestRuleRegistry_ListRules(t *testing.T) {
	registry := NewRuleRegistry()

	// Add a test rule
	testRule := &RuleInfo{
		Name:        "test_list_rule",
		Description: "Test rule for list functionality",
		Version:     "1.0.0",
		Factory:     func(client *gitlab.Client) shared.Rule { return &MockRule{name: "test_list_rule"} },
		Enabled:     false,
		Category:    "test",
	}
	err := registry.RegisterRule(testRule)
	assert.NoError(t, err)

	rules := registry.ListRules()
	assert.Greater(t, len(rules), 1, "Should have at least built-in rules plus test rule")

	// Verify built-in rules are present
	assert.Contains(t, rules, "warehouse_rule")
	assert.Contains(t, rules, "test_list_rule")

	// Verify returned map is a copy (modification doesn't affect registry)
	delete(rules, "warehouse_rule")
	originalRules := registry.ListRules()
	assert.Contains(t, originalRules, "warehouse_rule", "Original registry should not be affected")
}

func TestRuleRegistry_ListEnabledRules(t *testing.T) {
	registry := NewRuleRegistry()

	// Add enabled and disabled test rules
	enabledRule := &RuleInfo{
		Name:        "enabled_test_rule",
		Description: "Enabled test rule",
		Factory:     func(client *gitlab.Client) shared.Rule { return &MockRule{name: "enabled_test_rule"} },
		Enabled:     true,
		Category:    "test",
	}
	disabledRule := &RuleInfo{
		Name:        "disabled_test_rule",
		Description: "Disabled test rule",
		Factory:     func(client *gitlab.Client) shared.Rule { return &MockRule{name: "disabled_test_rule"} },
		Enabled:     false,
		Category:    "test",
	}

	err := registry.RegisterRule(enabledRule)
	assert.NoError(t, err)
	err = registry.RegisterRule(disabledRule)
	assert.NoError(t, err)

	enabledRules := registry.ListEnabledRules()

	// Should contain built-in enabled rules plus our enabled test rule
	assert.Contains(t, enabledRules, "warehouse_rule")
	assert.Contains(t, enabledRules, "enabled_test_rule")

	// Should NOT contain disabled rule
	assert.NotContains(t, enabledRules, "disabled_test_rule")

	// Verify all returned rules are enabled
	for _, rule := range enabledRules {
		assert.True(t, rule.Enabled, "All returned rules should be enabled")
	}
}

func TestRuleRegistry_ListRulesByCategory(t *testing.T) {
	registry := NewRuleRegistry()

	// Add test rules in different categories
	warehouseTestRule := &RuleInfo{
		Name:        "warehouse_test_rule",
		Description: "Test warehouse rule",
		Factory:     func(client *gitlab.Client) shared.Rule { return &MockRule{name: "warehouse_test_rule"} },
		Category:    "warehouse",
	}
	sourceTestRule := &RuleInfo{
		Name:        "source_test_rule",
		Description: "Test source rule",
		Factory:     func(client *gitlab.Client) shared.Rule { return &MockRule{name: "source_test_rule"} },
		Category:    "source",
	}
	securityRule := &RuleInfo{
		Name:        "security_rule",
		Description: "Test security rule",
		Factory:     func(client *gitlab.Client) shared.Rule { return &MockRule{name: "security_rule"} },
		Category:    "security",
	}

	err := registry.RegisterRule(warehouseTestRule)
	assert.NoError(t, err)
	err = registry.RegisterRule(sourceTestRule)
	assert.NoError(t, err)
	err = registry.RegisterRule(securityRule)
	assert.NoError(t, err)

	// Test warehouse category
	warehouseRules := registry.ListRulesByCategory("warehouse")
	assert.Contains(t, warehouseRules, "warehouse_rule")      // Built-in
	assert.Contains(t, warehouseRules, "warehouse_test_rule") // Our test rule
	assert.NotContains(t, warehouseRules, "source_binding_rule")
	assert.NotContains(t, warehouseRules, "security_rule")

	// Test source category
	sourceRules := registry.ListRulesByCategory("source")
	assert.Contains(t, sourceRules, "source_test_rule") // Our test rule
	assert.NotContains(t, sourceRules, "warehouse_rule")
	assert.NotContains(t, sourceRules, "security_rule")

	// Test security category
	securityRules := registry.ListRulesByCategory("security")
	assert.Contains(t, securityRules, "security_rule")
	assert.NotContains(t, securityRules, "warehouse_rule")
	assert.NotContains(t, securityRules, "source_binding_rule")

	// Test non-existent category
	nonExistentRules := registry.ListRulesByCategory("non_existent")
	assert.Empty(t, nonExistentRules)
}

func TestRuleRegistry_CreateRuleManager_DefaultEnabledRules(t *testing.T) {
	registry := NewRuleRegistry()
	client := &gitlab.Client{}

	// Create manager with default rules (empty slice means use all enabled)
	manager, err := registry.CreateRuleManager(client, []string{})
	assert.NoError(t, err)
	assert.NotNil(t, manager)

	// Verify it's a valid RuleManager
	assert.NotNil(t, manager)
}

func TestRuleRegistry_CreateRuleManager_SpecificRules(t *testing.T) {
	registry := NewRuleRegistry()
	client := &gitlab.Client{}

	// Add a test rule
	testRule := &RuleInfo{
		Name:        "specific_test_rule",
		Description: "Rule for specific test",
		Factory:     func(client *gitlab.Client) shared.Rule { return &MockRule{name: "specific_test_rule"} },
		Enabled:     false, // Even disabled rules can be specifically requested
		Category:    "test",
	}
	err := registry.RegisterRule(testRule)
	assert.NoError(t, err)

	// Create manager with specific rules
	requestedRules := []string{"warehouse_rule", "specific_test_rule"}
	manager, err := registry.CreateRuleManager(client, requestedRules)
	assert.NoError(t, err)
	assert.NotNil(t, manager)

	// Test with non-existent rule
	invalidRules := []string{"warehouse_rule", "non_existent_rule"}
	manager, err = registry.CreateRuleManager(client, invalidRules)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "rule not found: non_existent_rule")
	assert.Nil(t, manager)
}

func TestRuleRegistry_CreateDataverseRuleManager(t *testing.T) {
	registry := NewRuleRegistry()
	client := &gitlab.Client{}

	manager := registry.CreateDataverseRuleManager(client)
	assert.NotNil(t, manager)

	// Should be a valid RuleManager
	assert.NotNil(t, manager)
}

func TestRuleRegistry_CreateDataverseRuleManager_WithMissingRule(t *testing.T) {
	// Create a fresh registry and manually register only some rules
	registry := &RuleRegistry{
		rules: make(map[string]*RuleInfo),
	}

	// Only register warehouse rule, not service account rule
	warehouseRule := &RuleInfo{
		Name:        "warehouse_rule",
		Description: "Warehouse rule",
		Factory:     func(client *gitlab.Client) shared.Rule { return &MockRule{name: "warehouse_rule"} },
		Enabled:     true,
		Category:    "warehouse",
	}
	err := registry.RegisterRule(warehouseRule)
	assert.NoError(t, err)

	client := &gitlab.Client{}

	// Should return empty manager when some dataverse rules are missing (fallback behavior)
	manager := registry.CreateDataverseRuleManager(client)
	assert.NotNil(t, manager)

	// Should fallback to valid manager when rules are missing
	assert.NotNil(t, manager)
}

func TestGlobalRegistry(t *testing.T) {
	// Reset global registry for test isolation
	globalRegistry = nil

	registry1 := GetGlobalRegistry()
	assert.NotNil(t, registry1)

	registry2 := GetGlobalRegistry()
	assert.NotNil(t, registry2)

	// Should be the same instance (singleton)
	assert.Same(t, registry1, registry2)

	// Test global rule registration
	testRule := &RuleInfo{
		Name:        "global_test_rule",
		Description: "Test rule for global registry",
		Factory:     func(client *gitlab.Client) shared.Rule { return &MockRule{name: "global_test_rule"} },
		Category:    "test",
	}

	err := RegisterGlobalRule(testRule)
	assert.NoError(t, err)

	// Verify rule was registered globally
	rule, exists := GetGlobalRegistry().GetRule("global_test_rule")
	assert.True(t, exists)
	assert.Equal(t, "global_test_rule", rule.Name)
}

func TestRuleFactory_Functionality(t *testing.T) {
	registry := NewRuleRegistry()
	client := &gitlab.Client{}

	// Test that factory functions actually work
	warehouseRuleInfo, exists := registry.GetRule("warehouse_rule")
	assert.True(t, exists)
	assert.NotNil(t, warehouseRuleInfo.Factory)

	// Create rule using factory
	rule := warehouseRuleInfo.Factory(client)
	assert.NotNil(t, rule)
	assert.Equal(t, "warehouse_rule", rule.Name())
}

func TestRuleRegistry_EdgeCases(t *testing.T) {
	registry := NewRuleRegistry()

	// Test empty category search
	rules := registry.ListRulesByCategory("")
	assert.Empty(t, rules)

	// Test case sensitivity
	rules = registry.ListRulesByCategory("Warehouse") // Different case
	assert.Empty(t, rules, "Category search should be case sensitive")

	// Test rule with empty category
	emptyCategoryRule := &RuleInfo{
		Name:        "empty_category_rule",
		Description: "Rule with empty category",
		Factory:     func(client *gitlab.Client) shared.Rule { return &MockRule{name: "empty_category_rule"} },
		Category:    "",
	}
	err := registry.RegisterRule(emptyCategoryRule)
	assert.NoError(t, err)

	emptyRules := registry.ListRulesByCategory("")
	assert.Contains(t, emptyRules, "empty_category_rule")
}

func TestRuleInfo_Completeness(t *testing.T) {
	registry := NewRuleRegistry()

	// Verify all built-in rules have complete information
	rules := registry.ListRules()
	for name, rule := range rules {
		assert.NotEmpty(t, rule.Name, "Rule %s should have a name", name)
		assert.NotEmpty(t, rule.Description, "Rule %s should have a description", name)
		assert.NotEmpty(t, rule.Version, "Rule %s should have a version", name)
		assert.NotNil(t, rule.Factory, "Rule %s should have a factory", name)
		assert.NotEmpty(t, rule.Category, "Rule %s should have a category", name)
	}
}
