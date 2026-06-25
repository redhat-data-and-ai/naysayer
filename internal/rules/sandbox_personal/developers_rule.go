package sandbox_personal

import (
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/redhat-data-and-ai/naysayer/internal/config"
	"github.com/redhat-data-and-ai/naysayer/internal/gitlab"
	"github.com/redhat-data-and-ai/naysayer/internal/logging"
	"github.com/redhat-data-and-ai/naysayer/internal/rules/common"
	"github.com/redhat-data-and-ai/naysayer/internal/rules/shared"
	"gopkg.in/yaml.v3"
)

// DevelopersRule validates that product-level developers.yaml files
// for UnstructuredDataProducts have exactly 2 owners (1 human + 1 service account) and match CODEOWNERS
type DevelopersRule struct {
	*common.BaseRule
	*common.ValidationHelper
	client             gitlab.GitLabClient
	mrContext          *shared.MRContext
	serviceAccountName string // Configurable service account name from config
}

// DevelopersYAML represents the structure of developers.yaml
type DevelopersYAML struct {
	Group DevelopersGroup `yaml:"group"`
}

// DevelopersGroup represents the group section
type DevelopersGroup struct {
	Owners []string `yaml:"owners"`
}

// NewDevelopersRule creates a new sandbox developers rule
func NewDevelopersRule(client gitlab.GitLabClient, cfg *config.Config) *DevelopersRule {
	serviceAccountName := cfg.Rules.SandboxPersonalRule.ServiceAccountName
	if serviceAccountName == "" {
		logging.Warn("[sandbox_developers_rule] SANDBOX_SERVICE_ACCOUNT_NAME not set - service account validation will be disabled")
	}

	return &DevelopersRule{
		BaseRule:           common.NewBaseRule("sandbox_developers_rule", "Validates exactly 2 developers (1 human + 1 service account) for sandbox UnstructuredDataProduct and matches CODEOWNERS"),
		ValidationHelper:   common.NewValidationHelper(),
		client:             client,
		serviceAccountName: serviceAccountName,
	}
}

// SetMRContext implements the ContextAwareRule interface
func (r *DevelopersRule) SetMRContext(mrCtx *shared.MRContext) {
	r.mrContext = mrCtx
}

