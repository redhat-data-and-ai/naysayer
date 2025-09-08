package shared

import (
	"testing"

	"github.com/redhat-data-and-ai/naysayer/internal/gitlab"
	"github.com/stretchr/testify/assert"
)

func TestDecisionType_Constants(t *testing.T) {
	assert.Equal(t, DecisionType("approve"), Approve)
	assert.Equal(t, DecisionType("manual_review"), ManualReview)
}

func TestDecision_Creation(t *testing.T) {
	decision := Decision{
		Type:    Approve,
		Reason:  "All tests passed",
		Summary: "Auto-approved",
		Details: "No issues found",
	}

	assert.Equal(t, Approve, decision.Type)
	assert.Equal(t, "All tests passed", decision.Reason)
	assert.Equal(t, "Auto-approved", decision.Summary)
	assert.Equal(t, "No issues found", decision.Details)
}

func TestMRContext_Creation(t *testing.T) {
	mrCtx := &MRContext{
		ProjectID:   123,
		MRIID:       456,
		Changes:     []gitlab.FileChange{{NewPath: "test.yaml"}},
		MRInfo:      &gitlab.MRInfo{Title: "Test MR", Author: "testuser"},
		Environment: "dev",
		Labels:      []string{"enhancement"},
		Metadata:    map[string]any{"priority": "high"},
	}

	assert.Equal(t, 123, mrCtx.ProjectID)
	assert.Equal(t, 456, mrCtx.MRIID)
	assert.Len(t, mrCtx.Changes, 1)
	assert.Equal(t, "test.yaml", mrCtx.Changes[0].NewPath)
	assert.Equal(t, "Test MR", mrCtx.MRInfo.Title)
	assert.Equal(t, "dev", mrCtx.Environment)
	assert.Contains(t, mrCtx.Labels, "enhancement")
	assert.Equal(t, "high", mrCtx.Metadata["priority"])
}

func TestLineRange_Creation(t *testing.T) {
	lineRange := LineRange{
		StartLine: 10,
		EndLine:   20,
		FilePath:  "dataproducts/analytics/product.yaml",
	}

	assert.Equal(t, 10, lineRange.StartLine)
	assert.Equal(t, 20, lineRange.EndLine)
	assert.Equal(t, "dataproducts/analytics/product.yaml", lineRange.FilePath)
}

func TestLineValidationResult_Creation(t *testing.T) {
	result := LineValidationResult{
		RuleName:   "test_rule",
		Decision:   Approve,
		Reason:     "Test validation passed",
		LineRanges: []LineRange{{StartLine: 1, EndLine: 5, FilePath: "test.yaml"}},
	}

	assert.Equal(t, "test_rule", result.RuleName)
	assert.Equal(t, Approve, result.Decision)
	assert.Equal(t, "Test validation passed", result.Reason)
	assert.Len(t, result.LineRanges, 1)
	assert.Equal(t, 1, result.LineRanges[0].StartLine)
}

func TestFileValidationSummary_Creation(t *testing.T) {
	summary := &FileValidationSummary{
		FilePath:       "dataproducts/analytics/product.yaml",
		TotalLines:     100,
		CoveredLines:   []LineRange{{StartLine: 1, EndLine: 50, FilePath: "test.yaml"}},
		UncoveredLines: []LineRange{{StartLine: 51, EndLine: 100, FilePath: "test.yaml"}},
		RuleResults: []LineValidationResult{
			{RuleName: "test_rule", Decision: Approve, Reason: "Passed"},
		},
		FileDecision: Approve,
	}

	assert.Equal(t, "dataproducts/analytics/product.yaml", summary.FilePath)
	assert.Equal(t, 100, summary.TotalLines)
	assert.Len(t, summary.CoveredLines, 1)
	assert.Len(t, summary.UncoveredLines, 1)
	assert.Len(t, summary.RuleResults, 1)
	assert.Equal(t, Approve, summary.FileDecision)
}

func TestRuleEvaluation_Creation(t *testing.T) {
	evaluation := &RuleEvaluation{
		FinalDecision: Decision{Type: Approve, Reason: "All files approved"},
		FileValidations: map[string]*FileValidationSummary{
			"test.yaml": {FilePath: "test.yaml", FileDecision: Approve},
		},
		TotalFiles:     1,
		ApprovedFiles:  1,
		ReviewFiles:    0,
		UncoveredFiles: 0,
	}

	assert.Equal(t, Approve, evaluation.FinalDecision.Type)
	assert.Len(t, evaluation.FileValidations, 1)
	assert.Equal(t, 1, evaluation.TotalFiles)
	assert.Equal(t, 1, evaluation.ApprovedFiles)
	assert.Equal(t, 0, evaluation.ReviewFiles)
}
