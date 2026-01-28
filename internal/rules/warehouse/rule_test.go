package warehouse

import (
	"errors"
	"testing"

	"github.com/redhat-data-and-ai/naysayer/internal/gitlab"
	"github.com/redhat-data-and-ai/naysayer/internal/rules/shared"
	"github.com/stretchr/testify/assert"
)

// MockAnalyzer for testing
type MockAnalyzer struct {
	changes []WarehouseChange
	err     error
}

func (m *MockAnalyzer) AnalyzeChanges(projectID int, mrIID int, changes []gitlab.FileChange) ([]WarehouseChange, error) {
	return m.changes, m.err
}

func TestWarehouseRule_Name(t *testing.T) {
	rule := NewRule(nil)
	assert.Equal(t, "warehouse_rule", rule.Name())
}

func TestWarehouseRule_Description(t *testing.T) {
	rule := NewRule(nil)
	description := rule.Description()
	assert.Contains(t, description, "warehouse")
	assert.Contains(t, description, "product.yaml")
	assert.Contains(t, description, "manual review")
}

func TestWarehouseRule_isWarehouseFile(t *testing.T) {
	rule := NewRule(nil)

	tests := []struct {
		name     string
		path     string
		expected bool
	}{
		{"product.yaml file", "dataproducts/analytics/product.yaml", true},
		{"product.yml file", "path/to/product.yml", true},
		{"Product.YAML uppercase", "Product.YAML", true},
		{"nested product.yaml", "dataproducts/source/platform/dev/product.yaml", true},
		{"not a warehouse file - README", "README.md", false},
		{"not a warehouse file - config", "config.yaml", false},
		{"not a warehouse file - developers", "developers.yaml", false},
		{"empty path", "", false},
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
		{"warehouse file with content", "dataproducts/analytics/product.yaml", "name: test\nwarehouses:\n- type: user\n  size: XSMALL\n", true},
		{"warehouse file with minimal content", "product.yaml", "name: test", true},
		{"non-warehouse file", "README.md", "# README\nThis is a readme file\n", false},
		{"warehouse file with empty content", "product.yaml", "", false},
		{"warehouse file with whitespace only", "product.yaml", "   \n  \t  \n", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lines := rule.GetCoveredLines(tt.filePath, tt.fileContent)
			if tt.expectCover {
				assert.Len(t, lines, 1, "Should return exactly one placeholder line range for warehouse files")
				assert.Equal(t, tt.filePath, lines[0].FilePath)
				assert.Equal(t, 1, lines[0].StartLine)
				assert.Equal(t, 1, lines[0].EndLine)
			} else {
				assert.Len(t, lines, 0, "Should not cover lines for non-warehouse files or empty files")
			}
		})
	}
}

func TestWarehouseRule_ValidateLines_NoContext(t *testing.T) {
	rule := NewRule(nil)

	tests := []struct {
		name           string
		filePath       string
		expectedResult shared.DecisionType
		expectedReason string
	}{
		// Without analyzer/context, warehouse files require manual review for safety
		{"warehouse file without context", "dataproducts/analytics/product.yaml", shared.ManualReview, "Warehouse changes require manual review"},
		{"non-warehouse file", "README.md", shared.Approve, "Not a warehouse file"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lineRanges := []shared.LineRange{{StartLine: 1, EndLine: 4, FilePath: tt.filePath}}
			decision, reason := rule.ValidateLines(tt.filePath, "test content", lineRanges)
			assert.Equal(t, tt.expectedResult, decision)
			assert.Contains(t, reason, tt.expectedReason)
		})
	}
}

