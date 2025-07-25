package shared

import (
	"testing"

	"github.com/redhat-data-and-ai/naysayer/internal/gitlab"
	"github.com/stretchr/testify/assert"
)

func TestIsDataverseFile(t *testing.T) {
	tests := []struct {
		name         string
		path         string
		expectedSafe bool
		expectedType DataverseFileType
	}{
		// Warehouse files
		{
			name:         "warehouse file - product.yaml",
			path:         "dataproducts/aggregate/bookingsmaster/prod/product.yaml",
			expectedSafe: true,
			expectedType: WarehouseFile,
		},
		{
			name:         "warehouse file - product.yml",
			path:         "dataproducts/aggregate/costops/dev/product.yml",
			expectedSafe: true,
			expectedType: WarehouseFile,
		},
		{
			name:         "warehouse file - uppercase",
			path:         "dataproducts/aggregate/spa/PRODUCT.YAML",
			expectedSafe: true,
			expectedType: WarehouseFile,
		},

		// Sourcebinding files
		{
			name:         "sourcebinding file - sourcebinding.yaml",
			path:         "dataproducts/source/marketo/sandbox/sourcebinding.yaml",
			expectedSafe: true,
			expectedType: SourceBindingFile,
		},
		{
			name:         "sourcebinding file - sourcebinding.yml",
			path:         "dataproducts/source/hellosource/prod/sourcebinding.yml",
			expectedSafe: true,
			expectedType: SourceBindingFile,
		},
		{
			name:         "sourcebinding file - uppercase",
			path:         "dataproducts/source/sfsales/SOURCEBINDING.YAML",
			expectedSafe: true,
			expectedType: SourceBindingFile,
		},

		// Non-dataverse files
		{
			name:         "README file",
			path:         "README.md",
			expectedSafe: false,
			expectedType: "",
		},
		{
			name:         "random YAML file",
			path:         "config/settings.yaml",
			expectedSafe: false,
			expectedType: "",
		},
		{
			name:         "developers file",
			path:         "dataproducts/aggregate/bookingsmaster/developers.yaml",
			expectedSafe: false,
			expectedType: "",
		},
		{
			name:         "similar but not exact match",
			path:         "dataproducts/source/test/product-config.yaml",
			expectedSafe: false,
			expectedType: "",
		},

		// Edge cases
		{
			name:         "empty path",
			path:         "",
			expectedSafe: false,
			expectedType: "",
		},
		{
			name:         "path with spaces",
			path:         "data products/source/test/sourcebinding.yaml",
			expectedSafe: true,
			expectedType: SourceBindingFile,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actualSafe, actualType := IsDataverseFile(tt.path)
			assert.Equal(t, tt.expectedSafe, actualSafe, "IsDataverseFile() safety check failed")
			assert.Equal(t, tt.expectedType, actualType, "IsDataverseFile() type detection failed")
		})
	}
}

func TestAnalyzeDataverseChanges(t *testing.T) {
	tests := []struct {
		name     string
		changes  []gitlab.FileChange
		expected map[DataverseFileType]int
	}{
		{
			name:     "no changes",
			changes:  []gitlab.FileChange{},
			expected: map[DataverseFileType]int{},
		},
		{
			name: "only warehouse changes",
			changes: []gitlab.FileChange{
				{NewPath: "dataproducts/agg/bookings/prod/product.yaml"},
				{NewPath: "dataproducts/agg/costops/dev/product.yaml"},
			},
			expected: map[DataverseFileType]int{
				WarehouseFile: 2,
			},
		},
		{
			name: "only sourcebinding changes",
			changes: []gitlab.FileChange{
				{NewPath: "dataproducts/source/marketo/sandbox/sourcebinding.yaml"},
				{NewPath: "dataproducts/source/sfsales/prod/sourcebinding.yaml"},
				{NewPath: "dataproducts/source/hellosource/dev/sourcebinding.yaml"},
			},
			expected: map[DataverseFileType]int{
				SourceBindingFile: 3,
			},
		},
		{
			name: "mixed warehouse and sourcebinding changes",
			changes: []gitlab.FileChange{
				{NewPath: "dataproducts/agg/spa/prod/product.yaml"},
				{NewPath: "dataproducts/source/marketo/sandbox/sourcebinding.yaml"},
				{NewPath: "dataproducts/agg/forecasting/dev/product.yaml"},
				{NewPath: "dataproducts/source/orders/prod/sourcebinding.yaml"},
			},
			expected: map[DataverseFileType]int{
				WarehouseFile:     2,
				SourceBindingFile: 2,
			},
		},
		{
			name: "non-dataverse files",
			changes: []gitlab.FileChange{
				{NewPath: "README.md"},
				{NewPath: "config/settings.yaml"},
				{NewPath: "dataproducts/agg/bookings/developers.yaml"},
			},
			expected: map[DataverseFileType]int{},
		},
		{
			name: "mixed dataverse and non-dataverse files",
			changes: []gitlab.FileChange{
				{NewPath: "dataproducts/agg/spa/prod/product.yaml"},
				{NewPath: "README.md"},
				{NewPath: "dataproducts/source/marketo/sandbox/sourcebinding.yaml"},
			},
			expected: map[DataverseFileType]int{
				WarehouseFile:     1,
				SourceBindingFile: 1,
			},
		},
		{
			name: "file deletions - old path detection",
			changes: []gitlab.FileChange{
				{OldPath: "dataproducts/agg/old/product.yaml", NewPath: ""},
				{OldPath: "dataproducts/source/old/sourcebinding.yaml", NewPath: ""},
			},
			expected: map[DataverseFileType]int{
				WarehouseFile:     1,
				SourceBindingFile: 1,
			},
		},
		{
			name: "file renames - both paths same type",
			changes: []gitlab.FileChange{
				{
					OldPath: "dataproducts/agg/old/product.yaml",
					NewPath: "dataproducts/agg/new/product.yaml",
				},
			},
			expected: map[DataverseFileType]int{
				WarehouseFile: 1,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual := AnalyzeDataverseChanges(tt.changes)
			assert.Equal(t, tt.expected, actual, "AnalyzeDataverseChanges() failed")
		})
	}
}

