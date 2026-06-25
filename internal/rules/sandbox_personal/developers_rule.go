package sandbox_personal

import (
	"fmt"
	"strings"

	"github.com/redhat-data-and-ai/naysayer/internal/gitlab"
	"github.com/redhat-data-and-ai/naysayer/internal/logging"
	"github.com/redhat-data-and-ai/naysayer/internal/rules/common"
	"github.com/redhat-data-and-ai/naysayer/internal/rules/shared"
	"gopkg.in/yaml.v3"
)

// DevelopersRule validates that product-level developers.yaml files
// for Personal UnstructuredDataProducts have exactly 1 owner and cannot be changed
type DevelopersRule struct {
	*common.BaseRule
	*common.ValidationHelper
	client    gitlab.GitLabClient
	mrContext *shared.MRContext
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
func NewDevelopersRule(client gitlab.GitLabClient) *DevelopersRule {
	return &DevelopersRule{
		BaseRule:         common.NewBaseRule("sandbox_developers_rule", "Validates exactly 1 developer for sandbox Personal UnstructuredDataProduct"),
		ValidationHelper: common.NewValidationHelper(),
		client:           client,
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

	// Only apply when the associated product is a sandbox Personal UnstructuredDataProduct
	if !IsSandboxPersonalProductForFile(r.mrContext, r.client, filePath) {
		return r.CreateApprovalResult("Auto-approved: Not a sandbox Personal UnstructuredDataProduct")
	}

	// This MR IS a sandbox Personal context - now check if THIS file is developers.yaml at product root
	if !strings.Contains(filePath, "developers.yaml") && !strings.Contains(filePath, "developers.yml") {
		return r.CreateApprovalResult("Auto-approved: Not a developers.yaml file")
	}

	// Skip if inside sandbox/dev/preprod/prod folders (not at product root)
	if strings.Contains(filePath, "/sandbox/") || strings.Contains(filePath, "/dev/") ||
		strings.Contains(filePath, "/preprod/") || strings.Contains(filePath, "/prod/") {
		return r.CreateApprovalResult("Auto-approved: developers.yaml not at product root")
	}

	// Parse current content
	var currentDevelopers DevelopersYAML
	err := yaml.Unmarshal([]byte(fileContent), &currentDevelopers)
	if err != nil {
		logging.Error("[%s] Failed to parse developers.yaml: %v", r.Name(), err)
		return r.CreateManualReviewResult(fmt.Sprintf("Manual review required: Failed to parse developers.yaml: %v", err))
	}

	currentCount := len(currentDevelopers.Group.Owners)

	// Check if this is a NEW file
	isNewFile := isNewFileInMR(r.mrContext, r.client, filePath)

	if isNewFile {
		logging.Info("[%s] NEW developers.yaml detected, validating count=1", r.Name())

		// NEW file must have exactly 1 owner
		if currentCount != 1 {
			reason := fmt.Sprintf("Manual review required: New developers.yaml must have exactly 1 developer, found: %d", currentCount)
			logging.Warn("[%s] %s", r.Name(), reason)
			return r.CreateManualReviewResult(reason)
		}

		return r.CreateApprovalResult("Auto-approved: New developers.yaml with 1 developer")
	}

	// EXISTING file - validate count=1 and no changes
	logging.Info("[%s] EXISTING developers.yaml detected, validating count=1 and unchanged", r.Name())

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

	// Validate both current and previous have exactly 1 owner
	if currentCount != 1 || previousCount != 1 {
		reason := fmt.Sprintf("Manual review required: Must maintain exactly 1 developer (current: %d, previous: %d)", currentCount, previousCount)
		logging.Warn("[%s] %s", r.Name(), reason)
		return r.CreateManualReviewResult(reason)
	}

	// Check if the owner has changed
	if currentDevelopers.Group.Owners[0] != previousDevelopers.Group.Owners[0] {
		reason := fmt.Sprintf("Manual review required: Developer cannot be changed (from: %s, to: %s)",
			previousDevelopers.Group.Owners[0],
			currentDevelopers.Group.Owners[0])
		logging.Warn("[%s] %s", r.Name(), reason)
		return r.CreateManualReviewResult(reason)
	}

	// No changes detected
	return r.CreateApprovalResult("Auto-approved: developers.yaml unchanged with 1 developer")
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
