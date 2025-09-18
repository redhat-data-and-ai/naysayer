package rules

import (
	"testing"

	"github.com/redhat-data-and-ai/naysayer/internal/config"
	"github.com/redhat-data-and-ai/naysayer/internal/gitlab"
	"github.com/redhat-data-and-ai/naysayer/internal/rules/shared"
	"github.com/stretchr/testify/assert"
)

// TestStrictPolicy_SQLFilesRequireManualReview verifies that SQL files are never auto-approved
func TestStrictPolicy_SQLFilesRequireManualReview(t *testing.T) {
	// Create a minimal config for testing
	ruleConfig := &config.GlobalRuleConfig{
		Enabled: true,
		Files:   []config.FileRuleConfig{}, // No rules configured
	}

	manager := NewSectionRuleManager(ruleConfig)

	// Create MR context with SQL file changes
	mrCtx := &shared.MRContext{
		ProjectID: 123,
		MRIID:     456,
		Changes: []gitlab.FileChange{
			{
				NewPath: "dataproducts/aggregate/test/migrations/V1__create_table.sql",
				Diff:    "@@ -0,0 +1,3 @@\n+CREATE TABLE test (\n+    id INT PRIMARY KEY\n+);",
			},
		},
		MRInfo: &gitlab.MRInfo{
			Title:  "Add migration script",
			Author: "developer",
		},
	}

	// Evaluate the MR
	result := manager.EvaluateAll(mrCtx)

	// Verify strict policy is enforced
	assert.NotNil(t, result)
	assert.Equal(t, shared.ManualReview, result.FinalDecision.Type)
	assert.Contains(t, result.FinalDecision.Reason, "manual review")
	assert.Equal(t, 1, result.TotalFiles)
	assert.Equal(t, 0, result.ApprovedFiles)
	assert.Equal(t, 1, result.ReviewFiles)

	// Verify file-level decision
	assert.Len(t, result.FileValidations, 1)
	sqlFileValidation := result.FileValidations["dataproducts/aggregate/test/migrations/V1__create_table.sql"]
	assert.NotNil(t, sqlFileValidation)
	assert.Equal(t, shared.ManualReview, sqlFileValidation.FileDecision)
}

// TestStrictPolicy_MixedFiles verifies that any uncovered file causes manual review
func TestStrictPolicy_MixedFiles(t *testing.T) {
	// Create config with only YAML product file rules
	ruleConfig := &config.GlobalRuleConfig{
		Enabled: true,
		Files: []config.FileRuleConfig{
			{
				Name:       "product_configs",
				Path:       "",
				Filename:   "product.yaml",
				ParserType: "yaml",
				Enabled:    true,
				Sections: []config.SectionDefinition{
					{
						Name:     "name",
						YAMLPath: "name",
						RuleConfigs: []config.RuleConfig{
							{Name: "metadata_rule", Enabled: true},
						},
						AutoApprove: true,
					},
				},
			},
		},
	}

	manager := NewSectionRuleManager(ruleConfig)

	// Create MR context with both covered and uncovered files
	mrCtx := &shared.MRContext{
		ProjectID: 123,
		MRIID:     456,
		Changes: []gitlab.FileChange{
			{
				NewPath: "product.yaml",
				Diff:    "@@ -1,1 +1,1 @@\n-name: oldname\n+name: newname",
			},
			{
				NewPath: "migrations/V1__create.sql", // Uncovered file
				Diff:    "@@ -0,0 +1,1 @@\n+CREATE TABLE test();",
			},
		},
		MRInfo: &gitlab.MRInfo{
			Title:  "Mixed changes",
			Author: "developer",
		},
	}

	// Evaluate the MR
	result := manager.EvaluateAll(mrCtx)

	// Verify strict policy is enforced - entire MR requires manual review
	assert.NotNil(t, result)
	assert.Equal(t, shared.ManualReview, result.FinalDecision.Type)
	assert.Contains(t, result.FinalDecision.Details, "V1__create.sql")
	assert.Equal(t, 2, result.TotalFiles)
	assert.Equal(t, 1, result.ApprovedFiles) // product.yaml should be approved
	assert.Equal(t, 1, result.ReviewFiles)   // SQL file requires review
}
