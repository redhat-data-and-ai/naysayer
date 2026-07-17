package access_policy

import (
	"testing"

	"github.com/redhat-data-and-ai/naysayer/internal/rules/shared"
	"github.com/stretchr/testify/assert"
)

func TestAccessPolicyRule_ValidateLines(t *testing.T) {
	rule := NewAccessPolicyRule()

	tests := []struct {
		name                   string
		filePath               string
		fileContent            string
		lineRanges             []shared.LineRange
		expectedDecision       shared.DecisionType
		expectedReasonContains string
	}{
		{
			name:     "line ranges cover access_policy line",
			filePath: "dataproducts/source/jira/sandbox/product.yaml",
			fileContent: `data_product_db:
- database: jira_db
  presentation_schemas:
  - name: marts
    access_policy: rh_internal`,
			lineRanges:             []shared.LineRange{{StartLine: 1, EndLine: 5, FilePath: "dataproducts/source/jira/sandbox/product.yaml"}},
			expectedDecision:       shared.ManualReview,
			expectedReasonContains: "access_policy changes require manual review",
		},
		{
			name:     "access_policy and consumer change lines both in range",
			filePath: "dataproducts/source/jira/sandbox/product.yaml",
			fileContent: `data_product_db:
- database: jira_db
  presentation_schemas:
  - name: marts
    access_policy: rh_internal
    consumers:
    - name: journey
      kind: data_product
    - name: newproduct
      kind: data_product`,
			lineRanges:             []shared.LineRange{{StartLine: 4, EndLine: 10, FilePath: "dataproducts/source/jira/sandbox/product.yaml"}},
			expectedDecision:       shared.ManualReview,
			expectedReasonContains: "access_policy changes require manual review",
		},
		{
			name:     "line ranges do not cover access_policy line",
			filePath: "dataproducts/source/jira/sandbox/product.yaml",
			fileContent: `data_product_db:
- database: jira_db
  presentation_schemas:
  - name: marts
    access_policy: rh_internal
    consumers:
    - name: journey
      kind: data_product`,
			lineRanges:             []shared.LineRange{{StartLine: 6, EndLine: 8, FilePath: "dataproducts/source/jira/sandbox/product.yaml"}},
			expectedDecision:       shared.Approve,
			expectedReasonContains: "No access_policy changes detected",
		},
		{
			name:     "consumer-only content without access_policy",
			filePath: "dataproducts/source/jira/sandbox/product.yaml",
			fileContent: `data_product_db:
- database: jira_db
  presentation_schemas:
  - name: marts
    consumers:
    - name: journey
      kind: data_product`,
			lineRanges:             []shared.LineRange{{StartLine: 1, EndLine: 7, FilePath: "dataproducts/source/jira/sandbox/product.yaml"}},
			expectedDecision:       shared.Approve,
			expectedReasonContains: "No access_policy changes detected",
		},
		{
			name:     "non-product file with access_policy",
			filePath: "config/settings.yaml",
			fileContent: `access_policy: public
settings:
  debug: true`,
			lineRanges:             []shared.LineRange{{StartLine: 1, EndLine: 3, FilePath: "config/settings.yaml"}},
			expectedDecision:       shared.Approve,
			expectedReasonContains: "Not a product file",
		},
		{
			name:                   "empty line ranges",
			filePath:               "dataproducts/source/jira/sandbox/product.yaml",
			fileContent:            "access_policy: rh_internal",
			lineRanges:             []shared.LineRange{},
			expectedDecision:       shared.Approve,
			expectedReasonContains: "No access_policy changes detected",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			decision, reason := rule.ValidateLines(tt.filePath, tt.fileContent, tt.lineRanges)

			assert.Equal(t, tt.expectedDecision, decision)
			assert.Contains(t, reason, tt.expectedReasonContains)
		})
	}
}

func TestAccessPolicyRule_GetCoveredLines(t *testing.T) {
	rule := NewAccessPolicyRule()

	t.Run("product.yaml with access_policy returns actual line", func(t *testing.T) {
		content := `data_product_db:
- database: jira_db
  presentation_schemas:
  - name: marts
    access_policy: rh_internal`
		coveredLines := rule.GetCoveredLines("dataproducts/source/jira/sandbox/product.yaml", content)

		assert.Len(t, coveredLines, 1)
		assert.Equal(t, 5, coveredLines[0].StartLine)
		assert.Equal(t, 5, coveredLines[0].EndLine)
	})

	t.Run("product.yaml with multiple access_policy returns all lines", func(t *testing.T) {
		content := `data_product_db:
- database: db
  presentation_schemas:
  - name: marts
    access_policy: rh_internal
  - name: rhai_marts
    access_policy: restricted`
		coveredLines := rule.GetCoveredLines("dataproducts/source/jira/prod/product.yaml", content)

		assert.Len(t, coveredLines, 2)
		assert.Equal(t, 5, coveredLines[0].StartLine)
		assert.Equal(t, 7, coveredLines[1].StartLine)
	})

	t.Run("product.yaml without access_policy returns placeholder", func(t *testing.T) {
		content := `data_product_db:
- database: jira_db
  presentation_schemas:
  - name: marts
    consumers:
    - name: journey
      kind: data_product`
		coveredLines := rule.GetCoveredLines("dataproducts/source/jira/sandbox/product.yaml", content)

		assert.Len(t, coveredLines, 1)
		assert.Equal(t, 1, coveredLines[0].StartLine)
		assert.Equal(t, 1, coveredLines[0].EndLine)
	})

	t.Run("product.yaml with empty content returns placeholder", func(t *testing.T) {
		coveredLines := rule.GetCoveredLines("dataproducts/source/test/sandbox/product.yaml", "")

		assert.Len(t, coveredLines, 1)
		assert.Equal(t, 1, coveredLines[0].StartLine)
	})

	t.Run("non-product file returns empty", func(t *testing.T) {
		coveredLines := rule.GetCoveredLines("config/settings.yaml", "access_policy: public")

		assert.Empty(t, coveredLines)
	})
}

func TestAccessPolicyRule_Name(t *testing.T) {
	rule := NewAccessPolicyRule()
	assert.Equal(t, "access_policy_rule", rule.Name())
}

func TestAccessPolicyRule_Description(t *testing.T) {
	rule := NewAccessPolicyRule()
	assert.Equal(t, "Requires manual review for access_policy changes", rule.Description())
}
