package tag

import (
	"fmt"
	"strings"

	"github.com/redhat-data-and-ai/naysayer/internal/gitlab"
	"github.com/redhat-data-and-ai/naysayer/internal/rules/shared"
	"gopkg.in/yaml.v3"
)

// DefaultTargetBranch is the default branch to check for masking policy existence
const DefaultTargetBranch = "main"

// Rule implements tag validation for Tag CR files
type Rule struct {
	client    gitlab.GitLabClient
	validator *Validator
	mrCtx     *shared.MRContext
}

// NewRule creates a new tag validation rule
func NewRule(client gitlab.GitLabClient) *Rule {
	return &Rule{
		client:    client,
		validator: NewValidator(),
	}
}

// SetMRContext implements ContextAwareRule interface
func (r *Rule) SetMRContext(mrCtx *shared.MRContext) {
	r.mrCtx = mrCtx
}

// Name returns the rule identifier
func (r *Rule) Name() string {
	return "tag_rule"
}

// Description returns human-readable description
func (r *Rule) Description() string {
	return "Validates tag configurations in Tag CR files - auto-approves valid tags, requires manual review for invalid configurations"
}

// GetCoveredLines returns which line ranges this rule validates in a file
func (r *Rule) GetCoveredLines(filePath string, fileContent string) []shared.LineRange {
	if !r.isTagFile(filePath, fileContent) {
		return nil
	}

	// For deleted files (empty content), still return a range so ValidateLines is called
	if len(strings.TrimSpace(fileContent)) == 0 {
		return []shared.LineRange{{StartLine: 1, EndLine: 1, FilePath: filePath}}
	}

	// For tag files, we validate the entire file
	lineCount := strings.Count(fileContent, "\n") + 1
	return []shared.LineRange{
		{
			StartLine: 1,
			EndLine:   lineCount,
			FilePath:  filePath,
		},
	}
}

// ValidateLines validates tag configuration
func (r *Rule) ValidateLines(filePath string, fileContent string, lineRanges []shared.LineRange) (shared.DecisionType, string) {
	if !r.isTagFile(filePath, fileContent) {
		return shared.Approve, "Not a tag file"
	}

	// Deleted tag files require manual review (security-sensitive operation)
	if len(strings.TrimSpace(fileContent)) == 0 {
		return shared.ManualReview, "Tag deletion requires manual review - this removes tag configuration"
	}

	// Parse the YAML content
	tag, err := r.parseTag(fileContent)
	if err != nil || tag == nil {
		return shared.ManualReview, fmt.Sprintf("Failed to parse tag YAML: %v", err)
	}

	// Skip if this is not a Tag kind (might be a MaskingPolicy)
	if !strings.EqualFold(tag.Kind, TagKind) {
		return shared.Approve, fmt.Sprintf("File contains '%s' kind, not Tag", tag.Kind)
	}

	// Extract data product, type, and environment from file path
	dpType, dataProductFromPath, environment := r.extractPathInfo(filePath)

	// Validate the tag using the validator
	validationResult := r.validator.Validate(tag, dataProductFromPath)

	if !validationResult.IsValid {
		errorMessages := validationResult.GetErrorMessages()
		return shared.ManualReview, fmt.Sprintf("Tag validation failed: %s", strings.Join(errorMessages, "; "))
	}

	// Check if all masking policies exist in the repository
	if r.client != nil && r.mrCtx != nil {
		var missingPolicies []string
		for _, policy := range tag.MaskingPolicies {
			exists, reason := r.checkMaskingPolicyExists(policy.Name, dpType, tag.DataProduct, environment)
			if !exists {
				missingPolicies = append(missingPolicies, reason)
			}
		}
		if len(missingPolicies) > 0 {
			return shared.ManualReview, fmt.Sprintf("Missing masking policies: %s", strings.Join(missingPolicies, "; "))
		}
	}

	return shared.Approve, "Tag validation passed - auto-approved"
}

// isTagFile checks if a file is a tag file
func (r *Rule) isTagFile(filePath string, fileContent string) bool {
	if filePath == "" {
		return false
	}

	lowerPath := strings.ToLower(filePath)

	// Must be a YAML file
	if !strings.HasSuffix(lowerPath, ".yaml") && !strings.HasSuffix(lowerPath, ".yml") {
		return false
	}

	// Must be in dataproducts directory
	if !strings.Contains(lowerPath, "dataproducts/") {
		return false
	}

	// Check content for kind: Tag
	if fileContent != "" {
		var parsed struct {
			Kind string `yaml:"kind"`
		}
		if err := yaml.Unmarshal([]byte(fileContent), &parsed); err == nil {
			return strings.EqualFold(parsed.Kind, TagKind)
		}
	}

	// Fall back to filename heuristic if content is empty or unparseable
	parts := strings.Split(lowerPath, "/")
	filename := parts[len(parts)-1]
	return strings.Contains(filename, "tag")
}

// parseTag parses YAML content into a Tag struct
func (r *Rule) parseTag(content string) (*Tag, error) {
	var tag Tag
	err := yaml.Unmarshal([]byte(content), &tag)
	if err != nil {
		return nil, fmt.Errorf("YAML parsing error: %w", err)
	}
	return &tag, nil
}