func TestWarehouseRule_ValidateLines_WithContext(t *testing.T) {
	tests := []struct {
		name               string
		filePath           string
		mockChanges        []WarehouseChange
		mockError          error
		expectedResult     shared.DecisionType
		expectedReasonPart string
	}{
		{
			name:               "no warehouse changes detected",
			filePath:           "dataproducts/analytics/product.yaml",
			mockChanges:        []WarehouseChange{}, // Empty - no changes
			mockError:          nil,
			expectedResult:     shared.Approve,
			expectedReasonPart: "No warehouse size changes detected",
		},
		{
			name:     "warehouse size increase - requires manual review",
			filePath: "dataproducts/analytics/product.yaml",
			mockChanges: []WarehouseChange{
				{FilePath: "dataproducts/analytics/product.yaml (type: user)", FromSize: "XSMALL", ToSize: "SMALL", IsDecrease: false},
			},
			mockError:          nil,
			expectedResult:     shared.ManualReview,
			expectedReasonPart: "Warehouse size increase detected",
		},
		{
			// All warehouse changes now require manual review (including decreases)
			name:     "warehouse size decrease - requires manual review",
			filePath: "dataproducts/analytics/product.yaml",
			mockChanges: []WarehouseChange{
				{FilePath: "dataproducts/analytics/product.yaml (type: user)", FromSize: "SMALL", ToSize: "XSMALL", IsDecrease: true},
			},
			mockError:          nil,
			expectedResult:     shared.ManualReview,
			expectedReasonPart: "Warehouse size decrease detected",
		},
		{
			name:     "non-warehouse changes ignored",
			filePath: "dataproducts/analytics/product.yaml",
			mockChanges: []WarehouseChange{
				{FilePath: "dataproducts/analytics/product.yaml", FromSize: "N/A", ToSize: "N/A"}, // Non-warehouse change
			},
			mockError:          nil,
			expectedResult:     shared.Approve,
			expectedReasonPart: "No warehouse size changes detected",
		},
		{
			name:     "new warehouse added",
			filePath: "dataproducts/analytics/product.yaml",
			mockChanges: []WarehouseChange{
				{FilePath: "dataproducts/analytics/product.yaml (type: user)", FromSize: "", ToSize: "SMALL", IsDecrease: false},
			},
			mockError:          nil,
			expectedResult:     shared.ManualReview,
			expectedReasonPart: "New user warehouse: SMALL",
		},
		{
			name:     "mixed warehouse changes - increase and decrease",
			filePath: "dataproducts/analytics/product.yaml",
			mockChanges: []WarehouseChange{
				{FilePath: "dataproducts/analytics/product.yaml (type: user)", FromSize: "SMALL", ToSize: "MEDIUM", IsDecrease: false},
				{FilePath: "dataproducts/analytics/product.yaml (type: loader)", FromSize: "LARGE", ToSize: "MEDIUM", IsDecrease: true},
			},
			mockError:          nil,
			expectedResult:     shared.ManualReview,
			expectedReasonPart: "Warehouse changes detected - manual review required",
		},
		{
			name:               "analyzer error",
			filePath:           "dataproducts/analytics/product.yaml",
			mockChanges:        nil,
			mockError:          errors.New("analyzer failed"),
			expectedResult:     shared.ManualReview,
			expectedReasonPart: "Warehouse analysis failed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create mock analyzer
			mockAnalyzer := &MockAnalyzer{
				changes: tt.mockChanges,
				err:     tt.mockError,
			}
			rule := NewRule(nil)
			rule.analyzer = mockAnalyzer

			// Set MR context
			mrCtx := &shared.MRContext{
				ProjectID: 123,
				MRIID:     456,
				Changes: []gitlab.FileChange{
					{NewPath: tt.filePath},
				},
			}
			rule.SetMRContext(mrCtx)

			// Test
			lineRanges := []shared.LineRange{{StartLine: 1, EndLine: 4, FilePath: tt.filePath}}
			decision, reason := rule.ValidateLines(tt.filePath, "test content", lineRanges)

			// Assertions
			assert.Equal(t, tt.expectedResult, decision)
			assert.Contains(t, reason, tt.expectedReasonPart)
		})
	}
}

func TestWarehouseRule_SetMRContext(t *testing.T) {
	rule := NewRule(nil)

	mrCtx := &shared.MRContext{
		ProjectID: 123,
		MRIID:     456,
		Changes: []gitlab.FileChange{
			{NewPath: "product.yaml"},
		},
	}

	rule.SetMRContext(mrCtx)
	assert.Equal(t, mrCtx, rule.mrCtx)
}

