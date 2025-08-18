package shared

import (
	"fmt"
	"testing"

	"github.com/redhat-data-and-ai/naysayer/internal/gitlab"
	"github.com/stretchr/testify/assert"
)

func TestIsDataProductFile(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		expected bool
	}{
		// Data product files
		{
			name:     "data product file - product.yaml",
			path:     "dataproducts/aggregate/bookingsmaster/prod/product.yaml",
			expected: true,
		},
		{
			name:     "data product file - product.yml",
			path:     "dataproducts/aggregate/costops/dev/product.yml",
			expected: true,
		},
		{
			name:     "data product file - uppercase",
			path:     "dataproducts/aggregate/spa/PRODUCT.YAML",
			expected: true,
		},

		// Non-data product files
		{
			name:     "README file",
			path:     "README.md",
			expected: false,
		},
		{
			name:     "random YAML file",
			path:     "config/settings.yaml",
			expected: false,
		},
		{
			name:     "developers file",
			path:     "dataproducts/aggregate/bookingsmaster/developers.yaml",
			expected: false,
		},
		{
			name:     "similar but not exact match",
			path:     "dataproducts/source/test/product-config.yaml",
			expected: false,
		},

		// Edge cases
		{
			name:     "empty path",
			path:     "",
			expected: false,
		},
		{
			name:     "path with spaces",
			path:     "data products/source/test/product.yaml",
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual := IsDataProductFile(tt.path)
			assert.Equal(t, tt.expected, actual, "IsDataProductFile() failed")
		})
	}
}

func TestAnalyzeDataProductChanges(t *testing.T) {
	tests := []struct {
		name     string
		changes  []gitlab.FileChange
		expected int
	}{
		{
			name:     "no changes",
			changes:  []gitlab.FileChange{},
			expected: 0,
		},
		{
			name: "only data product changes",
			changes: []gitlab.FileChange{
				{NewPath: "dataproducts/agg/bookings/prod/product.yaml"},
				{NewPath: "dataproducts/agg/costops/dev/product.yaml"},
			},
			expected: 2,
		},
		{
			name: "mixed data product and non-data product files",
			changes: []gitlab.FileChange{
				{NewPath: "dataproducts/agg/spa/prod/product.yaml"},
				{NewPath: "README.md"},
				{NewPath: "dataproducts/source/marketo/sandbox/product.yaml"},
			},
			expected: 2,
		},
		{
			name: "non-data product files",
			changes: []gitlab.FileChange{
				{NewPath: "README.md"},
				{NewPath: "config/settings.yaml"},
				{NewPath: "dataproducts/agg/bookings/developers.yaml"},
			},
			expected: 0,
		},
		{
			name: "file deletions - old path detection",
			changes: []gitlab.FileChange{
				{OldPath: "dataproducts/agg/old/product.yaml", NewPath: ""},
			},
			expected: 1,
		},
		{
			name: "file renames - both paths data product",
			changes: []gitlab.FileChange{
				{
					OldPath: "dataproducts/agg/old/product.yaml",
					NewPath: "dataproducts/agg/new/product.yaml",
				},
			},
			expected: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual := CountDataProductChanges(tt.changes)
			assert.Equal(t, tt.expected, actual, "CountDataProductChanges() failed")
		})
	}
}

func TestAreAllDataProductSafe(t *testing.T) {
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
			name: "all data product files",
			changes: []gitlab.FileChange{
				{NewPath: "dataproducts/agg/bookings/prod/product.yaml"},
				{NewPath: "dataproducts/agg/costops/dev/product.yaml"},
			},
			expected: true,
		},
		{
			name: "contains non-data product file",
			changes: []gitlab.FileChange{
				{NewPath: "dataproducts/agg/spa/prod/product.yaml"},
				{NewPath: "README.md"},
			},
			expected: false,
		},
		{
			name: "all non-data product files",
			changes: []gitlab.FileChange{
				{NewPath: "README.md"},
				{NewPath: "config/settings.yaml"},
			},
			expected: false,
		},
		{
			name: "file deletion - old path is data product",
			changes: []gitlab.FileChange{
				{OldPath: "dataproducts/agg/old/product.yaml", NewPath: ""},
			},
			expected: true,
		},
		{
			name: "file addition - new path is data product",
			changes: []gitlab.FileChange{
				{OldPath: "", NewPath: "dataproducts/source/new/product.yaml"},
			},
			expected: true,
		},
		{
			name: "mixed safe and unsafe changes",
			changes: []gitlab.FileChange{
				{NewPath: "dataproducts/agg/spa/prod/product.yaml"},
				{NewPath: "scripts/deploy.sh"},
				{NewPath: "dataproducts/source/marketo/sandbox/product.yaml"},
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual := AreAllDataProductSafe(tt.changes)
			assert.Equal(t, tt.expected, actual, "AreAllDataProductSafe() failed")
		})
	}
}

func TestBuildDataProductApprovalMessage(t *testing.T) {
	tests := []struct {
		name        string
		changeCount int
		expected    string
	}{
		{
			name:        "no changes",
			changeCount: 0,
			expected:    "Auto-approving MR with no data product file changes",
		},
		{
			name:        "single change",
			changeCount: 1,
			expected:    "Auto-approving MR with only 1 data product changes",
		},
		{
			name:        "multiple changes",
			changeCount: 3,
			expected:    "Auto-approving MR with only 3 data product changes",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual := BuildDataProductApprovalMessage(tt.changeCount)
			assert.Equal(t, tt.expected, actual, "BuildDataProductApprovalMessage() failed")
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

// Helper functions for the tests

// CountDataProductChanges counts the number of data product file changes
func CountDataProductChanges(changes []gitlab.FileChange) int {
	count := 0
	for _, change := range changes {
		// Check both old and new paths for file additions, modifications, and deletions
		if IsDataProductFile(change.NewPath) || IsDataProductFile(change.OldPath) {
			count++
		}
	}
	return count
}

// AreAllDataProductSafe returns true if all changes are data product files
func AreAllDataProductSafe(changes []gitlab.FileChange) bool {
	if len(changes) == 0 {
		return true
	}

	for _, change := range changes {
		// For each change, check if at least one of the paths is a data product file
		if !IsDataProductFile(change.NewPath) && !IsDataProductFile(change.OldPath) {
			return false
		}
	}
	return true
}

// BuildDataProductApprovalMessage creates an approval message for data product changes
func BuildDataProductApprovalMessage(changeCount int) string {
	if changeCount == 0 {
		return "Auto-approving MR with no data product file changes"
	}
	return fmt.Sprintf("Auto-approving MR with only %d data product changes", changeCount)
}
