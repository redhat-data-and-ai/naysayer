package masking

import (
	"fmt"
	"strings"

	"github.com/redhat-data-and-ai/naysayer/internal/gitlab"
	"github.com/redhat-data-and-ai/naysayer/internal/rules/shared"
	"gopkg.in/yaml.v3"
)

// DefaultTargetBranch is the default branch to check for consumer existence
const DefaultTargetBranch = "main"

// Rule implements masking policy validation for *masking.yaml files
type Rule struct {
	client    gitlab.GitLabClient
	validator *Validator
	mrCtx     *shared.MRContext // Store MR context for consumer existence checks
}

// NewRule creates a new masking policy validation rule
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
	if err != nil || policy == nil {
		return shared.ManualReview, fmt.Sprintf("Failed to parse masking policy YAML: %v", err)
	}

	// Skip if this is not a MaskingPolicy kind (might be a Tag)
	if !strings.EqualFold(policy.Kind, MaskingPolicyKind) {
		return shared.Approve, fmt.Sprintf("File contains '%s' kind, not MaskingPolicy", policy.Kind)
	}

	// Extract data product and environment from file path
	dataProductFromPath, environment := r.extractPathInfo(filePath)

	// Validate the masking policy
	validationResult := r.validator.Validate(policy, dataProductFromPath, environment)

	if !validationResult.IsValid {
		errorMessages := validationResult.GetErrorMessages()
		return shared.ManualReview, fmt.Sprintf("Masking policy validation failed: %s", strings.Join(errorMessages, "; "))
	}

	// Check if all consumers exist in the repository
	if r.client != nil && r.mrCtx != nil {
		var missingConsumers []string
		for _, c := range policy.Cases {
			for _, consumer := range c.Consumers {
				exists, reason := r.checkConsumerExists(consumer, environment)
				if !exists {
					missingConsumers = append(missingConsumers, reason)
				}
			}
		}
		if len(missingConsumers) > 0 {
			return shared.ManualReview, fmt.Sprintf("Missing consumers: %s", strings.Join(missingConsumers, "; "))
		}
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

// extractPathInfo extracts data product and environment from the file path.
// Path format: dataproducts/<type>/<dataproduct>/<env>/<filename>
// Where type is: source, aggregate, or platform
// Example: dataproducts/source/hellosource/sandbox/pii_masking.yaml -> "hellosource", "sandbox"
func (r *Rule) extractPathInfo(filePath string) (dataProduct, environment string) {
	parts := strings.Split(filePath, "/")

	// Look for "dataproducts" in the path
	for i, part := range parts {
		if strings.ToLower(part) == "dataproducts" {
			// Need at least 4 parts after "dataproducts": <type>/<dataproduct>/<env>/<file>
			if len(parts)-i-1 >= 4 {
				// Verify parts[i+1] is a known type (source, aggregate, platform)
				typeDir := strings.ToLower(parts[i+1])
				if typeDir == "source" || typeDir == "aggregate" || typeDir == "platform" {
					return strings.ToLower(parts[i+2]), strings.ToLower(parts[i+3])
				}
			}
		}
	}

	return "", ""
}

// checkConsumerExists checks if a consumer (group or service account) exists in the repository
func (r *Rule) checkConsumerExists(consumer Consumer, environment string) (bool, string) {
	kind := strings.ToLower(consumer.Kind)
	name := strings.ToLower(consumer.Name)

	switch kind {
	case ConsumerKindGroup:
		return r.checkGroupExists(name)
	case ConsumerKindServiceAccount:
		return r.checkServiceAccountExists(name, environment)
	default:
		// Unknown kind - let naming validation handle it
		return true, ""
	}
}

// checkGroupExists checks if a consumer group file exists in the repository
// Group naming patterns:
//   - Pattern 1: dataverse-(source|aggregate|platform)-<dataproduct>
//     Example: dataverse-source-analytics, dataverse-aggregate-marketing
//   - Pattern 2: dataverse-consumer-<dataproduct>-<suffix>
//     Example: dataverse-consumer-analytics-marts, dataverse-consumer-sales-reports
//
// In both patterns, dataproduct is at position [2] when split by "-"
// File location: dataproducts/<type>/<dataproduct>/groups/<group_name>.yaml
func (r *Rule) checkGroupExists(groupName string) (bool, string) {
	// Extract dataproduct from group name
	dataProduct := r.extractDataProductFromGroupName(groupName)
	if dataProduct == "" {
		return false, fmt.Sprintf("Consumer group '%s' has invalid naming - cannot extract data product", groupName)
	}

	// Check all possible type directories (source, aggregate, platform)
	// because we don't know which type the referenced dataproduct is
	types := []string{"source", "aggregate", "platform"}

	for _, dpType := range types {
		filePath := fmt.Sprintf("dataproducts/%s/%s/groups/%s.yaml", dpType, dataProduct, groupName)
		if r.fileExistsInRepo(filePath) {
			return true, ""
		}
	}

	return false, fmt.Sprintf("Consumer group '%s' not found in repository - expected at dataproducts/<type>/%s/groups/%s.yaml", groupName, dataProduct, groupName)
}

// checkServiceAccountExists checks if a service account file exists in the repository
// File location: serviceaccounts/<env>/<sa_name>.yaml
func (r *Rule) checkServiceAccountExists(saName, environment string) (bool, string) {
	if environment == "" {
		return false, fmt.Sprintf("Cannot verify service account '%s' - environment not detected from file path", saName)
	}

	filePath := fmt.Sprintf("serviceaccounts/%s/%s.yaml", environment, saName)
	if r.fileExistsInRepo(filePath) {
		return true, ""
	}

	return false, fmt.Sprintf("Service account '%s' not found in repository - expected at serviceaccounts/%s/%s.yaml", saName, environment, saName)
}

// extractDataProductFromGroupName extracts the data product name from a consumer group name
// Pattern 1: dataverse-<source|aggregate|platform>-<dataproduct>
// Pattern 2: dataverse-consumer-<dataproduct>-<martname>
// In both patterns, dataproduct is at position [2] (3rd part)
func (r *Rule) extractDataProductFromGroupName(groupName string) string {
	parts := strings.Split(groupName, "-")
	if len(parts) >= 3 {
		return parts[2] // Always the 3rd part (index 2)
	}
	return ""
}

// fileExistsInRepo checks if a file exists in the repository using GitLab API
func (r *Rule) fileExistsInRepo(filePath string) bool {
	if r.client == nil || r.mrCtx == nil {
		return true // If no client/context, skip existence check (validation only)
	}

	// Get target branch from MR info
	targetBranch := DefaultTargetBranch
	if r.mrCtx.MRInfo != nil && r.mrCtx.MRInfo.TargetBranch != "" {
		targetBranch = r.mrCtx.MRInfo.TargetBranch
	}

	// Try to fetch the file from the repository
	_, err := r.client.FetchFileContent(r.mrCtx.ProjectID, filePath, targetBranch)
	if err != nil {
		// Check if file exists in the current MR changes (being added in same MR)
		for _, change := range r.mrCtx.Changes {
			if strings.EqualFold(change.NewPath, filePath) && !change.DeletedFile {
				return true // File is being added in this MR
			}
		}
		return false
	}

	return true
}
