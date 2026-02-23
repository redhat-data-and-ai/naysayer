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

func TestDataProductConsumerRule_detectSelfConsumer(t *testing.T) {
	tests := []struct {
		name                 string
		filePath             string
		fileContent          string
		expectedSelfConsumer bool
		expectedName         string
	}{
		{
			name:     "self-consumer with data_product kind should be detected",
			filePath: "dataproducts/aggregate/analytics/prod/product.yaml",
			fileContent: `---
name: analytics
kind: aggregated
rover_group: dataverse-aggregate-analytics
data_product_db:
- database: analytics_db
  presentation_schemas:
  - name: marts
    consumers:
    - name: analytics
      kind: data_product`,
			expectedSelfConsumer: true,
			expectedName:         "analytics",
		},
		{
			name:     "different consumer should not be flagged",
			filePath: "dataproducts/aggregate/analytics/prod/product.yaml",
			fileContent: `---
name: analytics
kind: aggregated
rover_group: dataverse-aggregate-analytics
data_product_db:
- database: analytics_db
  presentation_schemas:
  - name: marts
    consumers:
    - name: journey
      kind: data_product`,
			expectedSelfConsumer: false,
			expectedName:         "",
		},
		{
			name:     "self-consumer with consumer_group kind should NOT be flagged",
			filePath: "dataproducts/aggregate/analytics/prod/product.yaml",
			fileContent: `---
name: analytics
kind: aggregated
rover_group: dataverse-aggregate-analytics
data_product_db:
- database: analytics_db
  presentation_schemas:
  - name: marts
    consumers:
    - name: analytics
      kind: consumer_group`,
			expectedSelfConsumer: false,
			expectedName:         "",
		},
		{
			name:     "self-consumer with service_account kind should NOT be flagged",
			filePath: "dataproducts/aggregate/analytics/prod/product.yaml",
			fileContent: `---
name: analytics
kind: aggregated
data_product_db:
- database: analytics_db
  presentation_schemas:
  - name: marts
    consumers:
    - name: analytics
      kind: service_account`,
			expectedSelfConsumer: false,
			expectedName:         "",
		},
		{
			name:     "multiple consumers with one self-consumer should be detected",
			filePath: "dataproducts/source/sfsales/prod/product.yaml",
			fileContent: `---
name: sfsales
kind: source-aligned
rover_group: dataverse-source-sfsales
data_product_db:
- database: sfsales_db
  presentation_schemas:
  - name: marts
    consumers:
    - name: journey
      kind: data_product
    - name: sfsales
      kind: data_product
    - name: forecasting
      kind: data_product`,
			expectedSelfConsumer: true,
			expectedName:         "sfsales",
		},
		{
			name:     "self-consumer in second schema should be detected",
			filePath: "dataproducts/aggregate/analytics/prod/product.yaml",
			fileContent: `---
name: analytics
kind: aggregated
data_product_db:
- database: analytics_db
  presentation_schemas:
  - name: marts
    consumers:
    - name: journey
      kind: data_product
  - name: staging
    consumers:
    - name: analytics
      kind: data_product`,
			expectedSelfConsumer: true,
			expectedName:         "analytics",
		},
		{
			name:     "empty consumers list should not be flagged",
			filePath: "dataproducts/aggregate/test/prod/product.yaml",
			fileContent: `---
name: test
kind: aggregated
data_product_db:
- database: test_db
  presentation_schemas:
  - name: marts
    consumers: []`,
			expectedSelfConsumer: false,
			expectedName:         "",
		},
		{
			name:     "no consumers section should not be flagged",
			filePath: "dataproducts/aggregate/test/prod/product.yaml",
			fileContent: `---
name: test
kind: aggregated
data_product_db:
- database: test_db
  presentation_schemas:
  - name: marts`,
			expectedSelfConsumer: false,
			expectedName:         "",
		},
		{
			name:     "file without name field should use path extraction",
			filePath: "dataproducts/aggregate/test/prod/product.yaml",
			fileContent: `---
kind: aggregated
data_product_db:
- database: test_db
  presentation_schemas:
  - name: marts
    consumers:
    - name: test
      kind: data_product`,
			expectedSelfConsumer: true,
			expectedName:         "test",
		},
		{
			name:     "section content without name field should use path extraction",
			filePath: "dataproducts/aggregate/analytics/prod/product.yaml",
			fileContent: `- database: analytics_db
  presentation_schemas:
  - name: marts
    consumers:
    - name: analytics
      kind: data_product`,
			expectedSelfConsumer: true,
			expectedName:         "analytics",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rule := NewDataProductConsumerRule([]string{"preprod", "prod"})

			isSelfConsumer, name := rule.detectSelfConsumer(tt.filePath, tt.fileContent)

			assert.Equal(t, tt.expectedSelfConsumer, isSelfConsumer)
			assert.Equal(t, tt.expectedName, name)
		})
	}
}

