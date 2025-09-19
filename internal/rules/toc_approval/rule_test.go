package toc_approval

import (
	"testing"

	"github.com/redhat-data-and-ai/naysayer/internal/gitlab"
	"github.com/redhat-data-and-ai/naysayer/internal/rules/shared"
	"github.com/stretchr/testify/assert"
)

func TestNewTOCApprovalRule(t *testing.T) {
	tests := []struct {
		name            string
		preprodProdEnvs []string
		expectedName    string
		expectedEnvs    []string
	}{
		{
			name:            "with custom environments",
			preprodProdEnvs: []string{"staging", "production"},
			expectedName:    "toc_approval_rule",
			expectedEnvs:    []string{"staging", "production"},
		},
		{
			name:            "with nil environments (should use defaults)",
			preprodProdEnvs: nil,
			expectedName:    "toc_approval_rule",
			expectedEnvs:    []string{"preprod", "prod"},
		},
		{
			name:            "with empty environments",
			preprodProdEnvs: []string{},
			expectedName:    "toc_approval_rule",
			expectedEnvs:    []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rule := NewTOCApprovalRule(tt.preprodProdEnvs)

			assert.Equal(t, tt.expectedName, rule.Name())
			assert.Contains(t, rule.Description(), "TOC approval")
			assert.Equal(t, tt.expectedEnvs, rule.config.RequiredEnvironments)
		})
	}
}

func TestTOCApprovalRule_ValidateLines(t *testing.T) {
	tests := []struct {
		name                   string
		filePath               string
		fileContent            string
		mrContext              *shared.MRContext
		expectedDecision       shared.DecisionType
		expectedReasonContains string
	}{
		{
			name:                   "non-product file should approve",
			filePath:               "dataproducts/test/README.md",
			fileContent:            "# Test",
			mrContext:              &shared.MRContext{},
			expectedDecision:       shared.Approve,
			expectedReasonContains: "Not a product.yaml file",
		},
		{
			name:        "new product.yaml in prod environment should require manual review",
			filePath:    "dataproducts/analytics/prod/product.yaml",
			fileContent: "name: test-product\nversion: 1.0",
			mrContext: &shared.MRContext{
				Changes: []gitlab.FileChange{
					{
						NewPath: "dataproducts/analytics/prod/product.yaml",
						NewFile: true,
					},
				},
			},
			expectedDecision:       shared.ManualReview,
			expectedReasonContains: "TOC (Technical Oversight Committee) approval",
		},
		{
			name:        "new product.yaml in preprod environment should require manual review",
			filePath:    "dataproducts/analytics/preprod/product.yaml",
			fileContent: "name: test-product\nversion: 1.0",
			mrContext: &shared.MRContext{
				Changes: []gitlab.FileChange{
					{
						NewPath: "dataproducts/analytics/preprod/product.yaml",
						NewFile: true,
					},
				},
			},
			expectedDecision:       shared.ManualReview,
			expectedReasonContains: "preprod environment requires TOC",
		},
		{
			name:        "existing product.yaml in prod should approve",
			filePath:    "dataproducts/analytics/prod/product.yaml",
			fileContent: "name: test-product\nversion: 1.0",
			mrContext: &shared.MRContext{
				Changes: []gitlab.FileChange{
					{
						OldPath: "dataproducts/analytics/prod/product.yaml",
						NewPath: "dataproducts/analytics/prod/product.yaml",
						NewFile: false,
					},
				},
			},
			expectedDecision:       shared.Approve,
			expectedReasonContains: "Existing product.yaml file",
		},
		{
			name:        "new product.yaml in dev environment should approve",
			filePath:    "dataproducts/analytics/dev/product.yaml",
			fileContent: "name: test-product\nversion: 1.0",
			mrContext: &shared.MRContext{
				Changes: []gitlab.FileChange{
					{
						NewPath: "dataproducts/analytics/dev/product.yaml",
						NewFile: true,
					},
				},
			},
			expectedDecision:       shared.Approve,
			expectedReasonContains: "not in critical environment",
		},
		{
			name:        "new product.yaml with prod in filename should require manual review",
			filePath:    "dataproducts/analytics/my_prod_setup/product.yaml",
			fileContent: "name: test-product\nversion: 1.0",
			mrContext: &shared.MRContext{
				Changes: []gitlab.FileChange{
					{
						NewPath: "dataproducts/analytics/my_prod_setup/product.yaml",
						NewFile: true,
					},
				},
			},
			expectedDecision:       shared.ManualReview,
			expectedReasonContains: "TOC (Technical Oversight Committee) approval",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rule := NewTOCApprovalRule([]string{"preprod", "prod"})
			rule.SetMRContext(tt.mrContext)

			lineRanges := []shared.LineRange{
				{StartLine: 1, EndLine: 2, FilePath: tt.filePath},
			}

			decision, reason := rule.ValidateLines(tt.filePath, tt.fileContent, lineRanges)

			assert.Equal(t, tt.expectedDecision, decision)
			assert.Contains(t, reason, tt.expectedReasonContains)
		})
	}
}

