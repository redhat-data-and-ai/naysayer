package dataproduct_consumer

import (
	"testing"

	"github.com/redhat-data-and-ai/naysayer/internal/gitlab"
	"github.com/redhat-data-and-ai/naysayer/internal/rules/shared"
	"github.com/stretchr/testify/assert"
)

func TestNewDataProductConsumerRule(t *testing.T) {
	tests := []struct {
		name         string
		allowedEnvs  []string
		expectedName string
		expectedEnvs []string
	}{
		{
			name:         "with custom environments",
			allowedEnvs:  []string{"staging", "production"},
			expectedName: "dataproduct_consumer_rule",
			expectedEnvs: []string{"staging", "production"},
		},
		{
			name:         "with nil environments (should use defaults)",
			allowedEnvs:  nil,
			expectedName: "dataproduct_consumer_rule",
			expectedEnvs: []string{"preprod", "prod"},
		},
		{
			name:         "with empty environments",
			allowedEnvs:  []string{},
			expectedName: "dataproduct_consumer_rule",
			expectedEnvs: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rule := NewDataProductConsumerRule(tt.allowedEnvs)

			assert.Equal(t, tt.expectedName, rule.Name())
			assert.Contains(t, rule.Description(), "consumer access")
			assert.Equal(t, tt.expectedEnvs, rule.config.AllowedEnvironments)
		})
	}
}

func TestDataProductConsumerRule_ValidateLines(t *testing.T) {
	consumerYaml := `---
name: rosettastone
kind: aggregated
rover_group: dataverse-aggregate-rosettastone
data_product_db:
- database: rosettastone_db
  presentation_schemas:
  - name: marts
    consumers:
    - name: journey
      kind: data_product`

	tests := []struct {
		name                   string
		filePath               string
		fileContent            string
		lineRanges             []shared.LineRange
		mrContext              *shared.MRContext
		expectedDecision       shared.DecisionType
		expectedReasonContains string
	}{
		{
			name:                   "non-product file should approve",
			filePath:               "dataproducts/test/README.md",
			fileContent:            "# Test",
			lineRanges:             []shared.LineRange{{StartLine: 1, EndLine: 1, FilePath: "dataproducts/test/README.md"}},
			mrContext:              &shared.MRContext{},
			expectedDecision:       shared.Approve,
			expectedReasonContains: "Not a product.yaml file",
		},
		{
			name:        "consumer changes in prod environment should auto-approve",
			filePath:    "dataproducts/analytics/prod/product.yaml",
			fileContent: consumerYaml,
			lineRanges: []shared.LineRange{
				{StartLine: 9, EndLine: 11, FilePath: "dataproducts/analytics/prod/product.yaml"},
			},
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
			expectedReasonContains: "data product owner approval sufficient",
		},
		{
			name:        "consumer changes in preprod environment should auto-approve",
			filePath:    "dataproducts/analytics/preprod/product.yaml",
			fileContent: consumerYaml,
			lineRanges: []shared.LineRange{
				{StartLine: 9, EndLine: 11, FilePath: "dataproducts/analytics/preprod/product.yaml"},
			},
			mrContext: &shared.MRContext{
				Changes: []gitlab.FileChange{
					{
						OldPath: "dataproducts/analytics/preprod/product.yaml",
						NewPath: "dataproducts/analytics/preprod/product.yaml",
						NewFile: false,
					},
				},
			},
			expectedDecision:       shared.Approve,
			expectedReasonContains: "data product owner approval sufficient",
		},
		{
			name:        "consumer changes in dev environment should auto-approve",
			filePath:    "dataproducts/analytics/dev/product.yaml",
			fileContent: consumerYaml,
			lineRanges: []shared.LineRange{
				{StartLine: 9, EndLine: 11, FilePath: "dataproducts/analytics/dev/product.yaml"},
			},
			mrContext: &shared.MRContext{
				Changes: []gitlab.FileChange{
					{
						NewPath: "dataproducts/analytics/dev/product.yaml",
						NewFile: true,
					},
				},
			},
			expectedDecision:       shared.Approve,
			expectedReasonContains: "data product owner approval sufficient",
		},
		{
			name:        "non-consumer changes should approve with generic message",
			filePath:    "dataproducts/analytics/prod/product.yaml",
			fileContent: consumerYaml,
			lineRanges: []shared.LineRange{
				{StartLine: 1, EndLine: 4, FilePath: "dataproducts/analytics/prod/product.yaml"},
			},
			mrContext: &shared.MRContext{
				Changes: []gitlab.FileChange{
					{
						NewPath: "dataproducts/analytics/prod/product.yaml",
						NewFile: false,
					},
				},
			},
			expectedDecision:       shared.Approve,
			expectedReasonContains: "No consumer-only changes detected",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rule := NewDataProductConsumerRule([]string{"preprod", "prod"})
			rule.SetMRContext(tt.mrContext)

			decision, reason := rule.ValidateLines(tt.filePath, tt.fileContent, tt.lineRanges)

			assert.Equal(t, tt.expectedDecision, decision)
			assert.Contains(t, reason, tt.expectedReasonContains)
		})
	}
}

