package serviceaccount

import (
	"fmt"
	"strings"
)

// ScopingValidator handles data product scoping validation
type ScopingValidator struct{}

// NewScopingValidator creates a new scoping validator
func NewScopingValidator() *ScopingValidator {
	return &ScopingValidator{}
}

// ValidateScoping validates that the service account is properly scoped to its data product
func (v *ScopingValidator) ValidateScoping(sa ServiceAccount, saFile ServiceAccountFile) []ValidationIssue {
	var issues []ValidationIssue

	// Check if service account name includes the data product it belongs to
	// This helps ensure service accounts are scoped to their intended data product
	if saFile.DataProduct != "unknown" && saFile.DataProduct != "" {
		expectedNamePrefix := fmt.Sprintf("%s_", saFile.DataProduct)
		if !strings.HasPrefix(strings.ToLower(sa.Name), strings.ToLower(expectedNamePrefix)) {
			issues = append(issues, ValidationIssue{
				Type:       "scoping",
				Severity:   "warning",
				Message:    "Service account name should include the data product for proper scoping",
				Field:      "name",
				Value:      sa.Name,
				Suggestion: fmt.Sprintf("Consider naming pattern: %s{integration}_{environment}_appuser", saFile.DataProduct),
			})
		}
	}

	// Validate that the service account comment mentions the correct data product
	if saFile.DataProduct != "unknown" && saFile.DataProduct != "" && sa.Comment != "" {
		if !strings.Contains(strings.ToLower(sa.Comment), strings.ToLower(saFile.DataProduct)) {
			issues = append(issues, ValidationIssue{
				Type:       "scoping",
				Severity:   "warning",
				Message:    "Service account comment should reference the data product for clarity",
				Field:      "comment",
				Value:      sa.Comment,
				Suggestion: fmt.Sprintf("Include '%s' in the comment to clarify data product association", saFile.DataProduct),
			})
		}
	}

	return issues
}