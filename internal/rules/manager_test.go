package rules

import (
	"testing"

	"github.com/redhat-data-and-ai/naysayer/internal/gitlab"
	"github.com/redhat-data-and-ai/naysayer/internal/rules/shared"
	"github.com/stretchr/testify/assert"
)

func TestSimpleRuleManager_EvaluateAll_DraftMR(t *testing.T) {
	manager := NewSimpleRuleManager()

	mrCtx := &shared.MRContext{
		ProjectID: 123,
		MRIID:     456,
		Changes: []gitlab.FileChange{
			{NewPath: "test.yaml"},
		},
		MRInfo: &gitlab.MRInfo{
			Title:  "Draft: Test MR",
			Author: "user1",
		},
	}

	result := manager.EvaluateAll(mrCtx)

	assert.Equal(t, shared.Approve, result.FinalDecision.Type, "Draft MR should be auto-approved")
	assert.Equal(t, "Draft MR - auto-approved", result.FinalDecision.Reason)
	assert.Equal(t, "✅ Draft MR skipped", result.FinalDecision.Summary)
	assert.Contains(t, result.FinalDecision.Details, "Draft MRs are automatically approved")
	assert.Equal(t, 0, len(result.FileValidations), "No files should be evaluated for draft MR")
	assert.True(t, result.ExecutionTime > 0, "Execution time should be recorded")
}

func TestSimpleRuleManager_EvaluateAll_AutomatedUser(t *testing.T) {
	manager := NewSimpleRuleManager()

	mrCtx := &shared.MRContext{
		ProjectID: 123,
		MRIID:     456,
		Changes: []gitlab.FileChange{
			{NewPath: "test.yaml"},
		},
		MRInfo: &gitlab.MRInfo{
			Title:  "Test MR",
			Author: "renovate[bot]", // Automated user
		},
	}

	result := manager.EvaluateAll(mrCtx)

	assert.Equal(t, shared.Approve, result.FinalDecision.Type, "Bot MR should be auto-approved")
	assert.Equal(t, "Automated user MR - auto-approved", result.FinalDecision.Reason)
	assert.Equal(t, "🤖 Bot MR skipped", result.FinalDecision.Summary)
	assert.Contains(t, result.FinalDecision.Details, "automated users")
	assert.Equal(t, 0, len(result.FileValidations), "No files should be evaluated for bot MR")
}

func TestSimpleRuleManager_EvaluateAll_NoRules(t *testing.T) {
	manager := NewSimpleRuleManager()
	// No rules registered

	mrCtx := &shared.MRContext{
		ProjectID: 123,
		MRIID:     456,
		Changes: []gitlab.FileChange{
			{NewPath: "test.yaml"},
		},
		MRInfo: &gitlab.MRInfo{
			Title:  "Test MR",
			Author: "user1",
		},
	}

	result := manager.EvaluateAll(mrCtx)

	assert.Equal(t, shared.ManualReview, result.FinalDecision.Type, "Should require manual review when no rules cover files")
	assert.Contains(t, result.FinalDecision.Reason, "Manual review required")
	assert.Contains(t, result.FinalDecision.Summary, "Manual review required")
	assert.Equal(t, 1, len(result.FileValidations), "Should validate the file even with no rules")
	assert.Equal(t, shared.ManualReview, result.FileValidations["test.yaml"].FileDecision)
}

func TestSimpleRuleManager_LineValidation(t *testing.T) {
	manager := NewSimpleRuleManager()

	// Add a mock rule that covers all lines in the file
	mockRule := &TestRule{
		name: "test_rule",
		coveredLines: []shared.LineRange{
			{StartLine: 1, EndLine: 7, FilePath: "test.yaml"}, // Cover all lines from getFileContent (7 lines total)
		},
		decision: shared.Approve,
		reason:   "Test validation passed",
	}
	manager.AddRule(mockRule)

	mrCtx := &shared.MRContext{
		ProjectID: 123,
		MRIID:     456,
		Changes: []gitlab.FileChange{
			{NewPath: "test.yaml"},
		},
		MRInfo: &gitlab.MRInfo{
			Title:  "Test MR",
			Author: "user1",
		},
	}

	result := manager.EvaluateAll(mrCtx)

	assert.Equal(t, shared.Approve, result.FinalDecision.Type, "Should approve when rule covers all lines")
	assert.Equal(t, 1, len(result.FileValidations), "Should have validation for one file")

	fileValidation := result.FileValidations["test.yaml"]
	assert.Equal(t, shared.Approve, fileValidation.FileDecision)
	assert.Equal(t, 1, len(fileValidation.RuleResults))
	assert.Equal(t, "test_rule", fileValidation.RuleResults[0].RuleName)
	assert.Equal(t, shared.Approve, fileValidation.RuleResults[0].Decision)
}

// TestRule is a mock rule for testing
type TestRule struct {
	name         string
	coveredLines []shared.LineRange
	decision     shared.DecisionType
	reason       string
}

func (r *TestRule) Name() string {
	return r.name
}

func (r *TestRule) Description() string {
	return "Test rule for unit testing"
}

func (r *TestRule) GetCoveredLines(filePath string, fileContent string) []shared.LineRange {
	// Return covered lines only for files that match our test
	var result []shared.LineRange
	for _, lineRange := range r.coveredLines {
		if lineRange.FilePath == filePath {
			result = append(result, lineRange)
		}
	}
	return result
}

func (r *TestRule) ValidateLines(filePath string, fileContent string, lineRanges []shared.LineRange) (shared.DecisionType, string) {
	return r.decision, r.reason
}
