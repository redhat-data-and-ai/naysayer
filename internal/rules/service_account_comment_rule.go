package rules

import (
	"strings"
	"github.com/redhat-data-and-ai/naysayer/internal/rules/shared"
)

// ServiceAccountCommentRule demonstrates field-level auto-approval concept for service account files.
// NOTE: This rule currently approves ALL service account file changes as a placeholder.
// A real implementation would need GitLab API access to compare old vs new file content
// to determine if only comment/description fields changed.
type ServiceAccountCommentRule struct {
	name string
}

// NewServiceAccountCommentRule creates a new service account comment auto-approval rule
func NewServiceAccountCommentRule() *ServiceAccountCommentRule {
	return &ServiceAccountCommentRule{
		name: "service_account_comment_rule",
	}
}

// Name returns the unique identifier for this rule
func (r *ServiceAccountCommentRule) Name() string {
	return r.name
}

// Description returns a human-readable description of what this rule does
func (r *ServiceAccountCommentRule) Description() string {
	return "Placeholder for field-level auto-approval of service account comment changes (currently requires manual review)"
}

// GetCoveredLines returns which line ranges this rule validates in a file
func (r *ServiceAccountCommentRule) GetCoveredLines(filePath string, fileContent string) []shared.LineRange {
	// This rule is currently a placeholder - it doesn't cover any lines
	// because we can't reliably detect comment-only changes without comparing old vs new content
	return []shared.LineRange{}
}

// ValidateLines validates the specified line ranges using file-level logic
func (r *ServiceAccountCommentRule) ValidateLines(filePath string, fileContent string, lineRanges []shared.LineRange) (shared.DecisionType, string) {
	if !r.isServiceAccountFile(filePath) {
		return shared.ManualReview, "Not a service account file"
	}

	// This is a placeholder rule - we can't reliably detect comment-only changes
	// without access to both old and new file content via GitLab API
	return shared.ManualReview, "Service account changes require manual review (comment detection not implemented)"
}

// isServiceAccountFile checks if a file path is a service account file
func (r *ServiceAccountCommentRule) isServiceAccountFile(filePath string) bool {
	// Service account files are in serviceaccounts/ directory and end with _appuser.yaml or _appuser.yml
	lowerPath := strings.ToLower(filePath)
	return strings.Contains(lowerPath, "serviceaccounts/") && 
		   (strings.HasSuffix(lowerPath, "_appuser.yaml") || strings.HasSuffix(lowerPath, "_appuser.yml"))
}

// TODO: To implement proper comment-only detection, this rule would need:
// 1. Access to GitLab API to fetch old file content
// 2. YAML parsing to compare old vs new field values
// 3. Logic to determine if only comment/description fields changed
//
// Example implementation would look like:
// func (r *ServiceAccountCommentRule) isCommentOnlyChange(oldContent, newContent string) bool {
//     oldYaml := parseYAML(oldContent)
//     newYaml := parseYAML(newContent)
//     return onlyCommentFieldsChanged(oldYaml, newYaml)
// }