func TestAreAllDataverseSafe(t *testing.T) {
	tests := []struct {
		name     string
		changes  []gitlab.FileChange
		expected bool
	}{
		{
			name:     "no changes",
			changes:  []gitlab.FileChange{},
			expected: true,
		},
		{
			name: "all warehouse files",
			changes: []gitlab.FileChange{
				{NewPath: "dataproducts/agg/bookings/prod/product.yaml"},
				{NewPath: "dataproducts/agg/costops/dev/product.yaml"},
			},
			expected: true,
		},
		{
			name: "all sourcebinding files",
			changes: []gitlab.FileChange{
				{NewPath: "dataproducts/source/marketo/sandbox/sourcebinding.yaml"},
				{NewPath: "dataproducts/source/sfsales/prod/sourcebinding.yaml"},
			},
			expected: true,
		},
		{
			name: "mixed warehouse and sourcebinding",
			changes: []gitlab.FileChange{
				{NewPath: "dataproducts/agg/spa/prod/product.yaml"},
				{NewPath: "dataproducts/source/marketo/sandbox/sourcebinding.yaml"},
			},
			expected: true,
		},
		{
			name: "contains non-dataverse file",
			changes: []gitlab.FileChange{
				{NewPath: "dataproducts/agg/spa/prod/product.yaml"},
				{NewPath: "README.md"},
			},
			expected: false,
		},
		{
			name: "all non-dataverse files",
			changes: []gitlab.FileChange{
				{NewPath: "README.md"},
				{NewPath: "config/settings.yaml"},
			},
			expected: false,
		},
		{
			name: "file deletion - old path is dataverse",
			changes: []gitlab.FileChange{
				{OldPath: "dataproducts/agg/old/product.yaml", NewPath: ""},
			},
			expected: true,
		},
		{
			name: "file addition - new path is dataverse",
			changes: []gitlab.FileChange{
				{OldPath: "", NewPath: "dataproducts/source/new/sourcebinding.yaml"},
			},
			expected: true,
		},
		{
			name: "mixed safe and unsafe changes",
			changes: []gitlab.FileChange{
				{NewPath: "dataproducts/agg/spa/prod/product.yaml"},
				{NewPath: "scripts/deploy.sh"},
				{NewPath: "dataproducts/source/marketo/sandbox/sourcebinding.yaml"},
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual := AreAllDataverseSafe(tt.changes)
			assert.Equal(t, tt.expected, actual, "AreAllDataverseSafe() failed")
		})
	}
}

