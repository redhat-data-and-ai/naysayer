package rules

import (
	"testing"
	"time"

	"github.com/redhat-data-and-ai/naysayer/internal/gitlab"
	"github.com/redhat-data-and-ai/naysayer/internal/rules/shared"
	"github.com/stretchr/testify/assert"
)

func TestNewSimpleRuleManager(t *testing.T) {
	manager := NewSimpleRuleManager()
	assert.NotNil(t, manager, "Manager should not be nil")
	assert.NotNil(t, manager.rules, "Rules slice should be initialized")
	assert.Equal(t, 0, len(manager.rules), "Rules slice should be empty initially")
}

func TestSimpleRuleManager_AddRule(t *testing.T) {
	manager := NewSimpleRuleManager()
	rule1 := &MockRule{name: "test_rule_1"}
	rule2 := &MockRule{name: "test_rule_2"}

	// Add first rule
	manager.AddRule(rule1)
	assert.Equal(t, 1, len(manager.rules), "Should have 1 rule after adding first")

	// Add second rule
	manager.AddRule(rule2)
	assert.Equal(t, 2, len(manager.rules), "Should have 2 rules after adding second")
}

func TestSimpleRuleManager_EvaluateAll_DraftMR(t *testing.T) {
	manager := NewSimpleRuleManager()
	rule := &MockRule{name: "test_rule", applies: true, decision: shared.Approve}
	manager.AddRule(rule)

	mrCtx := &shared.MRContext{
		MRInfo: &gitlab.MRInfo{
			Title: "Draft: Test MR",
		},
		Changes: []gitlab.FileChange{
			{NewPath: "test.yaml"},
		},
	}

	result := manager.EvaluateAll(mrCtx)

	assert.Equal(t, shared.Approve, result.FinalDecision.Type, "Draft MR should be auto-approved")
	assert.Equal(t, "Draft MR - auto-approved", result.FinalDecision.Reason)
	assert.Equal(t, "✅ Draft MR skipped", result.FinalDecision.Summary)
	assert.Contains(t, result.FinalDecision.Details, "Draft MRs are automatically approved")
	assert.Equal(t, 0, len(result.RuleResults), "No rules should be evaluated for draft MR")
	assert.True(t, result.ExecutionTime > 0, "Execution time should be recorded")
	assert.False(t, rule.applyWasCalled, "Rules should not be evaluated for draft MR")
}

func TestSimpleRuleManager_EvaluateAll_AutomatedUser(t *testing.T) {
	manager := NewSimpleRuleManager()
	rule := &MockRule{name: "test_rule", applies: true, decision: shared.Approve}
	manager.AddRule(rule)

	mrCtx := &shared.MRContext{
		MRInfo: &gitlab.MRInfo{
			Title:  "Update dependencies",
			Author: "dependabot[bot]",
		},
		Changes: []gitlab.FileChange{
			{NewPath: "package.json"},
		},
	}

	result := manager.EvaluateAll(mrCtx)

	assert.Equal(t, shared.Approve, result.FinalDecision.Type, "Automated user MR should be auto-approved")
	assert.Equal(t, "Automated user MR - auto-approved", result.FinalDecision.Reason)
	assert.Equal(t, "🤖 Bot MR skipped", result.FinalDecision.Summary)
	assert.Contains(t, result.FinalDecision.Details, "automated users")
	assert.Equal(t, 0, len(result.RuleResults), "No rules should be evaluated for bot MR")
	assert.False(t, rule.applyWasCalled, "Rules should not be evaluated for bot MR")
}

func TestSimpleRuleManager_EvaluateAll_NoApplicableRules(t *testing.T) {
	manager := NewSimpleRuleManager()
	rule1 := &MockRule{name: "rule1", applies: false}
	rule2 := &MockRule{name: "rule2", applies: false}
	manager.AddRule(rule1)
	manager.AddRule(rule2)

	mrCtx := &shared.MRContext{
		MRInfo: &gitlab.MRInfo{Title: "Normal MR", Author: "user"},
		Changes: []gitlab.FileChange{
			{NewPath: "README.md"},
		},
	}

	result := manager.EvaluateAll(mrCtx)

	assert.Equal(t, shared.Approve, result.FinalDecision.Type, "Should auto-approve when no rules apply")
	assert.Equal(t, "No applicable rules found", result.FinalDecision.Reason)
	assert.Equal(t, "✅ No rules apply - auto-approved", result.FinalDecision.Summary)
	assert.Contains(t, result.FinalDecision.Details, "No rules matched")
	assert.Equal(t, 0, len(result.RuleResults), "No rule results when no rules apply")
	assert.True(t, rule1.applyWasCalled, "Apply should be called to check if rule applies")
	assert.True(t, rule2.applyWasCalled, "Apply should be called to check if rule applies")
	assert.False(t, rule1.shouldApproveWasCalled, "ShouldApprove should not be called for non-applicable rules")
	assert.False(t, rule2.shouldApproveWasCalled, "ShouldApprove should not be called for non-applicable rules")
}