func TestDataProductConsumerRule_GetCoveredLines(t *testing.T) {
	consumerYaml := `---
name: rosettastone
kind: aggregated
rover_group: dataverse-aggregate-rosettastone
data_product_db:
- database: rosettastone_db
  presentation_schemas:
  - name: marts
    consumers:
    - name: journey
      kind: data_product`

	tests := []struct {
		name                string
		filePath            string
		fileContent         string
		expectedCoverageLen int
		expectedStartLine   int
		expectedEndLine     int
	}{
		{
			name:                "non-product file should have no coverage",
			filePath:            "dataproducts/test/README.md",
			fileContent:         "# Test",
			expectedCoverageLen: 0,
		},
		{
			name:                "product.yaml with consumers should return placeholder",
			filePath:            "dataproducts/analytics/prod/product.yaml",
			fileContent:         consumerYaml,
			expectedCoverageLen: 1,
			expectedStartLine:   1,
			expectedEndLine:     1,
		},
		{
			name:     "product.yaml without consumers should return placeholder",
			filePath: "dataproducts/analytics/prod/product.yaml",
			fileContent: `---
name: test
kind: aggregated`,
			expectedCoverageLen: 1,
			expectedStartLine:   1,
			expectedEndLine:     1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rule := NewDataProductConsumerRule([]string{"preprod", "prod"})

			coveredLines := rule.GetCoveredLines(tt.filePath, tt.fileContent)

			assert.Len(t, coveredLines, tt.expectedCoverageLen)

			if tt.expectedCoverageLen > 0 {
				assert.Equal(t, tt.expectedStartLine, coveredLines[0].StartLine)
				assert.Equal(t, tt.expectedEndLine, coveredLines[0].EndLine)
			}
		})
	}
}

func TestDataProductConsumerRule_extractEnvironmentFromPath(t *testing.T) {
	tests := []struct {
		name     string
		filePath string
		expected string
	}{
		{
			name:     "prod in directory path",
			filePath: "dataproducts/analytics/prod/product.yaml",
			expected: "prod",
		},
		{
			name:     "preprod in directory path",
			filePath: "dataproducts/source/preprod/product.yaml",
			expected: "preprod",
		},
		{
			name:     "dev in directory path",
			filePath: "dataproducts/analytics/dev/product.yaml",
			expected: "dev",
		},
		{
			name:     "sandbox in directory path",
			filePath: "dataproducts/analytics/sandbox/product.yaml",
			expected: "sandbox",
		},
		{
			name:     "environment with underscore",
			filePath: "dataproducts/analytics/my_prod_setup/product.yaml",
			expected: "prod",
		},
		{
			name:     "case insensitive matching",
			filePath: "dataproducts/analytics/PROD/product.yaml",
			expected: "prod",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rule := NewDataProductConsumerRule([]string{"preprod", "prod"})

			result := rule.extractEnvironmentFromPath(tt.filePath)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestDataProductConsumerRule_isConsumerRelatedLine(t *testing.T) {
	tests := []struct {
		name     string
		line     string
		expected bool
	}{
		{
			name:     "consumers keyword",
			line:     "    consumers:",
			expected: true,
		},
		{
			name:     "name field in consumer",
			line:     "    - name: journey",
			expected: true,
		},
		{
			name:     "kind field in consumer",
			line:     "      kind: data_product",
			expected: true,
		},
		{
			name:     "non-consumer line",
			line:     "    warehouse:",
			expected: false,
		},
		{
			name:     "database line",
			line:     "- database: test_db",
			expected: false,
		},
		{
			name:     "empty line",
			line:     "",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rule := NewDataProductConsumerRule([]string{"preprod", "prod"})

			result := rule.isConsumerRelatedLine(tt.line)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestDataProductConsumerRule_fileContainsConsumersSection(t *testing.T) {
	tests := []struct {
		name        string
		fileContent string
		expected    bool
	}{
		{
			name: "file with consumers section",
			fileContent: `---
name: rosettastone
data_product_db:
- database: rosettastone_db
  presentation_schemas:
  - name: marts
    consumers:
    - name: journey
      kind: data_product`,
			expected: true,
		},
		{
			name: "file without consumers section",
			fileContent: `---
name: test
kind: aggregated
data_product_db:
- database: test_db
  presentation_schemas:
  - name: marts`,
			expected: false,
		},
		{
			name: "empty consumers section",
			fileContent: `---
name: test
data_product_db:
- database: test_db
  presentation_schemas:
  - name: marts
    consumers: []`,
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rule := NewDataProductConsumerRule([]string{"preprod", "prod"})

			result := rule.fileContainsConsumersSection(tt.fileContent)
			assert.Equal(t, tt.expected, result)
		})
	}
}