// extractPathInfo extracts data product type, name, and environment from the file path.
// Path format: dataproducts/<type>/<dataproduct>/<env>/<filename>
// Where type is: source, aggregate, or platform
// Example: dataproducts/source/analytics/sandbox/tag_pii.yaml -> "source", "analytics", "sandbox"
func (r *Rule) extractPathInfo(filePath string) (dpType, dataProduct, environment string) {
	parts := strings.Split(filePath, "/")

	// Look for "dataproducts" in the path
	for i, part := range parts {
		if strings.ToLower(part) == "dataproducts" {
			// Need at least 4 parts after "dataproducts": <type>/<dataproduct>/<env>/<file>
			if len(parts)-i-1 >= 4 {
				// Verify parts[i+1] is a known type (source, aggregate, platform)
				typeDir := strings.ToLower(parts[i+1])
				if typeDir == "source" || typeDir == "aggregate" || typeDir == "platform" {
					return typeDir, strings.ToLower(parts[i+2]), strings.ToLower(parts[i+3])
				}
			}
		}
	}

	return "", "", ""
}

// checkMaskingPolicyExists checks if a masking policy exists in the repository
func (r *Rule) checkMaskingPolicyExists(policyName, dpType, dataProduct, environment string) (bool, string) {
	// Step 1: Check if policy is being added in the same MR
	if r.policyExistsInMRChanges(policyName, dataProduct) {
		return true, ""
	}

	// Step 2: Check target branch
	if r.client == nil || r.mrCtx == nil {
		return true, "" // Skip check if no client/context
	}

	// Get target branch from MR info
	targetBranch := DefaultTargetBranch
	if r.mrCtx.MRInfo != nil && r.mrCtx.MRInfo.TargetBranch != "" {
		targetBranch = r.mrCtx.MRInfo.TargetBranch
	}

	// If dpType is empty, try all types
	typesToCheck := []string{dpType}
	if dpType == "" {
		typesToCheck = []string{"source", "aggregate", "platform"}
	}

	for _, checkType := range typesToCheck {
		dirPath := fmt.Sprintf("dataproducts/%s/%s/%s", checkType, dataProduct, environment)

		// List files in the directory
		files, err := r.client.ListDirectoryFiles(r.mrCtx.ProjectID, dirPath, targetBranch)
		if err != nil {
			continue
		}

		// Check each masking file for the policy
		for _, file := range files {
			if file.Type != "blob" {
				continue
			}
			if !strings.Contains(strings.ToLower(file.Name), "masking") {
				continue
			}
			if !strings.HasSuffix(strings.ToLower(file.Name), ".yaml") &&
				!strings.HasSuffix(strings.ToLower(file.Name), ".yml") {
				continue
			}

			// Fetch and parse the file
			content, err := r.client.FetchFileContent(r.mrCtx.ProjectID, file.Path, targetBranch)
			if err != nil {
				continue
			}

			// Check if this file contains the policy
			if r.fileContainsPolicy(content.Content, policyName) {
				return true, ""
			}
		}
	}

	return false, fmt.Sprintf("Masking policy '%s' not found in repository", policyName)
}

// policyExistsInMRChanges checks if a masking policy is being added in the same MR
func (r *Rule) policyExistsInMRChanges(policyName, dataProduct string) bool {
	if r.mrCtx == nil {
		return false
	}

	for _, change := range r.mrCtx.Changes {
		if change.DeletedFile {
			continue
		}
		// Check if file is a masking file in this data product
		lowerPath := strings.ToLower(change.NewPath)
		if !strings.Contains(lowerPath, "masking") {
			continue
		}
		if !strings.Contains(lowerPath, dataProduct) {
			continue
		}
		// Check diff for policy name and kind
		if strings.Contains(change.Diff, "kind: MaskingPolicy") &&
			strings.Contains(change.Diff, "name: "+policyName) {
			return true
		}
	}
	return false
}

// fileContainsPolicy checks if a file content contains a specific masking policy name
func (r *Rule) fileContainsPolicy(content, policyName string) bool {
	var parsed struct {
		Kind string `yaml:"kind"`
		Name string `yaml:"name"`
	}
	if err := yaml.Unmarshal([]byte(content), &parsed); err != nil {
		return false
	}
	return parsed.Kind == "MaskingPolicy" && parsed.Name == policyName
}

// CheckDuplicateTagNames checks for duplicate tag names in the MR changes
// Returns a list of duplicate tag names found
func (r *Rule) CheckDuplicateTagNames() []string {
	if r.mrCtx == nil {
		return nil
	}

	tagNames := make(map[string]int)
	var duplicates []string

	for _, change := range r.mrCtx.Changes {
		if change.DeletedFile {
			continue
		}
		// Check if file looks like a tag file
		lowerPath := strings.ToLower(change.NewPath)
		if !strings.Contains(lowerPath, "dataproducts/") {
			continue
		}
		if !strings.HasSuffix(lowerPath, ".yaml") && !strings.HasSuffix(lowerPath, ".yml") {
			continue
		}

		// Try to extract tag name from diff
		if strings.Contains(change.Diff, "kind: Tag") {
			// Extract name from diff
			name := r.extractTagNameFromDiff(change.Diff)
			if name != "" {
				tagNames[name]++
			}
		}
	}

	// Find duplicates
	for name, count := range tagNames {
		if count > 1 {
			duplicates = append(duplicates, name)
		}
	}

	return duplicates
}

// extractTagNameFromDiff extracts the tag name from a diff
func (r *Rule) extractTagNameFromDiff(diff string) string {
	lines := strings.Split(diff, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "+name:") || strings.HasPrefix(line, "name:") {
			parts := strings.SplitN(line, ":", 2)
			if len(parts) == 2 {
				name := strings.TrimSpace(parts[1])
				name = strings.TrimPrefix(name, "+")
				return strings.TrimSpace(name)
			}
		}
	}
	return ""
}