// ValidateLines validates the developers.yaml file
func (r *DevelopersRule) ValidateLines(filePath string, fileContent string, lineRanges []shared.LineRange) (shared.DecisionType, string) {
	if r.mrContext == nil {
		logging.Warn("[%s] No MR context available for validation", r.Name())
		return r.CreateApprovalResult("Auto-approved: No MR context")
	}

	// Only apply when the associated product is a sandbox UnstructuredDataProduct with aif-* name
	isAIFProduct, err := IsSandboxPersonalProductForFile(r.mrContext, r.client, filePath)
	if err != nil {
		// Fail-closed: if we can't verify the product type, require manual review
		logging.Error("[%s] Failed to check product type: %v", r.Name(), err)
		return r.CreateManualReviewResult(fmt.Sprintf("Manual review required: Failed to verify product type: %v", err))
	}
	if !isAIFProduct {
		return r.CreateApprovalResult("Auto-approved: Not a sandbox UnstructuredDataProduct with aif-* name")
	}

	// This MR IS a sandbox Personal context - now check if THIS file is developers.yaml at product root
	if !strings.HasSuffix(filePath, "developers.yaml") && !strings.HasSuffix(filePath, "developers.yml") {
		return r.CreateApprovalResult("Auto-approved: Not a developers.yaml file")
	}

	// Validate file is at product root (not inside sandbox/dev/preprod/prod)
	// Expected: dataproducts/unstructured/{product}/developers.yaml (exactly 4 parts)
	if !r.isAtProductRoot(filePath) {
		return r.CreateApprovalResult("Auto-approved: developers.yaml not at product root")
	}

	// Parse current content
	var currentDevelopers DevelopersYAML
	err = yaml.Unmarshal([]byte(fileContent), &currentDevelopers)
	if err != nil {
		logging.Error("[%s] Failed to parse developers.yaml: %v", r.Name(), err)
		return r.CreateManualReviewResult(fmt.Sprintf("Manual review required: Failed to parse developers.yaml: %v", err))
	}

	currentCount := len(currentDevelopers.Group.Owners)

	// Extract product name from file path
	productName, err := r.extractProductName(filePath)
	if err != nil {
		logging.Error("[%s] Failed to extract product name: %v", r.Name(), err)
		return r.CreateManualReviewResult(fmt.Sprintf("Manual review required: Failed to extract product name: %v", err))
	}

	// Check if this is a NEW file
	isNewFile, err := isNewFileInMR(r.mrContext, r.client, filePath)
	if err != nil {
		// Network error, API error, or rate limit - fail-closed
		logging.Error("[%s] Failed to check if file is new: %v", r.Name(), err)
		return r.CreateManualReviewResult(fmt.Sprintf("Manual review required: Failed to verify if file is new: %v", err))
	}

	if isNewFile {
		logging.Info("[%s] NEW developers.yaml detected, validating count=2 and CODEOWNERS match", r.Name())

		// NEW file must have exactly 2 owners
		if currentCount != 2 {
			reason := fmt.Sprintf("Manual review required: New developers.yaml must have exactly 2 developers (1 human + 1 service account), found: %d", currentCount)
			logging.Warn("[%s] %s", r.Name(), reason)
			return r.CreateManualReviewResult(reason)
		}

		// Validate one is service account
		if !r.containsServiceAccount(currentDevelopers.Group.Owners) {
			reason := fmt.Sprintf("Manual review required: developers.yaml must include service account '%s'", r.serviceAccountName)
			logging.Warn("[%s] %s", r.Name(), reason)
			return r.CreateManualReviewResult(reason)
		}

		// Validate CODEOWNERS match
		if decision, reason := r.validateCodeownersMatch(productName, currentDevelopers.Group.Owners); decision != shared.Approve {
			return decision, reason
		}

		return r.CreateApprovalResult("Auto-approved: New developers.yaml with 2 developers matching CODEOWNERS")
	}

	// EXISTING file - validate count=2 and no changes
	logging.Info("[%s] EXISTING developers.yaml detected, validating count=2 and unchanged", r.Name())

	// Get previous content
	previousContent, err := r.getPreviousContent(filePath)
	if err != nil {
		logging.Error("[%s] Failed to fetch previous developers.yaml: %v", r.Name(), err)
		return r.CreateManualReviewResult(fmt.Sprintf("Manual review required: Failed to fetch previous content: %v", err))
	}

	var previousDevelopers DevelopersYAML
	err = yaml.Unmarshal([]byte(previousContent), &previousDevelopers)
	if err != nil {
		logging.Error("[%s] Failed to parse previous developers.yaml: %v", r.Name(), err)
		return r.CreateManualReviewResult(fmt.Sprintf("Manual review required: Failed to parse previous developers.yaml: %v", err))
	}

	previousCount := len(previousDevelopers.Group.Owners)

	// Validate both current and previous have exactly 2 owners
	if currentCount != 2 || previousCount != 2 {
		reason := fmt.Sprintf("Manual review required: Must maintain exactly 2 developers (current: %d, previous: %d)", currentCount, previousCount)
		logging.Warn("[%s] %s", r.Name(), reason)
		return r.CreateManualReviewResult(reason)
	}

	// Validate current has service account
	if !r.containsServiceAccount(currentDevelopers.Group.Owners) {
		reason := fmt.Sprintf("Manual review required: developers.yaml must include service account '%s'", r.serviceAccountName)
		logging.Warn("[%s] %s", r.Name(), reason)
		return r.CreateManualReviewResult(reason)
	}

	// Check if any owner has changed
	currentOwners := make([]string, len(currentDevelopers.Group.Owners))
	copy(currentOwners, currentDevelopers.Group.Owners)
	sort.Strings(currentOwners)

	previousOwners := make([]string, len(previousDevelopers.Group.Owners))
	copy(previousOwners, previousDevelopers.Group.Owners)
	sort.Strings(previousOwners)

	hasChanged := false
	if len(currentOwners) != len(previousOwners) {
		hasChanged = true
	} else {
		for i := range currentOwners {
			if currentOwners[i] != previousOwners[i] {
				hasChanged = true
				break
			}
		}
	}

	if hasChanged {
		reason := fmt.Sprintf("Manual review required: Developers cannot be changed (from: %v, to: %v)",
			previousOwners, currentOwners)
		logging.Warn("[%s] %s", r.Name(), reason)
		return r.CreateManualReviewResult(reason)
	}

	// Validate CODEOWNERS match for existing file too
	if decision, reason := r.validateCodeownersMatch(productName, currentDevelopers.Group.Owners); decision != shared.Approve {
		return decision, reason
	}

	// No changes detected
	return r.CreateApprovalResult("Auto-approved: developers.yaml unchanged with 2 developers matching CODEOWNERS")
}