func TestTOCApprovalRule_GetCoveredLines(t *testing.T) {
	tests := []struct {
		name                string
		filePath            string
		fileContent         string
		mrContext           *shared.MRContext
		expectedCoverageLen int
		shouldCoverFullFile bool
	}{
		{
			name:                "non-product file should have no coverage",
			filePath:            "dataproducts/test/README.md",
			fileContent:         "# Test",
			mrContext:           &shared.MRContext{},
			expectedCoverageLen: 0,
			shouldCoverFullFile: false,
		},
		{
			name:        "new product.yaml in prod should cover full file",
			filePath:    "dataproducts/analytics/prod/product.yaml",
			fileContent: "name: test-product\nversion: 1.0\ntags:\n  - production",
			mrContext: &shared.MRContext{
				Changes: []gitlab.FileChange{
					{
						NewPath: "dataproducts/analytics/prod/product.yaml",
						NewFile: true,
					},
				},
			},
			expectedCoverageLen: 1,
			shouldCoverFullFile: true,
		},
		{
			name:        "existing product.yaml should have minimal coverage",
			filePath:    "dataproducts/analytics/prod/product.yaml",
			fileContent: "name: test-product\nversion: 1.0",
			mrContext: &shared.MRContext{
				Changes: []gitlab.FileChange{
					{
						OldPath: "dataproducts/analytics/prod/product.yaml",
						NewPath: "dataproducts/analytics/prod/product.yaml",
						NewFile: false,
					},
				},
			},
			expectedCoverageLen: 1,
			shouldCoverFullFile: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rule := NewTOCApprovalRule([]string{"preprod", "prod"})
			rule.SetMRContext(tt.mrContext)

			coveredLines := rule.GetCoveredLines(tt.filePath, tt.fileContent)

			assert.Len(t, coveredLines, tt.expectedCoverageLen)

			if tt.shouldCoverFullFile && len(coveredLines) > 0 {
				expectedLines := shared.CountLines(tt.fileContent)
				assert.Equal(t, 1, coveredLines[0].StartLine)
				assert.Equal(t, expectedLines, coveredLines[0].EndLine)
			}
		})
	}
}

func TestTOCApprovalRule_analyzeFile(t *testing.T) {
	tests := []struct {
		name                     string
		filePath                 string
		mrContext                *shared.MRContext
		expectedRequiresApproval bool
		expectedEnvironment      string
	}{
		{
			name:     "new file in prod directory",
			filePath: "dataproducts/analytics/prod/product.yaml",
			mrContext: &shared.MRContext{
				Changes: []gitlab.FileChange{
					{
						NewPath: "dataproducts/analytics/prod/product.yaml",
						NewFile: true,
					},
				},
			},
			expectedRequiresApproval: true,
			expectedEnvironment:      "prod",
		},
		{
			name:     "new file in preprod directory",
			filePath: "dataproducts/source/preprod/product.yaml",
			mrContext: &shared.MRContext{
				Changes: []gitlab.FileChange{
					{
						NewPath: "dataproducts/source/preprod/product.yaml",
						NewFile: true,
					},
				},
			},
			expectedRequiresApproval: true,
			expectedEnvironment:      "preprod",
		},
		{
			name:     "existing file in prod directory",
			filePath: "dataproducts/analytics/prod/product.yaml",
			mrContext: &shared.MRContext{
				Changes: []gitlab.FileChange{
					{
						OldPath: "dataproducts/analytics/prod/product.yaml",
						NewPath: "dataproducts/analytics/prod/product.yaml",
						NewFile: false,
					},
				},
			},
			expectedRequiresApproval: false,
			expectedEnvironment:      "prod",
		},
		{
			name:     "new file in dev directory",
			filePath: "dataproducts/analytics/dev/product.yaml",
			mrContext: &shared.MRContext{
				Changes: []gitlab.FileChange{
					{
						NewPath: "dataproducts/analytics/dev/product.yaml",
						NewFile: true,
					},
				},
			},
			expectedRequiresApproval: false,
			expectedEnvironment:      "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rule := NewTOCApprovalRule([]string{"preprod", "prod"})
			rule.SetMRContext(tt.mrContext)

			context := rule.analyzeFile(tt.filePath)

			assert.Equal(t, tt.expectedRequiresApproval, context.RequiresApproval)
			assert.Equal(t, tt.expectedEnvironment, context.Environment)
			assert.Equal(t, tt.filePath, context.FilePath)
		})
	}
}

func TestTOCApprovalRule_extractEnvironmentFromPath(t *testing.T) {
	tests := []struct {
		name     string
		filePath string
		envs     []string
		expected string
	}{
		{
			name:     "prod in directory path",
			filePath: "dataproducts/analytics/prod/product.yaml",
			envs:     []string{"preprod", "prod"},
			expected: "prod",
		},
		{
			name:     "preprod in directory path",
			filePath: "dataproducts/source/preprod/product.yaml",
			envs:     []string{"preprod", "prod"},
			expected: "preprod",
		},
		{
			name:     "custom environment staging",
			filePath: "dataproducts/analytics/staging/product.yaml",
			envs:     []string{"staging", "production"},
			expected: "staging",
		},
		{
			name:     "environment with underscore",
			filePath: "dataproducts/analytics/my_prod_setup/product.yaml",
			envs:     []string{"preprod", "prod"},
			expected: "prod",
		},
		{
			name:     "no environment match",
			filePath: "dataproducts/analytics/dev/product.yaml",
			envs:     []string{"preprod", "prod"},
			expected: "",
		},
		{
			name:     "case insensitive matching",
			filePath: "dataproducts/analytics/PROD/product.yaml",
			envs:     []string{"preprod", "prod"},
			expected: "prod",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rule := NewTOCApprovalRule(tt.envs)

			result := rule.extractEnvironmentFromPath(tt.filePath)
			assert.Equal(t, tt.expected, result)
		})
	}
}
