package warehouse

import (
	"errors"
	"testing"

	"github.com/redhat-data-and-ai/naysayer/internal/gitlab"
	"github.com/redhat-data-and-ai/naysayer/internal/rules/shared"
	"github.com/stretchr/testify/assert"
)

func TestWarehouseRule_Name(t *testing.T) {
	rule := NewRule(nil)
	assert.Equal(t, "warehouse_rule", rule.Name())
}

func TestWarehouseRule_Description(t *testing.T) {
	rule := NewRule(nil)
	expected := "Auto-approves MRs with only dataverse-safe files (warehouse/sourcebinding), requires manual review for warehouse increases"
	assert.Equal(t, expected, rule.Description())
}

func TestWarehouseRule_Applies(t *testing.T) {
	rule := NewRule(nil)

	tests := []struct {
		name     string
		changes  []gitlab.FileChange
		expected bool
	}{
		{
			name:     "no changes",
			changes:  []gitlab.FileChange{},
			expected: false,
		},
		{
			name: "only warehouse files",
			changes: []gitlab.FileChange{
				{NewPath: "dataproducts/agg/bookings/prod/product.yaml"},
				{NewPath: "dataproducts/agg/costops/dev/product.yaml"},
			},
			expected: true,
		},
		{
			name: "mixed warehouse and other files",
			changes: []gitlab.FileChange{
				{NewPath: "dataproducts/agg/spa/prod/product.yaml"},
				{NewPath: "README.md"},
			},
			expected: true, // Rule applies because there are warehouse files
		},
		{
			name: "only non-warehouse files",
			changes: []gitlab.FileChange{
				{NewPath: "README.md"},
				{NewPath: "config/settings.yaml"},
			},
			expected: false,
		},
		{
			name: "warehouse file deletion",
			changes: []gitlab.FileChange{
				{OldPath: "dataproducts/agg/old/product.yaml", NewPath: ""},
			},
			expected: true,
		},
		{
			name: "warehouse file rename",
			changes: []gitlab.FileChange{
				{
					OldPath: "dataproducts/agg/old/product.yaml",
					NewPath: "dataproducts/agg/new/product.yaml",
				},
			},
			expected: true,
		},
		{
			name: "case insensitive detection",
			changes: []gitlab.FileChange{
				{NewPath: "dataproducts/agg/test/PRODUCT.YAML"},
			},
			expected: true,
		},
		{
			name: "product.yml extension",
			changes: []gitlab.FileChange{
				{NewPath: "dataproducts/agg/test/product.yml"},
			},
			expected: true,
		},
		{
			name: "sourcebinding files should apply rule",
			changes: []gitlab.FileChange{
				{NewPath: "dataproducts/source/test/sourcebinding.yaml"},
			},
			expected: true,
		},
		{
			name: "mixed sourcebinding and non-dataverse files should apply rule",
			changes: []gitlab.FileChange{
				{NewPath: "dataproducts/source/test/sourcebinding.yaml"},
				{NewPath: "README.md"},
			},
			expected: true, // CRITICAL: Must apply to catch mixed changes
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mrCtx := &shared.MRContext{
				Changes: tt.changes,
			}
			actual := rule.Applies(mrCtx)
			assert.Equal(t, tt.expected, actual, "Applies() failed")
		})
	}
}