// GetCoveredLines returns the full file coverage
func (r *DevelopersRule) GetCoveredLines(filePath string, fileContent string) []shared.LineRange {
	return r.GetFullFileCoverage(filePath, fileContent)
}

// getPreviousContent fetches the file content from the target branch (before the MR changes)
func (r *DevelopersRule) getPreviousContent(filePath string) (string, error) {
	if r.client == nil {
		return "", fmt.Errorf("GitLab client not available")
	}

	if r.mrContext == nil {
		return "", fmt.Errorf("MR context not available")
	}

	targetBranch := r.mrContext.MRInfo.TargetBranch
	if targetBranch == "" {
		return "", fmt.Errorf("target branch not available")
	}

	// Fetch from target branch (the "before" state)
	beforeContent, err := r.client.FetchFileContent(r.mrContext.ProjectID, filePath, targetBranch)
	if err != nil {
		return "", fmt.Errorf("failed to fetch file from %s: %w", targetBranch, err)
	}

	if beforeContent == nil {
		return "", fmt.Errorf("file content is nil")
	}

	content := strings.TrimSpace(beforeContent.Content)
	if content == "" {
		return "", fmt.Errorf("file content is empty")
	}

	return content, nil
}

// extractProductName extracts the product name from file path
// Example: dataproducts/unstructured/aif-test/developers.yaml -> aif-test
func (r *DevelopersRule) extractProductName(filePath string) (string, error) {
	parts := strings.Split(filePath, "/")
	for i, part := range parts {
		if part == "dataproducts" && i+2 < len(parts) && parts[i+1] == "unstructured" {
			return parts[i+2], nil
		}
	}
	return "", fmt.Errorf("could not extract product name from path: %s", filePath)
}

// containsServiceAccount checks if the owners list contains the service account
func (r *DevelopersRule) containsServiceAccount(owners []string) bool {
	for _, owner := range owners {
		if owner == r.serviceAccountName {
			return true
		}
	}
	return false
}