func TestDataProductConsumerRule_extractProductNameFromPath(t *testing.T) {
	tests := []struct {
		name         string
		filePath     string
		expectedName string
	}{
		{
			name:         "aggregate product in prod",
			filePath:     "dataproducts/aggregate/analytics/prod/product.yaml",
			expectedName: "analytics",
		},
		{
			name:         "source product in dev",
			filePath:     "dataproducts/source/sfsales/dev/product.yaml",
			expectedName: "sfsales",
		},
		{
			name:         "platform product in sandbox",
			filePath:     "dataproducts/platform/myproduct/sandbox/product.yaml",
			expectedName: "myproduct",
		},
		{
			name:         "with absolute path",
			filePath:     "/some/root/dataproducts/aggregate/analytics/prod/product.yaml",
			expectedName: "analytics",
		},
		{
			name:         "invalid path without dataproducts",
			filePath:     "some/other/path/product.yaml",
			expectedName: "",
		},
		{
			name:         "path too short",
			filePath:     "dataproducts/source/product.yaml",
			expectedName: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rule := NewDataProductConsumerRule([]string{"preprod", "prod"})

			result := rule.extractProductNameFromPath(tt.filePath)
			assert.Equal(t, tt.expectedName, result)
		})
	}
}

func TestDataProductConsumerRule_ValidateLines_SelfConsumer(t *testing.T) {
	selfConsumerYaml := `---
name: analytics
kind: aggregated
rover_group: dataverse-aggregate-analytics
data_product_db:
- database: analytics_db
  presentation_schemas:
  - name: marts
    consumers:
    - name: dataverse-consumer-analytics-marts
      kind: consumer_group
    - name: analytics
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
			name:        "self-consumer in prod should require manual review",
			filePath:    "dataproducts/aggregate/analytics/prod/product.yaml",
			fileContent: selfConsumerYaml,
			lineRanges: []shared.LineRange{
				{StartLine: 12, EndLine: 14, FilePath: "dataproducts/aggregate/analytics/prod/product.yaml"},
			},
			mrContext: &shared.MRContext{
				Changes: []gitlab.FileChange{
					{
						OldPath: "dataproducts/aggregate/analytics/prod/product.yaml",
						NewPath: "dataproducts/aggregate/analytics/prod/product.yaml",
						NewFile: false,
					},
				},
			},
			expectedDecision:       shared.ManualReview,
			expectedReasonContains: "Self-consumer detected",
		},
		{
			name:        "self-consumer in preprod should require manual review",
			filePath:    "dataproducts/aggregate/analytics/preprod/product.yaml",
			fileContent: selfConsumerYaml,
			lineRanges: []shared.LineRange{
				{StartLine: 12, EndLine: 14, FilePath: "dataproducts/aggregate/analytics/preprod/product.yaml"},
			},
			mrContext: &shared.MRContext{
				Changes: []gitlab.FileChange{
					{
						NewPath: "dataproducts/aggregate/analytics/preprod/product.yaml",
						NewFile: false,
					},
				},
			},
			expectedDecision:       shared.ManualReview,
			expectedReasonContains: "cannot be added as a consumer of itself",
		},
		{
			name:        "self-consumer in dev should also require manual review",
			filePath:    "dataproducts/aggregate/analytics/dev/product.yaml",
			fileContent: selfConsumerYaml,
			lineRanges: []shared.LineRange{
				{StartLine: 12, EndLine: 14, FilePath: "dataproducts/aggregate/analytics/dev/product.yaml"},
			},
			mrContext: &shared.MRContext{
				Changes: []gitlab.FileChange{
					{
						NewPath: "dataproducts/aggregate/analytics/dev/product.yaml",
						NewFile: true,
					},
				},
			},
			expectedDecision:       shared.ManualReview,
			expectedReasonContains: "analytics",
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
