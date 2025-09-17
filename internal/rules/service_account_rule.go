package rules

import (
	"path/filepath"
	"strings"

	"github.com/redhat-data-and-ai/naysayer/internal/gitlab"
	"github.com/redhat-data-and-ai/naysayer/internal/logging"
	"github.com/redhat-data-and-ai/naysayer/internal/rules/common"
	"github.com/redhat-data-and-ai/naysayer/internal/rules/shared"
	"gopkg.in/yaml.v3"
)

// ServiceAccountRule validates service account files based on configurable patterns and rules.
// This rule supports various service account types and can be extended with additional validation logic.
type ServiceAccountRule struct {
	*common.BaseRule
	client *gitlab.Client
}

// NewServiceAccountRule creates a new service account rule
func NewServiceAccountRule(client *gitlab.Client) *ServiceAccountRule {
	return &ServiceAccountRule{
		BaseRule: common.NewBaseRule(
			"service_account_rule",
			"Auto-approves Astro service account files (**_astro_<env>_appuser.yaml/yml) when the 'name' field matches the filename. Other service account files require manual review.",
		),
		client: client,
	}
}

// GetCoveredLines returns which line ranges this rule validates in a file
func (r *ServiceAccountRule) GetCoveredLines(filePath string, fileContent string) []shared.LineRange {
	if !r.isServiceAccountFile(filePath) {
		return []shared.LineRange{}
	}

	// Don't cover empty or whitespace-only files
	if strings.TrimSpace(fileContent) == "" {
		return []shared.LineRange{}
	}

	// This rule validates the entire file structure
	return r.GetFullFileCoverage(filePath, fileContent)
}

// ValidateLines validates the specified line ranges using file-level logic
func (r *ServiceAccountRule) ValidateLines(filePath string, fileContent string, lineRanges []shared.LineRange) (shared.DecisionType, string) {
	if !r.isServiceAccountFile(filePath) {
		return shared.ManualReview, "Not a service account file"
	}

	// Only auto-approve Astro service accounts, all others require manual review
	saType := r.getServiceAccountType(filePath)

	if saType == "astro" {
		return r.validateAstroServiceAccount(filePath, fileContent)
	}

	// All non-Astro service accounts require manual review
	logging.Info("Non-Astro service account file %s requires manual review", filePath)
	return shared.ManualReview, "Only Astro service account files (*_astro_*.yaml/yml) are auto-approved - other service account files require manual review"
}

// isServiceAccountFile checks if a file is a service account file
func (r *ServiceAccountRule) isServiceAccountFile(filePath string) bool {
	if filePath == "" {
		return false
	}

	lowerPath := strings.ToLower(filePath)
	filename := filepath.Base(lowerPath)

	// Check for various service account patterns
	return (strings.Contains(filename, "_astro_") ||
		strings.Contains(filename, "serviceaccount") ||
		strings.Contains(filename, "service-account") ||
		strings.Contains(lowerPath, "serviceaccounts/")) &&
		(strings.HasSuffix(filename, ".yaml") || strings.HasSuffix(filename, ".yml"))
}

// getServiceAccountType determines the type of service account based on file path patterns
func (r *ServiceAccountRule) getServiceAccountType(filePath string) string {
	if filePath == "" {
		return "unknown"
	}

	lowerPath := strings.ToLower(filePath)
	filename := filepath.Base(lowerPath)

	// Check for Astro service account pattern: **_astro_<env>_appuser.yaml/yml
	if r.isAstroServiceAccountPattern(filename) {
		return "astro"
	} else if strings.Contains(filename, "_appuser") {
		return "appuser"
	} else if strings.Contains(lowerPath, "serviceaccounts/") {
		return "generic"
	}

	return "generic"
}

// isAstroServiceAccountPattern checks if filename matches the Astro service account pattern
// Pattern: **_astro_<env>_appuser.yaml/yml
func (r *ServiceAccountRule) isAstroServiceAccountPattern(filename string) bool {
	lowerFilename := strings.ToLower(filename)

	// Must contain _astro_ and end with _appuser.yaml or _appuser.yml
	return strings.Contains(lowerFilename, "_astro_") &&
		(strings.HasSuffix(lowerFilename, "_appuser.yaml") || strings.HasSuffix(lowerFilename, "_appuser.yml"))
}

// validateAstroServiceAccount validates Astro-specific service account files
func (r *ServiceAccountRule) validateAstroServiceAccount(filePath string, fileContent string) (shared.DecisionType, string) {
	// Parse YAML content to extract the 'name' field
	var yamlData map[string]interface{}
	if err := yaml.Unmarshal([]byte(fileContent), &yamlData); err != nil {
		logging.Warn("Failed to parse YAML content for %s: %v", filePath, err)
		return shared.ManualReview, "Failed to parse YAML content"
	}

	// Extract the name field
	nameField, exists := yamlData["name"]
	if !exists {
		return shared.ManualReview, "YAML file does not contain a 'name' field"
	}

	nameValue, ok := nameField.(string)
	if !ok {
		return shared.ManualReview, "'name' field is not a string"
	}

	// Get expected name from filename (without extension)
	expectedName := r.getExpectedNameFromFilename(filePath)
	if expectedName == "" {
		return shared.ManualReview, "Could not extract expected name from filename"
	}

	// Check if the name matches
	if nameValue != expectedName {
		return shared.ManualReview,
			"Name field value '" + nameValue + "' does not match expected filename-based name '" + expectedName + "'"
	}

	logging.Info("Astro service account file %s validated successfully: name field '%s' matches filename", filePath, nameValue)
	return shared.Approve, "Astro service account file follows naming convention and name field matches filename"
}

// getExpectedNameFromFilename extracts the expected name from the filename by removing the extension
func (r *ServiceAccountRule) getExpectedNameFromFilename(filePath string) string {
	filename := filepath.Base(filePath)
	lowerFilename := strings.ToLower(filename)

	// Remove .yaml or .yml extension (case insensitive)
	if strings.HasSuffix(lowerFilename, ".yaml") {
		return filename[:len(filename)-5] // Remove ".yaml"
	} else if strings.HasSuffix(lowerFilename, ".yml") {
		return filename[:len(filename)-4] // Remove ".yml"
	}

	return ""
}
