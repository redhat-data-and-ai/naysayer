package ai_status

import (
	"testing"

	"github.com/redhat-data-and-ai/naysayer/internal/rules/shared"
	"github.com/stretchr/testify/assert"
)

func TestAiStatusRule_ValidateLines(t *testing.T) {
	rule := NewAiStatusRule()

	tests := []struct {
		name                   string
		filePath               string
		fileContent            string
		lineRanges             []shared.LineRange
		expectedDecision       shared.DecisionType
		expectedReasonContains string
	}{
		{
			name:     "line ranges cover ai_status line",
			filePath: "dataproducts/source/jira/sandbox/product.yaml",
			fileContent: `data_product_db:
- database: jira_db
  presentation_schemas:
  - name: marts
    ai_status: ready`,
			lineRanges:             []shared.LineRange{{StartLine: 5, EndLine: 5, FilePath: "dataproducts/source/jira/sandbox/product.yaml"}},
			expectedDecision:       shared.Approve,
			expectedReasonContains: "ai_status changes detected",
		},
		{
			name:     "ai_status and consumer change lines both in range",
			filePath: "dataproducts/source/jira/sandbox/product.yaml",
			fileContent: `data_product_db:
- database: jira_db
  presentation_schemas:
  - name: marts
    ai_status: ready
    consumers:
    - name: journey
      kind: data_product
    - name: newproduct
      kind: data_product`,
			lineRanges:             []shared.LineRange{{StartLine: 5, EndLine: 5, FilePath: "dataproducts/source/jira/sandbox/product.yaml"}},
			expectedDecision:       shared.Approve,
			expectedReasonContains: "ai_status changes detected",
		},
		{
			name:     "line ranges do not cover ai_status line - filtered by GetCoveredLinesForChanges",
			filePath: "dataproducts/source/jira/sandbox/product.yaml",
			fileContent: `data_product_db:
- database: jira_db
  presentation_schemas:
  - name: marts
    ai_status: ready
    consumers:
    - name: journey
      kind: data_product`,
			lineRanges:             nil,
			expectedDecision:       shared.Approve,
			expectedReasonContains: "No ai_status changes detected",
		},
		{
			name:     "consumer-only content without ai_status",
			filePath: "dataproducts/source/jira/sandbox/product.yaml",
			fileContent: `data_product_db:
- database: jira_db
  presentation_schemas:
  - name: marts
    consumers:
    - name: journey
      kind: data_product`,
			lineRanges:             nil,
			expectedDecision:       shared.Approve,
			expectedReasonContains: "No ai_status changes detected",
		},
		{
			name:     "non-product file with ai_status",
			filePath: "config/settings.yaml",
			fileContent: `ai_status: public
settings:
  debug: true`,
			lineRanges:             []shared.LineRange{{StartLine: 1, EndLine: 3, FilePath: "config/settings.yaml"}},
			expectedDecision:       shared.Approve,
			expectedReasonContains: "Not a product file",
		},
		{
			name:                   "empty line ranges",
			filePath:               "dataproducts/source/jira/sandbox/product.yaml",
			fileContent:            "ai_status: ready",
			lineRanges:             []shared.LineRange{},
			expectedDecision:       shared.Approve,
			expectedReasonContains: "No ai_status changes detected",
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

func TestAiStatusRule_GetCoveredLines(t *testing.T) {
	rule := NewAiStatusRule()

	t.Run("product.yaml with ai_status returns actual line", func(t *testing.T) {
		content := `data_product_db:
- database: jira_db
  presentation_schemas:
  - name: marts
    ai_status: ready`
		coveredLines := rule.GetCoveredLines("dataproducts/source/jira/sandbox/product.yaml", content)

		assert.Len(t, coveredLines, 1)
		assert.Equal(t, 5, coveredLines[0].StartLine)
		assert.Equal(t, 5, coveredLines[0].EndLine)
	})

	t.Run("product.yaml with multiple ai_status returns all lines", func(t *testing.T) {
		content := `data_product_db:
- database: db
  presentation_schemas:
  - name: marts
    ai_status: ready
  - name: rhai_marts
    ai_status: restricted`
		coveredLines := rule.GetCoveredLines("dataproducts/source/jira/prod/product.yaml", content)

		assert.Len(t, coveredLines, 2)
		assert.Equal(t, 5, coveredLines[0].StartLine)
		assert.Equal(t, 7, coveredLines[1].StartLine)
	})

	t.Run("product.yaml without ai_status returns placeholder", func(t *testing.T) {
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
		coveredLines := rule.GetCoveredLines("config/settings.yaml", "ai_status: public")

		assert.Empty(t, coveredLines)
	})
}

func TestAiStatusRule_GetCoveredLinesForChanges(t *testing.T) {
	rule := NewAiStatusRule()

	t.Run("changed lines include ai_status line", func(t *testing.T) {
		content := `data_product_db:
- database: jira_db
  presentation_schemas:
  - name: marts
    ai_status: ready
    consumers:
    - name: journey
      kind: data_product`
		filePath := "dataproducts/source/jira/sandbox/product.yaml"
		changedLines := []shared.LineRange{{StartLine: 4, EndLine: 6, FilePath: filePath}}

		result := rule.GetCoveredLinesForChanges(filePath, content, changedLines)

		assert.Len(t, result, 1)
		assert.Equal(t, 5, result[0].StartLine)
		assert.Equal(t, 5, result[0].EndLine)
	})

	t.Run("changed lines do not include ai_status line", func(t *testing.T) {
		content := `data_product_db:
- database: jira_db
  presentation_schemas:
  - name: marts
    ai_status: ready
    consumers:
    - name: journey
      kind: data_product`
		filePath := "dataproducts/source/jira/sandbox/product.yaml"
		changedLines := []shared.LineRange{{StartLine: 6, EndLine: 8, FilePath: filePath}}

		result := rule.GetCoveredLinesForChanges(filePath, content, changedLines)

		assert.Empty(t, result)
	})

	t.Run("multiple ai_status lines only one changed", func(t *testing.T) {
		content := `data_product_db:
- database: db
  presentation_schemas:
  - name: marts
    ai_status: ready
  - name: rhai_marts
    ai_status: excluded`
		filePath := "dataproducts/source/jira/prod/product.yaml"
		changedLines := []shared.LineRange{{StartLine: 7, EndLine: 7, FilePath: filePath}}

		result := rule.GetCoveredLinesForChanges(filePath, content, changedLines)

		assert.Len(t, result, 1)
		assert.Equal(t, 7, result[0].StartLine)
	})

	t.Run("non-product file returns nil", func(t *testing.T) {
		content := `ai_status: public`
		filePath := "config/settings.yaml"
		changedLines := []shared.LineRange{{StartLine: 1, EndLine: 1, FilePath: filePath}}

		result := rule.GetCoveredLinesForChanges(filePath, content, changedLines)

		assert.Nil(t, result)
	})
}

func TestAiStatusRule_Name(t *testing.T) {
	rule := NewAiStatusRule()
	assert.Equal(t, "ai_status_rule", rule.Name())
}

func TestAiStatusRule_Description(t *testing.T) {
	rule := NewAiStatusRule()
	assert.Equal(t, "Requires manual review for ai_status changes", rule.Description())
}