func TestWarehouseRule_ShouldApprove(t *testing.T) {
	tests := []struct {
		name             string
		client           *gitlab.Client
		changes          []gitlab.FileChange
		mockAnalyzer     *MockAnalyzer
		expectedDecision shared.DecisionType
		expectedReason   string
	}{
		{
			name:             "nil client",
			client:           nil,
			changes:          []gitlab.FileChange{{NewPath: "dataproducts/agg/test/product.yaml"}},
			expectedDecision: shared.ManualReview,
			expectedReason:   "GitLab token not configured - cannot analyze dataproduct files",
		},
		{
			name:   "no dataverse files",
			client: &gitlab.Client{},
			changes: []gitlab.FileChange{
				{NewPath: "README.md"},
				{NewPath: "config/settings.yaml"},
			},
			expectedDecision: shared.Approve,
			expectedReason:   "No dataverse file changes detected",
		},
		{
			name:   "only sourcebinding changes",
			client: &gitlab.Client{},
			changes: []gitlab.FileChange{
				{NewPath: "dataproducts/source/marketo/sandbox/sourcebinding.yaml"},
				{NewPath: "dataproducts/source/sfsales/prod/sourcebinding.yaml"},
			},
			expectedDecision: shared.Approve,
			expectedReason:   "Auto-approving MR with only 2 sourcebinding changes",
		},
		{
			name:   "mixed dataverse changes without warehouse files",
			client: &gitlab.Client{},
			changes: []gitlab.FileChange{
				{NewPath: "dataproducts/source/marketo/sandbox/sourcebinding.yaml"},
			},
			expectedDecision: shared.Approve,
			expectedReason:   "Auto-approving MR with only 1 sourcebinding changes",
		},
		{
			name:   "mixed with non-dataverse files",
			client: &gitlab.Client{},
			changes: []gitlab.FileChange{
				{NewPath: "dataproducts/agg/spa/prod/product.yaml"},
				{NewPath: "README.md"},
			},
			expectedDecision: shared.ManualReview,
			expectedReason:   "MR contains changes outside the allowed scope of the warehouse rule",
		},
		{
			name:   "CRITICAL: sourcebinding with non-dataverse files should require manual review",
			client: &gitlab.Client{},
			changes: []gitlab.FileChange{
				{NewPath: "dataproducts/source/test/sourcebinding.yaml"},
				{NewPath: "README.md"},
				{NewPath: "scripts/deploy.sh"},
			},
			expectedDecision: shared.ManualReview,
			expectedReason:   "MR contains changes outside the allowed scope of the warehouse rule",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rule := NewRule(tt.client)
			if tt.mockAnalyzer != nil {
				rule.analyzer = tt.mockAnalyzer
			}

			mrCtx := &shared.MRContext{
				ProjectID: 123,
				MRIID:     456,
				Changes:   tt.changes,
			}
			decision, reason := rule.ShouldApprove(mrCtx)
			assert.Equal(t, tt.expectedDecision, decision, "ShouldApprove() decision failed")
			assert.Equal(t, tt.expectedReason, reason, "ShouldApprove() reason failed")
		})
	}
}

func TestWarehouseRule_ShouldApprove_WithWarehouseChanges(t *testing.T) {
	tests := []struct {
		name             string
		changes          []gitlab.FileChange
		warehouseChanges []WarehouseChange
		analyzerError    error
		expectedDecision shared.DecisionType
		expectedReason   string
		expectedContains string
	}{
		{
			name: "analyzer error",
			changes: []gitlab.FileChange{
				{NewPath: "dataproducts/agg/test/product.yaml"},
			},
			warehouseChanges: nil,
			analyzerError:    errors.New("network timeout"),
			expectedDecision: shared.ManualReview,
			expectedReason:   "Warehouse analysis failed: network timeout",
		},
		{
			name: "warehouse decrease - auto approve",
			changes: []gitlab.FileChange{
				{NewPath: "dataproducts/agg/test/product.yaml"},
			},
			warehouseChanges: []WarehouseChange{
				{
					FilePath:   "dataproducts/agg/test/product.yaml (type: snowflake)",
					FromSize:   "LARGE",
					ToSize:     "MEDIUM",
					IsDecrease: true,
				},
			},
			expectedDecision: shared.Approve,
			expectedReason:   "Auto-approving MR with only 1 warehouse changes",
		},
		{
			name: "warehouse increase - manual review",
			changes: []gitlab.FileChange{
				{NewPath: "dataproducts/agg/test/product.yaml"},
			},
			warehouseChanges: []WarehouseChange{
				{
					FilePath:   "dataproducts/agg/test/product.yaml (type: snowflake)",
					FromSize:   "MEDIUM",
					ToSize:     "LARGE",
					IsDecrease: false,
				},
			},
			expectedDecision: shared.ManualReview,
			expectedContains: "Warehouse increase detected: MEDIUM → LARGE",
		},
		{
			name: "multiple warehouse decreases",
			changes: []gitlab.FileChange{
				{NewPath: "dataproducts/agg/test1/product.yaml"},
				{NewPath: "dataproducts/agg/test2/product.yaml"},
			},
			warehouseChanges: []WarehouseChange{
				{
					FilePath:   "dataproducts/agg/test1/product.yaml (type: snowflake)",
					FromSize:   "XLARGE",
					ToSize:     "LARGE",
					IsDecrease: true,
				},
				{
					FilePath:   "dataproducts/agg/test2/product.yaml (type: snowflake)",
					FromSize:   "LARGE",
					ToSize:     "MEDIUM",
					IsDecrease: true,
				},
			},
			expectedDecision: shared.Approve,
			expectedReason:   "Auto-approving MR with only 2 warehouse changes",
		},
		{
			name: "mixed warehouse changes - one increase",
			changes: []gitlab.FileChange{
				{NewPath: "dataproducts/agg/test1/product.yaml"},
				{NewPath: "dataproducts/agg/test2/product.yaml"},
			},
			warehouseChanges: []WarehouseChange{
				{
					FilePath:   "dataproducts/agg/test1/product.yaml (type: snowflake)",
					FromSize:   "LARGE",
					ToSize:     "MEDIUM",
					IsDecrease: true,
				},
				{
					FilePath:   "dataproducts/agg/test2/product.yaml (type: snowflake)",
					FromSize:   "MEDIUM",
					ToSize:     "LARGE",
					IsDecrease: false,
				},
			},
			expectedDecision: shared.ManualReview,
			expectedContains: "Warehouse increase detected: MEDIUM → LARGE",
		},
		{
			name: "no warehouse changes in files",
			changes: []gitlab.FileChange{
				{NewPath: "dataproducts/agg/test/product.yaml"},
			},
			warehouseChanges: []WarehouseChange{},
			expectedDecision: shared.Approve,
			expectedContains: "Auto-approving MR with only 1 warehouse changes",
		},
		{
			name: "warehouse and sourcebinding changes",
			changes: []gitlab.FileChange{
				{NewPath: "dataproducts/agg/test/product.yaml"},
				{NewPath: "dataproducts/source/test/sourcebinding.yaml"},
			},
			warehouseChanges: []WarehouseChange{
				{
					FilePath:   "dataproducts/agg/test/product.yaml (type: snowflake)",
					FromSize:   "LARGE",
					ToSize:     "MEDIUM",
					IsDecrease: true,
				},
			},
			expectedDecision: shared.Approve,
			expectedContains: "Auto-approving MR with only 1 warehouse and 1 sourcebinding changes",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockAnalyzer := &MockAnalyzer{
				warehouseChanges: tt.warehouseChanges,
				analyzeError:     tt.analyzerError,
			}
			rule := NewRule(&gitlab.Client{})
			rule.analyzer = mockAnalyzer

			mrCtx := &shared.MRContext{
				ProjectID: 123,
				MRIID:     456,
				Changes:   tt.changes,
			}
			decision, reason := rule.ShouldApprove(mrCtx)
			assert.Equal(t, tt.expectedDecision, decision, "ShouldApprove() decision failed")
			if tt.expectedReason != "" {
				assert.Equal(t, tt.expectedReason, reason, "ShouldApprove() reason failed")
			}
			if tt.expectedContains != "" {
				assert.Contains(t, reason, tt.expectedContains, "ShouldApprove() reason should contain expected text")
			}
		})
	}
}

