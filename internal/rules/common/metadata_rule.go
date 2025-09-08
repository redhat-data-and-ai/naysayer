package common

import (
	"strings"

	"github.com/redhat-data-and-ai/naysayer/internal/rules/shared"
)

// MetadataRule auto-approves changes to documentation and metadata files
type MetadataRule struct {
	*BaseRule
	*FileTypeMatcher
	*ValidationHelper
}

// NewMetadataRule creates a new metadata rule instance
func NewMetadataRule() *MetadataRule {
	return &MetadataRule{
		BaseRule:         NewBaseRule("metadata_rule", "Auto-approves documentation and metadata file changes"),
		FileTypeMatcher:  NewFileTypeMatcher(),
		ValidationHelper: NewValidationHelper(),
	}
}

// ValidateLines validates lines for metadata files
func (r *MetadataRule) ValidateLines(filePath string, fileContent string, lineRanges []shared.LineRange) (shared.DecisionType, string) {
	// Check if this is a metadata/documentation file
	if r.isMetadataFile(filePath) {
		return r.CreateApprovalResult(r.getApprovalReason(filePath))
	}

	// Check if this is a section-based validation for DBT metadata
	if r.isDBTMetadataSection(filePath, fileContent) {
		return r.CreateApprovalResult("Auto-approved: DBT metadata configuration changes are safe")
	}

	return r.CreateManualReviewResult("Not a metadata file - requires manual review")
}

// GetCoveredLines returns line ranges this rule covers
func (r *MetadataRule) GetCoveredLines(filePath string, fileContent string) []shared.LineRange {
	// Cover documentation files entirely
	if r.isMetadataFile(filePath) {
		return r.GetFullFileCoverage(filePath, fileContent)
	}

	// For section-based validation, return a placeholder to indicate participation
	if r.isDBTMetadataSection(filePath, fileContent) {
		// This will be handled by section-based validation
		return []shared.LineRange{
			{
				StartLine: 1,
				EndLine:   1,
				FilePath:  filePath,
			},
		}
	}

	return []shared.LineRange{}
}

// isMetadataFile checks if a file is a metadata/documentation file
func (r *MetadataRule) isMetadataFile(filePath string) bool {
	if filePath == "" {
		return false
	}

	lowerPath := strings.ToLower(filePath)

	// Documentation files
	if r.IsDocumentationFile(filePath) {
		return true
	}

	// Additional metadata file patterns
	return strings.HasSuffix(lowerPath, ".md") ||
		strings.HasSuffix(lowerPath, ".txt") ||
		strings.HasSuffix(lowerPath, "changelog") ||
		strings.HasSuffix(lowerPath, "changelog.md") ||
		strings.HasSuffix(lowerPath, "license") ||
		strings.HasSuffix(lowerPath, "license.md") ||
		strings.HasSuffix(lowerPath, "authors") ||
		strings.HasSuffix(lowerPath, "authors.md") ||
		strings.HasSuffix(lowerPath, "contributors.md") ||
		strings.HasSuffix(lowerPath, "codeowners") ||
		strings.HasSuffix(lowerPath, ".codeowners") ||
		strings.Contains(lowerPath, "docs/") ||
		strings.Contains(lowerPath, "documentation/")
}

// isDBTMetadataSection checks if this is DBT metadata configuration
func (r *MetadataRule) isDBTMetadataSection(filePath string, fileContent string) bool {
	// This is for section-based validation of DBT metadata in product.yaml files
	if !r.IsProductFile(filePath) {
		return false
	}

	// Check if content contains DBT-related metadata
	lowerContent := strings.ToLower(fileContent)
	return strings.Contains(lowerContent, "service_account") &&
		strings.Contains(lowerContent, "dbt")
}

// getApprovalReason returns a specific approval reason based on file type
func (r *MetadataRule) getApprovalReason(filePath string) string {
	lowerPath := strings.ToLower(filePath)

	switch {
	case strings.HasSuffix(lowerPath, "readme.md"):
		return "Auto-approved: README file changes are documentation updates"
	case strings.HasSuffix(lowerPath, "data_elements.md"):
		return "Auto-approved: Data elements documentation changes are metadata updates"
	case strings.HasSuffix(lowerPath, "promotion_checklist.md"):
		return "Auto-approved: Promotion checklist changes are process documentation"
	case strings.HasSuffix(lowerPath, "developers.yaml") || strings.HasSuffix(lowerPath, "developers.yml"):
		return "Auto-approved: Developer configuration changes are team metadata"
	case strings.HasSuffix(lowerPath, "changelog") || strings.HasSuffix(lowerPath, "changelog.md"):
		return "Auto-approved: Changelog updates are version history metadata"
	case strings.HasSuffix(lowerPath, "license") || strings.HasSuffix(lowerPath, "license.md"):
		return "Auto-approved: License file changes are legal metadata"
	case strings.Contains(lowerPath, "docs/") || strings.Contains(lowerPath, "documentation/"):
		return "Auto-approved: Documentation directory changes are content updates"
	case strings.HasSuffix(lowerPath, ".md"):
		return "Auto-approved: Markdown documentation changes are generally safe"
	case strings.HasSuffix(lowerPath, ".txt"):
		return "Auto-approved: Text file changes are documentation updates"
	default:
		return "Auto-approved: Metadata file changes are generally safe"
	}
}
