package warehouse

import (
	"testing"

	"github.com/redhat-data-and-ai/naysayer/internal/rules/shared"
	"github.com/stretchr/testify/assert"
)

func TestWarehouseRule_Name(t *testing.T) {
	rule := NewRule(nil)
	assert.Equal(t, "warehouse_rule", rule.Name())
}

func TestWarehouseRule_Description(t *testing.T) {
	rule := NewRule(nil)
	description := rule.Description()
	assert.Contains(t, description, "warehouse")
	assert.Contains(t, description, "product.yaml")
}

func TestWarehouseRule_isWarehouseFile(t *testing.T) {
	rule := NewRule(nil)

	tests := []struct {
		name     string
		path     string
		expected bool
	}{
		{
			name:     "product.yaml file",
			path:     "dataproducts/analytics/product.yaml",
			expected: true,
		},
		{
			name:     "product.yml file",
			path:     "path/to/product.yml",
			expected: true,
		},
		{
			name:     "Product.YAML uppercase",
			path:     "Product.YAML",
			expected: true,
		},
		{
			name:     "not a warehouse file",
			path:     "README.md",
			expected: false,
		},
		{
			name:     "empty path",
			path:     "",
			expected: false,
		},
		{
			name:     "different YAML file",
			path:     "config.yaml",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := rule.isWarehouseFile(tt.path)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestWarehouseRule_GetCoveredLines(t *testing.T) {
	rule := NewRule(nil)

	tests := []struct {
		name        string
		filePath    string
		fileContent string
		expectCover bool
	}{
		{
			name:        "warehouse file with content",
			filePath:    "dataproducts/analytics/product.yaml",
			fileContent: "name: test\nwarehouses:\n- type: user\n  size: XSMALL\n",
			expectCover: true,
		},
		{
			name:        "non-warehouse file",
			filePath:    "README.md",
			fileContent: "# README\nThis is a readme file\n",
			expectCover: false,
		},
		{
			name:        "warehouse file with empty content",
			filePath:    "product.yaml",
			fileContent: "",
			expectCover: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lines := rule.GetCoveredLines(tt.filePath, tt.fileContent)
			if tt.expectCover {
				assert.True(t, len(lines) > 0, "Should cover lines for warehouse files")
				assert.Equal(t, tt.filePath, lines[0].FilePath)
				assert.Equal(t, 1, lines[0].StartLine)
				assert.True(t, lines[0].EndLine > 0)
			} else {
				assert.Equal(t, 0, len(lines), "Should not cover lines for non-warehouse files")
			}
		})
	}
}

func TestWarehouseRule_ValidateLines(t *testing.T) {
	rule := NewRule(nil)

	tests := []struct {
		name           string
		filePath       string
		fileContent    string
		lineRanges     []shared.LineRange
		expectedResult shared.DecisionType
	}{
		{
			name:        "warehouse file validation",
			filePath:    "dataproducts/analytics/product.yaml",
			fileContent: "name: test\nwarehouses:\n- type: user\n  size: XSMALL\n",
			lineRanges: []shared.LineRange{
				{StartLine: 1, EndLine: 4, FilePath: "dataproducts/analytics/product.yaml"},
			},
			expectedResult: shared.Approve,
		},
		{
			name:        "non-warehouse file",
			filePath:    "README.md",
			fileContent: "# README\n",
			lineRanges: []shared.LineRange{
				{StartLine: 1, EndLine: 1, FilePath: "README.md"},
			},
			expectedResult: shared.Approve, // Should approve non-warehouse files (rule doesn't apply)
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			decision, reason := rule.ValidateLines(tt.filePath, tt.fileContent, tt.lineRanges)
			assert.Equal(t, tt.expectedResult, decision)
			assert.NotEmpty(t, reason)
		})
	}
}