func TestSimpleRuleManager_EvaluateAll_AllRulesApprove(t *testing.T) {
	manager := NewSimpleRuleManager()
	rule1 := &MockRule{
		name:     "warehouse_rule",
		applies:  true,
		decision: shared.Approve,
		reason:   "Auto-approving MR with only 2 warehouse changes",
	}
	manager.AddRule(rule1)

	mrCtx := &shared.MRContext{
		MRInfo: &gitlab.MRInfo{Title: "Update configs", Author: "user"},
		Changes: []gitlab.FileChange{
			{NewPath: "dataproducts/agg/test/product.yaml"},
		},
	}

	result := manager.EvaluateAll(mrCtx)

	assert.Equal(t, shared.Approve, result.FinalDecision.Type, "Should approve when all rules approve")
	assert.Equal(t, "All applicable rules approved", result.FinalDecision.Reason)
	assert.Equal(t, "✅ All rules approved", result.FinalDecision.Summary)
	assert.Contains(t, result.FinalDecision.Details, "Rule evaluations:")
	assert.Contains(t, result.FinalDecision.Details, "warehouse_rule:")
	assert.Equal(t, 1, len(result.RuleResults), "Should have results from the rule")

	// Check individual rule results
	assert.Equal(t, "warehouse_rule", result.RuleResults[0].RuleName)
	assert.Equal(t, shared.Approve, result.RuleResults[0].Decision.Type)
	assert.Equal(t, "✅ warehouse_rule approved", result.RuleResults[0].Decision.Summary)
	assert.Equal(t, 1.0, result.RuleResults[0].Confidence)

	assert.True(t, rule1.applyWasCalled, "Apply should be called")
	assert.True(t, rule1.shouldApproveWasCalled, "ShouldApprove should be called")
}

func TestSimpleRuleManager_EvaluateAll_OneRuleRequiresManualReview(t *testing.T) {
	manager := NewSimpleRuleManager()
	rule1 := &MockRule{
		name:     "warehouse_rule",
		applies:  true,
		decision: shared.Approve,
		reason:   "Auto-approving warehouse changes",
	}
	rule2 := &MockRule{
		name:     "second_rule",
		applies:  true,
		decision: shared.ManualReview,
		reason:   "MR contains non-dataverse file changes",
	}
	rule3 := &MockRule{
		name:     "third_rule",
		applies:  true,
		decision: shared.Approve,
		reason:   "This rule would approve",
	}
	manager.AddRule(rule1)
	manager.AddRule(rule2)
	manager.AddRule(rule3)

	mrCtx := &shared.MRContext{
		MRInfo: &gitlab.MRInfo{Title: "Mixed changes", Author: "user"},
		Changes: []gitlab.FileChange{
			{NewPath: "dataproducts/agg/test/product.yaml"},
			{NewPath: "README.md"},
		},
	}

	result := manager.EvaluateAll(mrCtx)

	assert.Equal(t, shared.ManualReview, result.FinalDecision.Type, "Should require manual review when any rule does")
	assert.Equal(t, "One or more rules require manual approval", result.FinalDecision.Reason)
	assert.Equal(t, "🚫 Manual review required", result.FinalDecision.Summary)
	assert.Contains(t, result.FinalDecision.Details, "Rule evaluations:")
	assert.Contains(t, result.FinalDecision.Details, "warehouse_rule:")
	assert.Contains(t, result.FinalDecision.Details, "second_rule:")

	// Should have results from first two rules (stops at manual review)
	assert.Equal(t, 2, len(result.RuleResults), "Should stop evaluation after manual review")
	assert.Equal(t, shared.Approve, result.RuleResults[0].Decision.Type)
	assert.Equal(t, shared.ManualReview, result.RuleResults[1].Decision.Type)

	assert.True(t, rule1.applyWasCalled, "First rule should be evaluated")
	assert.True(t, rule2.applyWasCalled, "Second rule should be evaluated")
	assert.False(t, rule3.applyWasCalled, "Third rule should not be evaluated after manual review")
}