func TestWarehouseRule_evaluateWarehouseChanges(t *testing.T) {
	rule := NewRule(nil)

	tests := []struct {
		name             string
		changes          []WarehouseChange
		expectedDecision shared.DecisionType
		expectedReason   string
	}{
		{
			name:             "no changes",
			changes:          []WarehouseChange{},
			expectedDecision: shared.Approve,
			expectedReason:   "No warehouse changes detected in dataproduct files",
		},
		{
			name: "single decrease",
			changes: []WarehouseChange{
				{
					FilePath:   "dataproducts/agg/test/product.yaml (type: snowflake)",
					FromSize:   "LARGE",
					ToSize:     "MEDIUM",
					IsDecrease: true,
				},
			},
			expectedDecision: shared.Approve,
			expectedReason:   "All 1 warehouse changes are decreases",
		},
		{
			name: "single increase",
			changes: []WarehouseChange{
				{
					FilePath:   "dataproducts/agg/test/product.yaml (type: snowflake)",
					FromSize:   "MEDIUM",
					ToSize:     "LARGE",
					IsDecrease: false,
				},
			},
			expectedDecision: shared.ManualReview,
			expectedReason:   "Warehouse increase detected: MEDIUM → LARGE in dataproducts/agg/test/product.yaml (type: snowflake)",
		},
		{
			name: "multiple decreases",
			changes: []WarehouseChange{
				{
					FilePath:   "dataproducts/agg/test1/product.yaml (type: snowflake)",
					FromSize:   "XLARGE",
					ToSize:     "LARGE",
					IsDecrease: true,
				},
				{
					FilePath:   "dataproducts/agg/test2/product.yaml (type: snowflake)",
					FromSize:   "LARGE",
					ToSize:     "MEDIUM",
					IsDecrease: true,
				},
				{
					FilePath:   "dataproducts/agg/test3/product.yaml (type: snowflake)",
					FromSize:   "MEDIUM",
					ToSize:     "SMALL",
					IsDecrease: true,
				},
			},
			expectedDecision: shared.Approve,
			expectedReason:   "All 3 warehouse changes are decreases",
		},
		{
			name: "mixed changes - first increase",
			changes: []WarehouseChange{
				{
					FilePath:   "dataproducts/agg/test1/product.yaml (type: snowflake)",
					FromSize:   "MEDIUM",
					ToSize:     "LARGE",
					IsDecrease: false,
				},
				{
					FilePath:   "dataproducts/agg/test2/product.yaml (type: snowflake)",
					FromSize:   "LARGE",
					ToSize:     "MEDIUM",
					IsDecrease: true,
				},
			},
			expectedDecision: shared.ManualReview,
			expectedReason:   "Warehouse increase detected: MEDIUM → LARGE in dataproducts/agg/test1/product.yaml (type: snowflake)",
		},
		{
			name: "mixed changes - later increase",
			changes: []WarehouseChange{
				{
					FilePath:   "dataproducts/agg/test1/product.yaml (type: snowflake)",
					FromSize:   "LARGE",
					ToSize:     "MEDIUM",
					IsDecrease: true,
				},
				{
					FilePath:   "dataproducts/agg/test2/product.yaml (type: snowflake)",
					FromSize:   "SMALL",
					ToSize:     "XLARGE",
					IsDecrease: false,
				},
			},
			expectedDecision: shared.ManualReview,
			expectedReason:   "Warehouse increase detected: SMALL → XLARGE in dataproducts/agg/test2/product.yaml (type: snowflake)",
		},
		{
			name: "new warehouse creation",
			changes: []WarehouseChange{
				{
					FilePath:   "dataproducts/agg/rosettastone/dev/product.yaml (type: user)",
					FromSize:   "",
					ToSize:     "XSMALL",
					IsDecrease: false,
				},
			},
			expectedDecision: shared.ManualReview,
			expectedReason:   "New warehouse creation detected: XSMALL in dataproducts/agg/rosettastone/dev/product.yaml (type: user)",
		},
		{
			name: "multiple new warehouse creation",
			changes: []WarehouseChange{
				{
					FilePath:   "dataproducts/agg/rosettastone/dev/product.yaml (type: user)",
					FromSize:   "",
					ToSize:     "XSMALL",
					IsDecrease: false,
				},
				{
					FilePath:   "dataproducts/agg/rosettastone/dev/product.yaml (type: service_account)",
					FromSize:   "",
					ToSize:     "XSMALL",
					IsDecrease: false,
				},
			},
			expectedDecision: shared.ManualReview,
			expectedReason:   "New warehouse creation detected: XSMALL in dataproducts/agg/rosettastone/dev/product.yaml (type: user)",
		},
		{
			name: "non-warehouse changes detected",
			changes: []WarehouseChange{
				{
					FilePath:   "dataproducts/source/ciam/dev/product.yaml (non-warehouse changes)",
					FromSize:   "N/A",
					ToSize:     "N/A",
					IsDecrease: false,
				},
			},
			expectedDecision: shared.ManualReview,
			expectedReason:   "Non-warehouse changes detected in dataproducts/source/ciam/dev/product.yaml",
		},
		{
			name: "warehouse decrease with non-warehouse changes",
			changes: []WarehouseChange{
				{
					FilePath:   "dataproducts/agg/test/product.yaml (type: snowflake)",
					FromSize:   "LARGE",
					ToSize:     "MEDIUM",
					IsDecrease: true,
				},
				{
					FilePath:   "dataproducts/agg/test/product.yaml (non-warehouse changes)",
					FromSize:   "N/A",
					ToSize:     "N/A",
					IsDecrease: false,
				},
			},
			expectedDecision: shared.ManualReview,
			expectedReason:   "Non-warehouse changes detected in dataproducts/agg/test/product.yaml",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			decision, reason := rule.evaluateWarehouseChanges(tt.changes)
			assert.Equal(t, tt.expectedDecision, decision, "evaluateWarehouseChanges() decision failed")
			assert.Equal(t, tt.expectedReason, reason, "evaluateWarehouseChanges() reason failed")
		})
	}
}

// MockAnalyzer is a test implementation of the AnalyzerInterface
type MockAnalyzer struct {
	warehouseChanges []WarehouseChange
	analyzeError     error
}

func (m *MockAnalyzer) AnalyzeChanges(projectID, mrIID int, changes []gitlab.FileChange) ([]WarehouseChange, error) {
	if m.analyzeError != nil {
		return nil, m.analyzeError
	}
	return m.warehouseChanges, nil
}