func TestBuildDataverseApprovalMessage(t *testing.T) {
	tests := []struct {
		name      string
		fileTypes map[DataverseFileType]int
		expected  string
	}{
		{
			name:      "no changes",
			fileTypes: map[DataverseFileType]int{},
			expected:  "Auto-approving MR with no dataverse file changes",
		},
		{
			name: "single warehouse change",
			fileTypes: map[DataverseFileType]int{
				WarehouseFile: 1,
			},
			expected: "Auto-approving MR with only 1 warehouse changes",
		},
		{
			name: "multiple warehouse changes",
			fileTypes: map[DataverseFileType]int{
				WarehouseFile: 3,
			},
			expected: "Auto-approving MR with only 3 warehouse changes",
		},
		{
			name: "single sourcebinding change",
			fileTypes: map[DataverseFileType]int{
				SourceBindingFile: 1,
			},
			expected: "Auto-approving MR with only 1 sourcebinding changes",
		},
		{
			name: "multiple sourcebinding changes",
			fileTypes: map[DataverseFileType]int{
				SourceBindingFile: 5,
			},
			expected: "Auto-approving MR with only 5 sourcebinding changes",
		},
		{
			name: "mixed changes - warehouse and sourcebinding",
			fileTypes: map[DataverseFileType]int{
				WarehouseFile:     2,
				SourceBindingFile: 3,
			},
			expected: "Auto-approving MR with only 2 warehouse and 3 sourcebinding changes",
		},
		{
			name: "mixed changes - single of each",
			fileTypes: map[DataverseFileType]int{
				WarehouseFile:     1,
				SourceBindingFile: 1,
			},
			expected: "Auto-approving MR with only 1 warehouse and 1 sourcebinding changes",
		},
		{
			name: "zero values in map (should be ignored)",
			fileTypes: map[DataverseFileType]int{
				WarehouseFile:     0,
				SourceBindingFile: 2,
			},
			expected: "Auto-approving MR with only 2 sourcebinding changes",
		},
		{
			name: "all zero values",
			fileTypes: map[DataverseFileType]int{
				WarehouseFile:     0,
				SourceBindingFile: 0,
			},
			expected: "Auto-approving MR with only no changes",
		},
		{
			name: "nil map (should behave like empty)",
			fileTypes: nil,
			expected: "Auto-approving MR with no dataverse file changes",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual := BuildDataverseApprovalMessage(tt.fileTypes)
			assert.Equal(t, tt.expected, actual, "BuildDataverseApprovalMessage() failed")
		})
	}
}

// Test helper functions

func TestIsDraftMR(t *testing.T) {
	tests := []struct {
		name     string
		mrCtx    *MRContext
		expected bool
	}{
		{
			name:     "nil MR info",
			mrCtx:    &MRContext{MRInfo: nil},
			expected: false,
		},
		{
			name: "draft in title",
			mrCtx: &MRContext{
				MRInfo: &gitlab.MRInfo{Title: "Draft: Add new feature"},
			},
			expected: true,
		},
		{
			name: "WIP in title",
			mrCtx: &MRContext{
				MRInfo: &gitlab.MRInfo{Title: "WIP: Work in progress"},
			},
			expected: true,
		},
		{
			name: "draft prefix",
			mrCtx: &MRContext{
				MRInfo: &gitlab.MRInfo{Title: "draft: lowercase prefix"},
			},
			expected: true,
		},
		{
			name: "wip prefix",
			mrCtx: &MRContext{
				MRInfo: &gitlab.MRInfo{Title: "wip: lowercase prefix"},
			},
			expected: true,
		},
		{
			name: "normal title",
			mrCtx: &MRContext{
				MRInfo: &gitlab.MRInfo{Title: "Add new feature"},
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual := IsDraftMR(tt.mrCtx)
			assert.Equal(t, tt.expected, actual, "IsDraftMR() failed")
		})
	}
}

func TestIsAutomatedUser(t *testing.T) {
	tests := []struct {
		name     string
		mrCtx    *MRContext
		expected bool
	}{
		{
			name:     "nil MR info",
			mrCtx:    &MRContext{MRInfo: nil},
			expected: false,
		},
		{
			name: "dependabot user",
			mrCtx: &MRContext{
				MRInfo: &gitlab.MRInfo{Author: "dependabot[bot]"},
			},
			expected: true,
		},
		{
			name: "renovate user",
			mrCtx: &MRContext{
				MRInfo: &gitlab.MRInfo{Author: "renovate-bot"},
			},
			expected: true,
		},
		{
			name: "greenkeeper user",
			mrCtx: &MRContext{
				MRInfo: &gitlab.MRInfo{Author: "greenkeeper"},
			},
			expected: true,
		},
		{
			name: "snyk-bot user",
			mrCtx: &MRContext{
				MRInfo: &gitlab.MRInfo{Author: "snyk-bot"},
			},
			expected: true,
		},
		{
			name: "human user",
			mrCtx: &MRContext{
				MRInfo: &gitlab.MRInfo{Author: "john.doe"},
			},
			expected: false,
		},
		{
			name: "case insensitive check",
			mrCtx: &MRContext{
				MRInfo: &gitlab.MRInfo{Author: "DEPENDABOT"},
			},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual := IsAutomatedUser(tt.mrCtx)
			assert.Equal(t, tt.expected, actual, "IsAutomatedUser() failed")
		})
	}
}
