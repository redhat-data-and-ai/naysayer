package masking

import (
	"fmt"
	"strings"

	"github.com/redhat-data-and-ai/naysayer/internal/gitlab"
	"github.com/redhat-data-and-ai/naysayer/internal/rules/shared"
	"gopkg.in/yaml.v3"
)

// Rule implements masking policy validation for *masking.yaml files
type Rule struct {
	client    gitlab.GitLabClient
	validator *Validator
}

// NewRule creates a new masking policy validation rule
func NewRule(client gitlab.GitLabClient) *Rule {
	return &Rule{
		client:    client,
		validator: NewValidator(),
	}
}

// Name returns the rule identifier
func (r *Rule) Name() string {
	return "masking_policy_rule"
}

// Description returns human-readable description
func (r *Rule) Description() string {
	return "Validates masking policy configurations in *masking.yaml files - auto-approves valid policies, requires manual review for invalid configurations"
}

// GetCoveredLines returns which line ranges this rule validates in a file
func (r *Rule) GetCoveredLines(filePath string, fileContent string) []shared.LineRange {
	if !r.isMaskingFile(filePath) {
		return nil
	}

	// For deleted files (empty content), still return a range so ValidateLines is called
	// This ensures deleted masking files require manual review (security-sensitive operation)
	if len(strings.TrimSpace(fileContent)) == 0 {
		return []shared.LineRange{{StartLine: 1, EndLine: 1, FilePath: filePath}}
	}

	// For masking files, we validate the entire file
	lineCount := strings.Count(fileContent, "\n") + 1
	return []shared.LineRange{
		{
			StartLine: 1,
			EndLine:   lineCount,
			FilePath:  filePath,
		},
	}
}

// ValidateLines validates masking policy configuration
func (r *Rule) ValidateLines(filePath string, fileContent string, lineRanges []shared.LineRange) (shared.DecisionType, string) {
	if !r.isMaskingFile(filePath) {
		return shared.Approve, "Not a masking policy file"
	}

	// Deleted masking policy files require manual review (security-sensitive operation)
	if len(strings.TrimSpace(fileContent)) == 0 {
		return shared.ManualReview, "Masking policy deletion requires manual review - this removes data protection"
	}

	// Parse the YAML content
	policy, err := r.parseMaskingPolicy(fileContent)
	if err != nil {
		return shared.ManualReview, fmt.Sprintf("Failed to parse masking policy YAML: %v", err)
	}

	// Skip if this is not a MaskingPolicy kind (might be a Tag)
	if !strings.EqualFold(policy.Kind, MaskingPolicyKind) {
		return shared.Approve, fmt.Sprintf("File contains '%s' kind, not MaskingPolicy", policy.Kind)
	}

	// Extract data product and environment from file path
	dataProductFromPath := r.extractDataProductFromPath(filePath)
	environment := r.extractEnvironmentFromPath(filePath)

	// Validate the masking policy
	validationResult := r.validator.Validate(policy, dataProductFromPath, environment)

	if !validationResult.IsValid {
		errorMessages := validationResult.GetErrorMessages()
		return shared.ManualReview, fmt.Sprintf("Masking policy validation failed: %s", strings.Join(errorMessages, "; "))
	}

	return shared.Approve, "Masking policy validation passed - auto-approved"
}

// isMaskingFile checks if a file is a masking policy file (excludes tag files)
func (r *Rule) isMaskingFile(path string) bool {
	if path == "" {
		return false
	}

	lowerPath := strings.ToLower(path)

	// Check for masking files (*masking.yaml or *masking.yml)
	isMasking := strings.HasSuffix(lowerPath, "masking.yaml") || strings.HasSuffix(lowerPath, "masking.yml")
	if !isMasking {
		return false
	}

	// Exclude tag files - they will have their own rule
	// Get just the filename
	parts := strings.Split(lowerPath, "/")
	filename := parts[len(parts)-1]
	return !strings.Contains(filename, "tag")
}

// parseMaskingPolicy parses YAML content into a MaskingPolicy struct
func (r *Rule) parseMaskingPolicy(content string) (*MaskingPolicy, error) {
	var policy MaskingPolicy
	err := yaml.Unmarshal([]byte(content), &policy)
	if err != nil {
		return nil, fmt.Errorf("YAML parsing error: %w", err)
	}
	return &policy, nil
}

// extractDataProductFromPath extracts the data product name from the file path
// Path format: dataproducts/<type>/<dataproduct>/<env>/<filename>
// Where type is: source, aggregate, or platform
// Example: dataproducts/source/hellosource/sandbox/pii_masking.yaml -> "hellosource"
func (r *Rule) extractDataProductFromPath(filePath string) string {
	parts := strings.Split(filePath, "/")

	// Look for "dataproducts" in the path
	for i, part := range parts {
		if strings.ToLower(part) == "dataproducts" {
			// Need at least 4 parts after "dataproducts": <type>/<dataproduct>/<env>/<file>
			if len(parts)-i-1 >= 4 {
				// Verify parts[i+1] is a known type (source, aggregate, platform)
				typeDir := strings.ToLower(parts[i+1])
				if typeDir == "source" || typeDir == "aggregate" || typeDir == "platform" {
					return strings.ToLower(parts[i+2])
				}
			}
		}
	}

	return ""
}

// extractEnvironmentFromPath extracts the environment from the file path
// Path format: dataproducts/<type>/<dataproduct>/<env>/<filename>
// Where type is: source, aggregate, or platform
// Example: dataproducts/source/hellosource/sandbox/pii_masking.yaml -> "sandbox"
func (r *Rule) extractEnvironmentFromPath(filePath string) string {
	parts := strings.Split(filePath, "/")

	// Look for "dataproducts" in the path
	for i, part := range parts {
		if strings.ToLower(part) == "dataproducts" {
			// Need at least 4 parts after "dataproducts": <type>/<dataproduct>/<env>/<file>
			if len(parts)-i-1 >= 4 {
				// Verify parts[i+1] is a known type (source, aggregate, platform)
				typeDir := strings.ToLower(parts[i+1])
				if typeDir == "source" || typeDir == "aggregate" || typeDir == "platform" {
					return strings.ToLower(parts[i+3])
				}
			}
		}
	}

	return ""
}
