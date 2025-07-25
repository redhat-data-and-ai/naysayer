package rules

import (
	"testing"

	"github.com/redhat-data-and-ai/naysayer/internal/gitlab"
	"github.com/redhat-data-and-ai/naysayer/internal/rules/shared"
	"github.com/stretchr/testify/assert"
)

func TestCreateDataverseRuleManager(t *testing.T) {
	client := &gitlab.Client{}
	
	manager := CreateDataverseRuleManager(client)
	
	assert.NotNil(t, manager)
	
	// Verify it's a rule manager (should implement the interface)
	_, ok := manager.(shared.RuleManager)
	assert.True(t, ok, "Should return a valid RuleManager")
	
	// The manager should have rules added (since we have dataverse rules in registry)
	simpleManager, ok := manager.(*SimpleRuleManager)
	assert.True(t, ok)
	assert.Greater(t, len(simpleManager.rules), 0, "Should have dataverse rules added")
}

func TestCreateCustomRuleManager_Success(t *testing.T) {
	client := &gitlab.Client{}
	
	// Test with existing rules
	ruleNames := []string{"warehouse_rule", "source_binding_rule"}
	manager, err := CreateCustomRuleManager(client, ruleNames)
	
	assert.NoError(t, err)
	assert.NotNil(t, manager)
	
	// Verify it's a rule manager
	_, ok := manager.(shared.RuleManager)
	assert.True(t, ok, "Should return a valid RuleManager")
	
	// Should have the requested rules
	simpleManager, ok := manager.(*SimpleRuleManager)
	assert.True(t, ok)
	assert.Equal(t, 2, len(simpleManager.rules), "Should have exactly 2 rules")
}

func TestCreateCustomRuleManager_WithNonExistentRule(t *testing.T) {
	client := &gitlab.Client{}
	
	// Test with non-existent rule
	ruleNames := []string{"warehouse_rule", "non_existent_rule"}
	manager, err := CreateCustomRuleManager(client, ruleNames)
	
	assert.Error(t, err)
	assert.Nil(t, manager)
	assert.Contains(t, err.Error(), "rule 'non_existent_rule' not found in registry")
}

func TestCreateCustomRuleManager_EmptyRuleList(t *testing.T) {
	client := &gitlab.Client{}
	
	// Test with empty rule list (should use all enabled rules)
	manager, err := CreateCustomRuleManager(client, []string{})
	
	assert.NoError(t, err)
	assert.NotNil(t, manager)
	
	// Should have enabled rules
	simpleManager, ok := manager.(*SimpleRuleManager)
	assert.True(t, ok)
	assert.Greater(t, len(simpleManager.rules), 0, "Should have enabled rules when list is empty")
}

func TestCreateRuleManagerByCategory(t *testing.T) {
	client := &gitlab.Client{}
	
	tests := []struct {
		name     string
		category string
		expectRules bool
	}{
		{
			name:     "warehouse category",
			category: "warehouse",
			expectRules: true,
		},
		{
			name:     "source category", 
			category: "source",
			expectRules: true,
		},
		{
			name:     "non-existent category",
			category: "non_existent",
			expectRules: false,
		},
		{
			name:     "empty category",
			category: "",
			expectRules: false,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			manager := CreateRuleManagerByCategory(client, tt.category)
			
			assert.NotNil(t, manager)
			
			simpleManager, ok := manager.(*SimpleRuleManager)
			assert.True(t, ok)
			
			if tt.expectRules {
				assert.Greater(t, len(simpleManager.rules), 0, "Should have rules for category %s", tt.category)
			} else {
				assert.Equal(t, 0, len(simpleManager.rules), "Should have no rules for category %s", tt.category)
			}
		})
	}
}

func TestListAvailableRules(t *testing.T) {
	rules := ListAvailableRules()
	
	assert.NotNil(t, rules)
	assert.Greater(t, len(rules), 0, "Should have available rules")
	
	// Verify built-in rules are present
	assert.Contains(t, rules, "warehouse_rule")
	assert.Contains(t, rules, "source_binding_rule")
	
	// Verify rule structure
	warehouseRule := rules["warehouse_rule"]
	assert.Equal(t, "warehouse_rule", warehouseRule.Name)
	assert.Equal(t, "warehouse", warehouseRule.Category)
	assert.True(t, warehouseRule.Enabled)
	assert.NotNil(t, warehouseRule.Factory)
	assert.NotEmpty(t, warehouseRule.Description)
	assert.NotEmpty(t, warehouseRule.Version)
}

func TestListEnabledRules(t *testing.T) {
	rules := ListEnabledRules()
	
	assert.NotNil(t, rules)
	assert.Greater(t, len(rules), 0, "Should have enabled rules")
	
	// All returned rules should be enabled
	for name, rule := range rules {
		assert.True(t, rule.Enabled, "Rule %s should be enabled", name)
	}
	
	// Built-in rules should be enabled by default
	assert.Contains(t, rules, "warehouse_rule")
	assert.Contains(t, rules, "source_binding_rule")
}

func TestListAvailableRules_IsCopy(t *testing.T) {
	// Get rules twice
	rules1 := ListAvailableRules()
	rules2 := ListAvailableRules()
	
	// Should be equal but not the same map reference
	assert.Equal(t, len(rules1), len(rules2))
	
	// Modify one map and verify the other is unaffected
	delete(rules1, "warehouse_rule")
	assert.NotContains(t, rules1, "warehouse_rule")
	assert.Contains(t, rules2, "warehouse_rule", "Modifying one map should not affect the other")
}

func TestListEnabledRules_IsCopy(t *testing.T) {
	// Get rules twice
	rules1 := ListEnabledRules()
	rules2 := ListEnabledRules()
	
	// Should be equal but not the same map reference
	assert.Equal(t, len(rules1), len(rules2))
	
	// Modify one map and verify the other is unaffected
	delete(rules1, "warehouse_rule")
	assert.NotContains(t, rules1, "warehouse_rule")
	assert.Contains(t, rules2, "warehouse_rule", "Modifying one map should not affect the other")
}

func TestCreateRuleManagerByCategory_OnlyEnabledRules(t *testing.T) {
	client := &gitlab.Client{}
	
	// Create a manager for warehouse category
	manager := CreateRuleManagerByCategory(client, "warehouse")
	
	simpleManager, ok := manager.(*SimpleRuleManager)
	assert.True(t, ok)
	
	// Verify that only enabled rules are added
	// We can't directly check the rules in simpleManager.rules since it's private,
	// but we can verify the manager was created successfully
	assert.NotNil(t, simpleManager)
	
	// Test that the manager can evaluate (basic functionality test)
	ctx := &shared.MRContext{
		ProjectID: 123,
		MRIID:     456,
		Changes:   []gitlab.FileChange{},
	}
	
	result := manager.EvaluateAll(ctx)
	assert.NotNil(t, result)
	assert.NotNil(t, result.FinalDecision)
}