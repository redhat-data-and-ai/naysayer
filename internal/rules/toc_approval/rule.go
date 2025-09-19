package toc_approval

import (
	"strings"

	"github.com/redhat-data-and-ai/naysayer/internal/rules/common"
	"github.com/redhat-data-and-ai/naysayer/internal/rules/shared"
)

// TOCApprovalRule requires TOC approval for new product.yaml files in preprod/prod environments
type TOCApprovalRule struct {
	*common.BaseRule
	*common.FileTypeMatcher
	*common.ValidationHelper
	config *TOCEnvironmentConfig
}

// NewTOCApprovalRule creates a new TOC approval rule instance
func NewTOCApprovalRule(preprodProdEnvs []string) *TOCApprovalRule {
	config := DefaultTOCEnvironmentConfig()
	if preprodProdEnvs != nil {
		config.RequiredEnvironments = preprodProdEnvs
	}

	return &TOCApprovalRule{
		BaseRule:         common.NewBaseRule("toc_approval_rule", "Requires TOC approval for new product.yaml files in preprod/prod environments"),
		FileTypeMatcher:  common.NewFileTypeMatcher(),
		ValidationHelper: common.NewValidationHelper(),
		config:           config,
	}
}

// ValidateLines validates lines for TOC approval requirements
func (r *TOCApprovalRule) ValidateLines(filePath string, fileContent string, lineRanges []shared.LineRange) (shared.DecisionType, string) {
	// Only apply to product.yaml files
	if !r.IsProductFile(filePath) {
		return r.CreateApprovalResult("Not a product.yaml file - no TOC approval required")
	}

	// Analyze the context for this file
	context := r.analyzeFile(filePath)

	if context.RequiresApproval {
		return r.CreateManualReviewResult(context.ApprovalReason)
	}

	// For existing files or non-critical environments, no TOC approval needed
	return r.CreateApprovalResult("Existing product.yaml file or not in critical environment - no TOC approval required")
}

// GetCoveredLines returns line ranges this rule covers
func (r *TOCApprovalRule) GetCoveredLines(filePath string, fileContent string) []shared.LineRange {
	// Only cover product.yaml files
	if !r.IsProductFile(filePath) {
		return []shared.LineRange{}
	}

	context := r.analyzeFile(filePath)

	// Cover the entire file for new products in critical environments
	if context.RequiresApproval {
		return r.GetFullFileCoverage(filePath, fileContent)
	}

	// Return minimal coverage for existing files to participate in validation
	return []shared.LineRange{
		{
			StartLine: 1,
			EndLine:   1,
			FilePath:  filePath,
		},
	}
}

// analyzeFile analyzes a file to determine if TOC approval is required
func (r *TOCApprovalRule) analyzeFile(filePath string) *TOCApprovalContext {
	context := &TOCApprovalContext{
		FilePath:         filePath,
		IsNewFile:        r.isNewFile(filePath),
		Environment:      r.extractEnvironmentFromPath(filePath),
		RequiresApproval: false,
	}

	// Check if this is a new file in a critical environment
	if context.IsNewFile && r.isEnvironmentCritical(context.Environment) {
		context.RequiresApproval = true
		context.ApprovalReason = r.getTOCApprovalReason(filePath, context.Environment)
	}

	return context
}

// isNewFile checks if this file is being added (new file)
func (r *TOCApprovalRule) isNewFile(filePath string) bool {
	if r.GetMRContext() == nil {
		return false
	}

	// Check if this file is being added (new file)
	for _, change := range r.GetMRContext().Changes {
		if change.NewPath == filePath && change.NewFile {
			return true
		}
	}

	return false
}

// isEnvironmentCritical checks if the environment requires TOC approval
func (r *TOCApprovalRule) isEnvironmentCritical(environment string) bool {
	if environment == "" {
		return false
	}

	for _, criticalEnv := range r.config.RequiredEnvironments {
		if r.config.CaseSensitive {
			if environment == criticalEnv {
				return true
			}
		} else {
			if strings.EqualFold(environment, criticalEnv) {
				return true
			}
		}
	}

	return false
}

// extractEnvironmentFromPath attempts to extract the environment name from the file path
func (r *TOCApprovalRule) extractEnvironmentFromPath(filePath string) string {
	lowerPath := strings.ToLower(filePath)

	for _, env := range r.config.RequiredEnvironments {
		lowerEnv := strings.ToLower(env)
		if strings.Contains(lowerPath, "/"+lowerEnv+"/") ||
			strings.Contains(lowerPath, "/"+lowerEnv+"_") ||
			strings.Contains(lowerPath, "_"+lowerEnv+"/") ||
			strings.Contains(lowerPath, "_"+lowerEnv+"_") {
			return env
		}
	}

	return ""
}

// getTOCApprovalReason returns a detailed reason for requiring TOC approval
func (r *TOCApprovalRule) getTOCApprovalReason(filePath, environment string) string {
	if environment != "" {
		return "Manual review required: New data product being promoted to " + environment + " environment requires TOC (Technical Oversight Committee) approval before deployment"
	}

	return "Manual review required: New data product in critical environment requires TOC (Technical Oversight Committee) approval before deployment"
}