// validateCodeownersMatch validates that developers.yaml members exactly match CODEOWNERS
func (r *DevelopersRule) validateCodeownersMatch(productName string, developersOwners []string) (shared.DecisionType, string) {
	// Fetch CODEOWNERS file
	codeownersContent, err := r.fetchCodeowners()
	if err != nil {
		logging.Error("[%s] Failed to fetch CODEOWNERS: %v", r.Name(), err)
		return r.CreateManualReviewResult(fmt.Sprintf("Manual review required: Failed to fetch CODEOWNERS: %v", err))
	}

	// Find the CODEOWNERS entry for this product
	codeownersPattern := fmt.Sprintf("/dataproducts/unstructured/%s/", productName)
	codeownersMembers, err := r.extractCodeownersMembers(codeownersContent, codeownersPattern)
	if err != nil {
		logging.Error("[%s] Failed to find CODEOWNERS entry for %s: %v", r.Name(), codeownersPattern, err)
		return r.CreateManualReviewResult(fmt.Sprintf("Manual review required: No CODEOWNERS entry found for %s", codeownersPattern))
	}

	// Normalize and sort both lists for comparison
	devOwners := make([]string, len(developersOwners))
	for i, owner := range developersOwners {
		// Add @ prefix if not present for comparison
		if !strings.HasPrefix(owner, "@") {
			devOwners[i] = "@" + owner
		} else {
			devOwners[i] = owner
		}
	}
	sort.Strings(devOwners)
	sort.Strings(codeownersMembers)

	// Exact match validation
	if len(devOwners) != len(codeownersMembers) {
		reason := fmt.Sprintf("Manual review required: developers.yaml members (%v) must exactly match CODEOWNERS members (%v)",
			devOwners, codeownersMembers)
		logging.Warn("[%s] %s", r.Name(), reason)
		return r.CreateManualReviewResult(reason)
	}

	for i := range devOwners {
		if devOwners[i] != codeownersMembers[i] {
			reason := fmt.Sprintf("Manual review required: developers.yaml members (%v) must exactly match CODEOWNERS members (%v)",
				devOwners, codeownersMembers)
			logging.Warn("[%s] %s", r.Name(), reason)
			return r.CreateManualReviewResult(reason)
		}
	}

	logging.Info("[%s] CODEOWNERS validation passed: %v matches %v", r.Name(), devOwners, codeownersMembers)
	return r.CreateApprovalResult("")
}

// fetchCodeowners fetches the CODEOWNERS file from the repository
func (r *DevelopersRule) fetchCodeowners() (string, error) {
	if r.client == nil {
		return "", fmt.Errorf("GitLab client not available")
	}

	if r.mrContext == nil {
		return "", fmt.Errorf("MR context not available")
	}

	sourceBranch := r.mrContext.MRInfo.SourceBranch
	if sourceBranch == "" {
		return "", fmt.Errorf("source branch not available")
	}

	fileContent, err := r.client.FetchFileContent(r.mrContext.ProjectID, "CODEOWNERS", sourceBranch)
	if err != nil {
		return "", fmt.Errorf("failed to fetch CODEOWNERS: %w", err)
	}

	if fileContent == nil {
		return "", fmt.Errorf("CODEOWNERS content is nil")
	}

	return fileContent.Content, nil
}

// extractCodeownersMembers extracts @usernames from a CODEOWNERS line matching the pattern
func (r *DevelopersRule) extractCodeownersMembers(codeownersContent, pattern string) ([]string, error) {
	lines := strings.Split(codeownersContent, "\n")

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Skip comments and empty lines
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}

		// Check if this line matches our pattern
		if strings.HasPrefix(trimmed, pattern) {
			// Extract all @usernames from this line
			re := regexp.MustCompile(`@[a-zA-Z0-9_-]+`)
			matches := re.FindAllString(trimmed, -1)

			if len(matches) == 0 {
				return nil, fmt.Errorf("no members found in CODEOWNERS line: %s", trimmed)
			}

			return matches, nil
		}
	}

	return nil, fmt.Errorf("pattern not found in CODEOWNERS: %s", pattern)
}

// isAtProductRoot checks if developers.yaml is at the product root
// Expected path: dataproducts/unstructured/{product}/developers.yaml (exactly 4 parts)
// Not at root: dataproducts/unstructured/{product}/sandbox/developers.yaml (5+ parts)
func (r *DevelopersRule) isAtProductRoot(filePath string) bool {
	parts := strings.Split(filePath, "/")

	// Find the "dataproducts" index
	for i, part := range parts {
		if part == "dataproducts" && i+2 < len(parts) && parts[i+1] == "unstructured" {
			// Expected: dataproducts/unstructured/{product}/developers.yaml = 4 parts total
			// Index: i, i+1, i+2, i+3
			expectedLength := i + 4
			return len(parts) == expectedLength
		}
	}

	return false
}
