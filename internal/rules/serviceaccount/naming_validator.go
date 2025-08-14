package serviceaccount

import (
	"fmt"
	"regexp"
	"strings"
)

// NamingValidator handles naming convention validation
type NamingValidator struct {
	serviceAccountNameRegex *regexp.Regexp
}

// NewNamingValidator creates a new naming validator
func NewNamingValidator() *NamingValidator {
	// Expected pattern based on existing files: {dataproduct}_{integration}_{environment}_{type}
	// e.g., "marketo_astro_preprod_appuser", "dataverse_operator_dev_appuser"
	nameRegex := regexp.MustCompile(`^[a-z][a-z0-9_]*[a-z0-9]$`)

	return &NamingValidator{
		serviceAccountNameRegex: nameRegex,
	}
}

// ValidateNaming validates service account naming conventions
func (v *NamingValidator) ValidateNaming(sa ServiceAccount, saFile ServiceAccountFile) []ValidationIssue {
	var issues []ValidationIssue

	// Basic name format validation
	if !v.serviceAccountNameRegex.MatchString(sa.Name) {
		issues = append(issues, ValidationIssue{
			Type:       "naming",
			Severity:   "error",
			Message:    "Service account name must follow naming convention: lowercase, alphanumeric with underscores",
			Field:      "name",
			Value:      sa.Name,
			Suggestion: "Use format: {dataproduct}_{integration}_{environment}_{type} (e.g., marketo_astro_preprod_appuser)",
		})
	}

	// Check if name includes data product
	if saFile.DataProduct != "unknown" && saFile.DataProduct != "" {
		if !strings.Contains(strings.ToLower(sa.Name), strings.ToLower(saFile.DataProduct)) {
			issues = append(issues, ValidationIssue{
				Type:       "naming",
				Severity:   "warning",
				Message:    "Service account name should include the data product name for clarity",
				Field:      "name",
				Value:      sa.Name,
				Suggestion: fmt.Sprintf("Consider including '%s' in the service account name", saFile.DataProduct),
			})
		}
	}

	// Check if name includes environment
	if saFile.Environment != "unknown" && saFile.Environment != "" {
		if !strings.Contains(strings.ToLower(sa.Name), strings.ToLower(saFile.Environment)) {
			issues = append(issues, ValidationIssue{
				Type:       "naming",
				Severity:   "warning",
				Message:    "Service account name should include the environment for clarity",
				Field:      "name",
				Value:      sa.Name,
				Suggestion: fmt.Sprintf("Consider including '%s' in the service account name", saFile.Environment),
			})
		}
	}

	// Check if name includes integration type
	if saFile.Integration != "unknown" && saFile.Integration != "" {
		if !strings.Contains(strings.ToLower(sa.Name), strings.ToLower(saFile.Integration)) {
			issues = append(issues, ValidationIssue{
				Type:       "naming",
				Severity:   "info",
				Message:    "Service account name should include the integration type for clarity",
				Field:      "name",
				Value:      sa.Name,
				Suggestion: fmt.Sprintf("Consider including '%s' in the service account name", saFile.Integration),
			})
		}
	}

	// Validate filename matches name
	expectedFilename := fmt.Sprintf("%s.yaml", sa.Name)
	if !strings.HasSuffix(saFile.Path, expectedFilename) {
		issues = append(issues, ValidationIssue{
			Type:       "naming",
			Severity:   "error",
			Message:    "Service account filename should match the name field",
			Field:      "name",
			Value:      sa.Name,
			Suggestion: fmt.Sprintf("Rename file to %s or update name field to match filename", expectedFilename),
		})
	}

	return issues
}