func TestWarehouseRule_extractWarehouseType(t *testing.T) {
	rule := NewRule(nil)

	tests := []struct {
		name         string
		filePath     string
		expectedType string
	}{
		{"user warehouse type", "dataproducts/analytics/product.yaml (type: user)", "user"},
		{"loader warehouse type", "path/to/product.yaml (type: loader)", "loader"},
		{"compute warehouse type", "product.yaml (type: compute)", "compute"},
		{"no type in path", "dataproducts/analytics/product.yaml", "unknown"},
		{"malformed type", "product.yaml (type:", "unknown"},
		{"empty path", "", "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := rule.extractWarehouseType(tt.filePath)
			assert.Equal(t, tt.expectedType, result)
		})
	}
}

// Test key scenarios that demonstrate the section-based approach
func TestWarehouseRule_SectionBasedScenarios(t *testing.T) {
	tests := []struct {
		name               string
		filePath           string
		fileContent        string
		mockChanges        []WarehouseChange
		expectCoverage     bool
		expectedDecision   shared.DecisionType
		expectedReasonPart string
	}{
		{
			name:        "DBT metadata changes only - should be approved by warehouse rule",
			filePath:    "dataproducts/source/fivetranplatform/sandbox/product.yaml",
			fileContent: "name: fivetranplatform\nservice_account:\n  dbt: sa-dbt-fivetranplatform-sandbox@redhat.com\n",
			mockChanges: []WarehouseChange{
				{FilePath: "dataproducts/source/fivetranplatform/sandbox/product.yaml", FromSize: "N/A", ToSize: "N/A"}, // Non-warehouse change
			},
			expectCoverage:     true,
			expectedDecision:   shared.Approve,
			expectedReasonPart: "No warehouse size changes detected",
		},
		{
			// All warehouse changes now require manual review (including decreases)
			name:        "warehouse size decrease - requires manual review",
			filePath:    "dataproducts/analytics/product.yaml",
			fileContent: "name: analytics\nwarehouses:\n- type: user\n  size: SMALL\n",
			mockChanges: []WarehouseChange{
				{FilePath: "dataproducts/analytics/product.yaml (type: user)", FromSize: "MEDIUM", ToSize: "SMALL", IsDecrease: true},
			},
			expectCoverage:     true,
			expectedDecision:   shared.ManualReview,
			expectedReasonPart: "Warehouse size decrease detected",
		},
		{
			name:        "warehouse size increase - should require manual review",
			filePath:    "dataproducts/ml/product.yaml",
			fileContent: "name: ml\nwarehouses:\n- type: compute\n  size: XLARGE\n",
			mockChanges: []WarehouseChange{
				{FilePath: "dataproducts/ml/product.yaml (type: compute)", FromSize: "LARGE", ToSize: "XLARGE", IsDecrease: false},
			},
			expectCoverage:     true,
			expectedDecision:   shared.ManualReview,
			expectedReasonPart: "compute warehouse: LARGE → XLARGE",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test coverage
			rule := NewRule(nil)
			lines := rule.GetCoveredLines(tt.filePath, tt.fileContent)
			if tt.expectCoverage {
				assert.Len(t, lines, 1, "Should cover warehouse files")
			} else {
				assert.Len(t, lines, 0, "Should not cover non-warehouse files")
			}

			// Test validation with context
			mockAnalyzer := &MockAnalyzer{
				changes: tt.mockChanges,
				err:     nil,
			}
			rule = NewRule(nil)
			rule.analyzer = mockAnalyzer

			mrCtx := &shared.MRContext{
				ProjectID: 123,
				MRIID:     456,
				Changes:   []gitlab.FileChange{{NewPath: tt.filePath}},
			}
			rule.SetMRContext(mrCtx)

			lineRanges := []shared.LineRange{{StartLine: 1, EndLine: 10, FilePath: tt.filePath}}
			decision, reason := rule.ValidateLines(tt.filePath, tt.fileContent, lineRanges)

			assert.Equal(t, tt.expectedDecision, decision)
			assert.Contains(t, reason, tt.expectedReasonPart)
		})
	}
}