func TestSimpleRuleManager_EvaluateAll_MixedApplicableRules(t *testing.T) {
	manager := NewSimpleRuleManager()
	rule1 := &MockRule{name: "rule1", applies: false}
	rule2 := &MockRule{
		name:     "rule2",
		applies:  true,
		decision: shared.Approve,
		reason:   "Rule 2 approves",
	}
	rule3 := &MockRule{name: "rule3", applies: false}
	rule4 := &MockRule{
		name:     "rule4",
		applies:  true,
		decision: shared.Approve,
		reason:   "Rule 4 approves",
	}
	manager.AddRule(rule1)
	manager.AddRule(rule2)
	manager.AddRule(rule3)
	manager.AddRule(rule4)

	mrCtx := &shared.MRContext{
		MRInfo:  &gitlab.MRInfo{Title: "Test MR", Author: "user"},
		Changes: []gitlab.FileChange{{NewPath: "test.yaml"}},
	}

	result := manager.EvaluateAll(mrCtx)

	assert.Equal(t, shared.Approve, result.FinalDecision.Type, "Should approve when applicable rules approve")
	assert.Equal(t, 2, len(result.RuleResults), "Should have results only from applicable rules")
	assert.Equal(t, "rule2", result.RuleResults[0].RuleName)
	assert.Equal(t, "rule4", result.RuleResults[1].RuleName)

	// Check that all rules had Applies called but only applicable ones had ShouldApprove called
	assert.True(t, rule1.applyWasCalled)
	assert.False(t, rule1.shouldApproveWasCalled)
	assert.True(t, rule2.applyWasCalled)
	assert.True(t, rule2.shouldApproveWasCalled)
	assert.True(t, rule3.applyWasCalled)
	assert.False(t, rule3.shouldApproveWasCalled)
	assert.True(t, rule4.applyWasCalled)
	assert.True(t, rule4.shouldApproveWasCalled)
}

func TestSimpleRuleManager_createSummary(t *testing.T) {
	manager := NewSimpleRuleManager()

	tests := []struct {
		name         string
		ruleName     string
		decision     shared.DecisionType
		expectedText string
	}{
		{
			name:         "approve decision",
			ruleName:     "test_rule",
			decision:     shared.Approve,
			expectedText: "✅ test_rule approved",
		},
		{
			name:         "manual review decision",
			ruleName:     "warehouse_rule",
			decision:     shared.ManualReview,
			expectedText: "🚫 warehouse_rule requires manual review",
		},
		{
			name:         "unknown decision",
			ruleName:     "unknown_rule",
			decision:     shared.DecisionType("unknown"),
			expectedText: "❓ unknown_rule unknown decision",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := manager.createSummary(tt.ruleName, tt.decision)
			assert.Equal(t, tt.expectedText, result)
		})
	}
}

func TestSimpleRuleManager_createDetailsFromResults(t *testing.T) {
	manager := NewSimpleRuleManager()

	tests := []struct {
		name     string
		results  []shared.RuleResult
		expected string
	}{
		{
			name:     "empty results",
			results:  []shared.RuleResult{},
			expected: "",
		},
		{
			name: "single result",
			results: []shared.RuleResult{
				{
					RuleName: "warehouse_rule",
					Decision: shared.Decision{
						Reason: "Auto-approving warehouse changes",
					},
				},
			},
			expected: "Rule evaluations:\n- warehouse_rule: Auto-approving warehouse changes\n",
		},
		{
			name: "multiple results",
			results: []shared.RuleResult{
				{
					RuleName: "warehouse_rule",
					Decision: shared.Decision{
						Reason: "Auto-approving warehouse changes",
					},
				},
				{
					RuleName: "second_rule",
					Decision: shared.Decision{
						Reason: "Manual review required",
					},
				},
			},
			expected: "Rule evaluations:\n- warehouse_rule: Auto-approving warehouse changes\n- second_rule: Manual review required\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := manager.createDetailsFromResults(tt.results)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestSimpleRuleManager_EvaluateAll_ExecutionTime(t *testing.T) {
	manager := NewSimpleRuleManager()
	rule := &MockRule{
		name:     "slow_rule",
		applies:  true,
		decision: shared.Approve,
		reason:   "Slow rule result",
		delay:    10 * time.Millisecond, // Simulate some processing time
	}
	manager.AddRule(rule)

	mrCtx := &shared.MRContext{
		MRInfo:  &gitlab.MRInfo{Title: "Test MR", Author: "user"},
		Changes: []gitlab.FileChange{{NewPath: "test.yaml"}},
	}

	result := manager.EvaluateAll(mrCtx)

	assert.True(t, result.ExecutionTime >= 10*time.Millisecond, "Total execution time should include rule execution time")
	assert.True(t, result.RuleResults[0].ExecutionTime > 0, "Individual rule execution time should be recorded")
	assert.True(t, result.RuleResults[0].ExecutionTime >= 10*time.Millisecond, "Rule execution time should include delay")
}

// MockRule is a test implementation of the Rule interface
type MockRule struct {
	name                   string
	applies                bool
	decision               shared.DecisionType
	reason                 string
	delay                  time.Duration
	applyWasCalled         bool
	shouldApproveWasCalled bool
}

func (m *MockRule) Name() string {
	return m.name
}

func (m *MockRule) Description() string {
	return "Mock rule for testing: " + m.name
}

func (m *MockRule) Applies(mrCtx *shared.MRContext) bool {
	m.applyWasCalled = true
	if m.delay > 0 {
		time.Sleep(m.delay)
	}
	return m.applies
}

func (m *MockRule) ShouldApprove(mrCtx *shared.MRContext) (shared.DecisionType, string) {
	m.shouldApproveWasCalled = true
	if m.delay > 0 {
		time.Sleep(m.delay)
	}
	return m.decision, m.reason
}
