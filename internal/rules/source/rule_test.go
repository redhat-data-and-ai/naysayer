package source

import (
	"testing"

	"github.com/redhat-data-and-ai/naysayer/internal/gitlab"
	"github.com/redhat-data-and-ai/naysayer/internal/rules/shared"
	"github.com/stretchr/testify/assert"
)

func TestSourceBindingRule_Name(t *testing.T) {
	rule := NewRule(nil)
	assert.Equal(t, "source_binding_rule", rule.Name())
}

func TestSourceBindingRule_Description(t *testing.T) {
	rule := NewRule(nil)
	expected := "Auto-approves MRs with only dataverse-safe files (warehouse/sourcebinding)"
	assert.Equal(t, expected, rule.Description())
}

func TestSourceBindingRule_Applies(t *testing.T) {
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
			name: "only sourcebinding files",
			changes: []gitlab.FileChange{
				{NewPath: "dataproducts/source/marketo/sandbox/sourcebinding.yaml"},
				{NewPath: "dataproducts/source/sfsales/prod/sourcebinding.yaml"},
			},
			expected: true,
		},
		{
			name: "mixed sourcebinding and other files",
			changes: []gitlab.FileChange{
				{NewPath: "dataproducts/source/marketo/sandbox/sourcebinding.yaml"},
				{NewPath: "README.md"},
			},
			expected: true, // Rule applies because there are sourcebinding files
		},
		{
			name: "only non-sourcebinding files",
			changes: []gitlab.FileChange{
				{NewPath: "README.md"},
				{NewPath: "config/settings.yaml"},
			},
			expected: false,
		},
		{
			name: "sourcebinding file deletion",
			changes: []gitlab.FileChange{
				{OldPath: "dataproducts/source/old/sourcebinding.yaml", NewPath: ""},
			},
			expected: true,
		},
		{
			name: "sourcebinding file rename",
			changes: []gitlab.FileChange{
				{
					OldPath: "dataproducts/source/old/sourcebinding.yaml",
					NewPath: "dataproducts/source/new/sourcebinding.yaml",
				},
			},
			expected: true,
		},
		{
			name: "case insensitive detection",
			changes: []gitlab.FileChange{
				{NewPath: "dataproducts/source/test/SOURCEBINDING.YAML"},
			},
			expected: true,
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

func TestSourceBindingRule_isSourceBindingFile(t *testing.T) {
	rule := NewRule(nil)

	tests := []struct {
		name     string
		path     string
		expected bool
	}{
		{
			name:     "sourcebinding.yaml",
			path:     "dataproducts/source/marketo/sandbox/sourcebinding.yaml",
			expected: true,
		},
		{
			name:     "sourcebinding.yml",
			path:     "dataproducts/source/hellosource/prod/sourcebinding.yml",
			expected: true,
		},
		{
			name:     "uppercase extension",
			path:     "dataproducts/source/test/sourcebinding.YAML",
			expected: true,
		},
		{
			name:     "contains sourcebinding",
			path:     "dataproducts/source/test/custom-sourcebinding-config.yaml",
			expected: true,
		},
		{
			name:     "product.yaml file",
			path:     "dataproducts/agg/bookings/prod/product.yaml",
			expected: false,
		},
		{
			name:     "README file",
			path:     "README.md",
			expected: false,
		},
		{
			name:     "empty path",
			path:     "",
			expected: false,
		},
		{
			name:     "similar but not exact",
			path:     "dataproducts/source/test/source-binding-config.yaml",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual := rule.isSourceBindingFile(tt.path)
			assert.Equal(t, tt.expected, actual, "isSourceBindingFile() failed")
		})
	}
}

func TestSourceBindingRule_ShouldApprove(t *testing.T) {
	// Create a rule with nil client to test logic without GitLab interactions
	rule := NewRule(nil)

	tests := []struct {
		name             string
		changes          []gitlab.FileChange
		expectedDecision shared.DecisionType
		expectedReason   string
	}{
		{
			name:             "nil client",
			changes:          []gitlab.FileChange{{NewPath: "dataproducts/source/test/sourcebinding.yaml"}},
			expectedDecision: shared.ManualReview,
			expectedReason:   "GitLab token not configured - cannot analyze sourceBinding files",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
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

func TestSourceBindingRule_ShouldApprove_WithClient(t *testing.T) {
	// Create a rule with a non-nil client to test the actual logic
	// We don't need a full mock since the rule uses shared functions now
	rule := NewRule(&gitlab.Client{})

	tests := []struct {
		name             string
		changes          []gitlab.FileChange
		expectedDecision shared.DecisionType
		expectedMessage  string
	}{
		{
			name:             "no changes",
			changes:          []gitlab.FileChange{},
			expectedDecision: shared.Approve,
			expectedMessage:  "No dataverse file changes detected",
		},
		{
			name: "only sourcebinding changes",
			changes: []gitlab.FileChange{
				{NewPath: "dataproducts/source/marketo/sandbox/sourcebinding.yaml"},
				{NewPath: "dataproducts/source/sfsales/prod/sourcebinding.yaml"},
			},
			expectedDecision: shared.Approve,
			expectedMessage:  "Auto-approving MR with only 2 sourcebinding changes",
		},
		{
			name: "mixed dataverse changes",
			changes: []gitlab.FileChange{
				{NewPath: "dataproducts/source/marketo/sandbox/sourcebinding.yaml"},
				{NewPath: "dataproducts/agg/bookings/prod/product.yaml"},
			},
			expectedDecision: shared.Approve,
			expectedMessage:  "Auto-approving MR with only 1 warehouse and 1 sourcebinding changes",
		},
		{
			name: "mixed with non-dataverse files",
			changes: []gitlab.FileChange{
				{NewPath: "dataproducts/source/marketo/sandbox/sourcebinding.yaml"},
				{NewPath: "README.md"},
			},
			expectedDecision: shared.ManualReview,
			expectedMessage:  "MR contains non-dataverse file changes",
		},
		{
			name: "only non-dataverse files",
			changes: []gitlab.FileChange{
				{NewPath: "README.md"},
				{NewPath: "config/settings.yaml"},
			},
			expectedDecision: shared.Approve,
			expectedMessage:  "No dataverse file changes detected",
		},
		{
			name: "single sourcebinding change",
			changes: []gitlab.FileChange{
				{NewPath: "dataproducts/source/hellosource/dev/sourcebinding.yaml"},
			},
			expectedDecision: shared.Approve,
			expectedMessage:  "Auto-approving MR with only 1 sourcebinding changes",
		},
		{
			name: "sourcebinding file deletion",
			changes: []gitlab.FileChange{
				{OldPath: "dataproducts/source/old/sourcebinding.yaml", NewPath: ""},
			},
			expectedDecision: shared.Approve,
			expectedMessage:  "Auto-approving MR with only 1 sourcebinding changes",
		},
		{
			name: "only warehouse changes",
			changes: []gitlab.FileChange{
				{NewPath: "dataproducts/agg/bookings/prod/product.yaml"},
				{NewPath: "dataproducts/agg/costops/dev/product.yaml"},
			},
			expectedDecision: shared.Approve,
			expectedMessage:  "Auto-approving MR with only 2 warehouse changes",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mrCtx := &shared.MRContext{
				ProjectID: 123,
				MRIID:     456,
				Changes:   tt.changes,
			}
			decision, message := rule.ShouldApprove(mrCtx)
			assert.Equal(t, tt.expectedDecision, decision, "ShouldApprove() decision failed")
			assert.Equal(t, tt.expectedMessage, message, "ShouldApprove() message failed")
		})
	}
}
