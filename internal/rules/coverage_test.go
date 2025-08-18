package rules

import (
	"testing"

	"github.com/redhat-data-and-ai/naysayer/internal/gitlab"
	"github.com/redhat-data-and-ai/naysayer/internal/rules/shared"
	"github.com/redhat-data-and-ai/naysayer/internal/rules/warehouse"
	"github.com/stretchr/testify/assert"
)

// MockGitLabClient for testing - implements the interface needed by warehouse rule
type MockGitLabClient struct {
	fileContent string
}

func (m *MockGitLabClient) FetchFileContent(projectID int, filePath, ref string) (*gitlab.FileContent, error) {
	return &gitlab.FileContent{Content: m.fileContent}, nil
}

func TestRuleManager_CompleteCoverageEnforcement(t *testing.T) {
	// Create rule manager with only warehouse rule
	manager := NewSimpleRuleManager()
	
	// Register only warehouse rule (pass nil since we're not testing GitLab integration)
	manager.AddRule(warehouse.NewRule(nil))

	tests := []struct {
		name            string
		changes         []gitlab.FileChange
		expectedDecision shared.DecisionType
		expectedReason   string
		description     string
	}{
		{
			name: "warehouse files only - should auto-approve",
			changes: []gitlab.FileChange{
				{NewPath: "dataproducts/analytics/product.yaml"},
				{NewPath: "dataproducts/reporting/product.yaml"},
			},
			expectedDecision: shared.Approve,
			expectedReason:   "files validated and approved",
			description:     "All files covered by warehouse rule",
		},
		{
			name: "documentation files - should require manual review without general rule",
			changes: []gitlab.FileChange{
				{NewPath: "README.md"},
				{NewPath: "docs/user-guide.md"},
				{NewPath: "CHANGELOG.md"},
			},
			expectedDecision: shared.ManualReview,
			expectedReason:   "Manual review required",
			description:     "Files not covered by specific rules require manual review",
		},
		{
			name: "mixed warehouse and uncovered files - should require manual review",
			changes: []gitlab.FileChange{
				{NewPath: "dataproducts/analytics/product.yaml"},
				{NewPath: "README.md"},
			},
			expectedDecision: shared.ManualReview,
			expectedReason:   "Manual review required",
			description:     "Any uncovered file forces manual review even if others are covered",
		},
		{
			name: "config files - should require manual review",
			changes: []gitlab.FileChange{
				{NewPath: "config/secrets.yaml"},
				{NewPath: "infrastructure/terraform/main.tf"},
			},
			expectedDecision: shared.ManualReview,
			expectedReason:   "Manual review required",
			description:     "Configuration files require manual review",
		},
		{
			name: "source code files - should require manual review",
			changes: []gitlab.FileChange{
				{NewPath: "src/main.go"},
				{NewPath: "lib/utils.py"},
			},
			expectedDecision: shared.ManualReview,
			expectedReason:   "Manual review required",
			description:     "Source code files require manual review",
		},
		{
			name: "mixed warehouse and config files - should require manual review",
			changes: []gitlab.FileChange{
				{NewPath: "dataproducts/analytics/product.yaml"}, // Covered by warehouse rule
				{NewPath: "config/secrets.yaml"},                  // Not covered by any rule
			},
			expectedDecision: shared.ManualReview,
			expectedReason:   "Manual review required",
			description:     "Any uncovered file forces manual review",
		},
		{
			name: "unknown file types - should require manual review",
			changes: []gitlab.FileChange{
				{NewPath: "random-file.xyz"},
				{NewPath: "config/unknown-settings.conf"},
			},
			expectedDecision: shared.ManualReview,
			expectedReason:   "Manual review required",
			description:     "Unknown files require manual review",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mrCtx := &shared.MRContext{
				ProjectID: 123,
				MRIID:     456,
				Changes:   tt.changes,
				MRInfo:    &gitlab.MRInfo{Title: "Test MR", Author: "user"},
			}

			evaluation := manager.EvaluateAll(mrCtx)
			
			assert.Equal(t, tt.expectedDecision, evaluation.FinalDecision.Type, 
				"Expected decision type mismatch for: %s", tt.description)
			
			assert.Contains(t, evaluation.FinalDecision.Reason, tt.expectedReason,
				"Expected reason not found in: %s. Got: %s", tt.description, evaluation.FinalDecision.Reason)
			
			// Additional assertions based on decision type
			if evaluation.FinalDecision.Type == shared.Approve {
				assert.Contains(t, evaluation.FinalDecision.Details, "Approved",
					"Auto-approval should indicate successful validation")
			} else {
				assert.Contains(t, evaluation.FinalDecision.Summary, "Manual review required",
					"Manual review should be clearly indicated")
			}
		})
	}
}

func TestRuleManager_FileCoverageAnalysis(t *testing.T) {
	manager := NewSimpleRuleManager()
	
	// Register only warehouse rule (no general rule)
	manager.AddRule(warehouse.NewRule(nil))

	mrCtx := &shared.MRContext{
		Changes: []gitlab.FileChange{
			{NewPath: "dataproducts/analytics/product.yaml"}, // Covered
			{NewPath: "README.md"},                           // Not covered
			{NewPath: "src/main.go"},                         // Not covered
		},
	}

	coverage := manager.analyzeFileCoverage(mrCtx)

	assert.Equal(t, 3, coverage.TotalFiles, "Should count all files")
	assert.Equal(t, 1, coverage.CoveredFiles, "Should count covered files")
	assert.Equal(t, 2, len(coverage.UncoveredFiles), "Should identify uncovered files")
	assert.True(t, coverage.HasUncoveredFiles, "Should detect uncovered files")
	
	// Check specific uncovered files
	assert.Contains(t, coverage.UncoveredFiles, "README.md")
	assert.Contains(t, coverage.UncoveredFiles, "src/main.go")
}

func TestRuleManager_NoRulesRegistered(t *testing.T) {
	// Empty rule manager - no rules registered
	manager := NewSimpleRuleManager()

	mrCtx := &shared.MRContext{
		Changes: []gitlab.FileChange{
			{NewPath: "any-file.txt"},
		},
	}

	evaluation := manager.EvaluateAll(mrCtx)

	// Should NEVER auto-approve when no rules are registered
	assert.Equal(t, shared.ManualReview, evaluation.FinalDecision.Type,
		"Should require manual review when no rules cover any files")
	
	assert.Contains(t, evaluation.FinalDecision.Reason, "Manual review required",
		"Should clearly state that manual review is required")
}
