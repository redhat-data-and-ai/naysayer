package rules

import (
	"strings"
	"github.com/redhat-data-and-ai/naysayer/internal/rules/shared"
)

// DocumentationAutoApprovalRule automatically approves changes to documentation files.
// This rule provides immediate value by auto-approving safe documentation updates.
type DocumentationAutoApprovalRule struct {
	name string
}

// NewDocumentationAutoApprovalRule creates a new documentation auto-approval rule
func NewDocumentationAutoApprovalRule() *DocumentationAutoApprovalRule {
	return &DocumentationAutoApprovalRule{
		name: "documentation_auto_approval",
	}
}

// Name returns the unique identifier for this rule
func (r *DocumentationAutoApprovalRule) Name() string {
	return r.name
}

// Description returns a human-readable description of what this rule does
func (r *DocumentationAutoApprovalRule) Description() string {
	return "Automatically approves changes to documentation files (README.md, data_elements.md, promotion_checklist.md, developers.yaml)"
}

// GetCoveredLines returns which line ranges this rule validates in a file
// For documentation files, we approve the entire file, so return full coverage
func (r *DocumentationAutoApprovalRule) GetCoveredLines(filePath string, fileContent string) []shared.LineRange {
	if !r.isDocumentationFile(filePath) {
		return []shared.LineRange{}
	}
	
	totalLines := shared.CountLines(fileContent)
	if totalLines == 0 {
		return []shared.LineRange{}
	}
	
	return []shared.LineRange{
		{
			StartLine: 1,
			EndLine:   totalLines,
			FilePath:  filePath,
		},
	}
}

// ValidateLines validates the specified line ranges using file-level logic
func (r *DocumentationAutoApprovalRule) ValidateLines(filePath string, fileContent string, lineRanges []shared.LineRange) (shared.DecisionType, string) {
	if r.isDocumentationFile(filePath) {
		return shared.Approve, "Documentation updates are automatically approved"
	}
	return shared.ManualReview, "Not a documentation file"
}

// isDocumentationFile checks if a file path is a documentation file that should be auto-approved
func (r *DocumentationAutoApprovalRule) isDocumentationFile(filePath string) bool {
	// Convert to lowercase for case-insensitive comparison
	lowerPath := strings.ToLower(filePath)
	
	// Check for documentation file patterns
	return strings.HasSuffix(lowerPath, "readme.md") ||
		   strings.HasSuffix(lowerPath, "data_elements.md") ||
		   strings.HasSuffix(lowerPath, "promotion_checklist.md") ||
		   strings.HasSuffix(lowerPath, "developers.yaml")
}