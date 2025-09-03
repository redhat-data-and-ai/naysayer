package rules

import (
	"fmt"
	"strings"
	"time"

	"github.com/redhat-data-and-ai/naysayer/internal/gitlab"
	"github.com/redhat-data-and-ai/naysayer/internal/rules/shared"
)

// SimpleRuleManager is a concrete implementation of RuleManager
type SimpleRuleManager struct {
	rules []shared.Rule
}

// NewSimpleRuleManager creates a new simple rule manager
func NewSimpleRuleManager() *SimpleRuleManager {
	return &SimpleRuleManager{
		rules: make([]shared.Rule, 0),
	}
}

// AddRule registers a rule with the manager
func (rm *SimpleRuleManager) AddRule(rule shared.Rule) {
	rm.rules = append(rm.rules, rule)
}

// EvaluateAll runs all applicable rules using line-level validation
func (rm *SimpleRuleManager) EvaluateAll(mrCtx *shared.MRContext) *shared.RuleEvaluation {
	start := time.Now()

	// Early filtering for common skip conditions
	if shared.IsDraftMR(mrCtx) {
		return &shared.RuleEvaluation{
			FinalDecision: shared.Decision{
				Type:    shared.Approve,
				Reason:  "Draft MR - auto-approved",
				Summary: "✅ Draft MR skipped",
				Details: "Draft MRs are automatically approved without rule evaluation",
			},

			FileValidations: make(map[string]*shared.FileValidationSummary),
			ExecutionTime:   time.Since(start),
		}
	}

	if shared.IsAutomatedUser(mrCtx) {
		return &shared.RuleEvaluation{
			FinalDecision: shared.Decision{
				Type:    shared.Approve,
				Reason:  "Automated user MR - auto-approved",
				Summary: "🤖 Bot MR skipped",
				Details: "MRs from automated users (bots) are automatically approved",
			},

			FileValidations: make(map[string]*shared.FileValidationSummary),
			ExecutionTime:   time.Since(start),
		}
	}

	// Set MR context for context-aware rules
	rm.setMRContextForRules(mrCtx)

	// Perform line-level validation
	fileValidations, overallDecision := rm.validateFilesLineByLine(mrCtx)

	// Calculate summary statistics
	totalFiles := len(fileValidations)
	approvedFiles := 0
	reviewFiles := 0
	uncoveredFiles := 0

	for _, fileValidation := range fileValidations {
		switch fileValidation.FileDecision {
		case shared.Approve:
			approvedFiles++
		case shared.ManualReview:
			reviewFiles++
			if len(fileValidation.UncoveredLines) > 0 {
				uncoveredFiles++
			}
		}
	}

	return &shared.RuleEvaluation{
		FinalDecision:   overallDecision,
		FileValidations: fileValidations,
		ExecutionTime:   time.Since(start),
		TotalFiles:      totalFiles,
		ApprovedFiles:   approvedFiles,
		ReviewFiles:     reviewFiles,
		UncoveredFiles:  uncoveredFiles,
	}
}

// FileCoverageResult represents the coverage analysis for an MR
type FileCoverageResult struct {
	TotalFiles        int      `json:"total_files"`
	CoveredFiles      int      `json:"covered_files"`
	UncoveredFiles    []string `json:"uncovered_files"`
	HasUncoveredFiles bool     `json:"has_uncovered_files"`
}

// analyzeFileCoverage analyzes which files in the MR are covered by rules
func (rm *SimpleRuleManager) analyzeFileCoverage(mrCtx *shared.MRContext) *FileCoverageResult {
	// Collect all unique file paths from the MR
	allFiles := make(map[string]bool)
	for _, change := range mrCtx.Changes {
		if change.NewPath != "" {
			allFiles[change.NewPath] = true
		}
		if change.OldPath != "" && change.OldPath != change.NewPath {
			allFiles[change.OldPath] = true
		}
	}

	var uncoveredFiles []string
	coveredCount := 0

	// Check coverage for each file
	for filePath := range allFiles {
		// Check if any rule covers this file by calling GetCoveredLines
		covered := false
		for _, rule := range rm.rules {
			fileContent := rm.getFileContent(filePath, mrCtx)
			coveredLines := rule.GetCoveredLines(filePath, fileContent)
			if len(coveredLines) > 0 {
				covered = true
				break
			}
		}

		if covered {
			coveredCount++
		} else {
			uncoveredFiles = append(uncoveredFiles, filePath)
		}
	}

	return &FileCoverageResult{
		TotalFiles:        len(allFiles),
		CoveredFiles:      coveredCount,
		UncoveredFiles:    uncoveredFiles,
		HasUncoveredFiles: len(uncoveredFiles) > 0,
	}
}

// validateFilesLineByLine performs line-level validation for each file
func (rm *SimpleRuleManager) validateFilesLineByLine(mrCtx *shared.MRContext) (map[string]*shared.FileValidationSummary, shared.Decision) {
	fileValidations := make(map[string]*shared.FileValidationSummary)

	// Get unique file paths from changes
	filePaths := rm.getUniqueFilePaths(mrCtx.Changes)

	for _, filePath := range filePaths {
		// Fetch file content from GitLab API
		// Note: In production, this would use the GitLab client from mrCtx
		// For now, we'll simulate with a placeholder
		fileContent := rm.getFileContent(filePath, mrCtx)
		totalLines := shared.CountLines(fileContent)

		// Validate this file with all applicable rules
		fileValidation := rm.validateSingleFile(filePath, fileContent, totalLines)
		fileValidations[filePath] = fileValidation
	}

	// Determine overall decision
	overallDecision := rm.determineOverallDecision(fileValidations)
	return fileValidations, overallDecision
}

// setMRContextForRules provides MR context to context-aware rules
func (rm *SimpleRuleManager) setMRContextForRules(mrCtx *shared.MRContext) {
	for _, rule := range rm.rules {
		// Check if the rule implements ContextAwareRule interface
		if contextRule, ok := rule.(shared.ContextAwareRule); ok {
			contextRule.SetMRContext(mrCtx)
		}
	}
}

// validateSingleFile validates a single file using all applicable rules
func (rm *SimpleRuleManager) validateSingleFile(filePath, fileContent string, totalLines int) *shared.FileValidationSummary {
	var allCoveredLines []shared.LineRange
	var ruleResults []shared.LineValidationResult

	// For each rule, check if it covers any lines in this file
	for _, rule := range rm.rules {
		coveredLines := rule.GetCoveredLines(filePath, fileContent)
		if len(coveredLines) == 0 {
			continue // Rule doesn't apply to this file
		}

		// Validate the lines covered by this rule
		decision, reason := rule.ValidateLines(filePath, fileContent, coveredLines)

		// Add rule result
		ruleResults = append(ruleResults, shared.LineValidationResult{
			RuleName:   rule.Name(),
			LineRanges: coveredLines,
			Decision:   decision,
			Reason:     reason,
		})

		// Accumulate covered lines
		allCoveredLines = append(allCoveredLines, coveredLines...)
	}

	// Calculate uncovered lines
	uncoveredLines := shared.GetUncoveredLines(totalLines, allCoveredLines)

	// Determine file decision
	fileDecision := rm.determineFileDecision(ruleResults, uncoveredLines)

	return &shared.FileValidationSummary{
		FilePath:       filePath,
		TotalLines:     totalLines,
		CoveredLines:   shared.MergeLineRanges(allCoveredLines),
		UncoveredLines: uncoveredLines,
		RuleResults:    ruleResults,
		FileDecision:   fileDecision,
	}
}

// determineFileDecision determines the decision for a single file
func (rm *SimpleRuleManager) determineFileDecision(ruleResults []shared.LineValidationResult, uncoveredLines []shared.LineRange) shared.DecisionType {
	// If there are uncovered lines, require manual review
	if len(uncoveredLines) > 0 {
		return shared.ManualReview
	}

	// If any rule requires manual review, the file requires manual review
	for _, result := range ruleResults {
		if result.Decision == shared.ManualReview {
			return shared.ManualReview
		}
	}

	// All rules approved and full coverage
	return shared.Approve
}

// determineOverallDecision determines the overall MR decision
func (rm *SimpleRuleManager) determineOverallDecision(fileValidations map[string]*shared.FileValidationSummary) shared.Decision {
	approvedFiles := 0
	reviewFiles := 0
	uncoveredFiles := 0

	for _, fileValidation := range fileValidations {
		if fileValidation.FileDecision == shared.Approve {
			approvedFiles++
		} else {
			reviewFiles++
			if len(fileValidation.UncoveredLines) > 0 {
				uncoveredFiles++
			}
		}
	}

	// If any file requires manual review, the MR requires manual review
	if reviewFiles > 0 {
		return shared.Decision{
			Type:    shared.ManualReview,
			Reason:  fmt.Sprintf("Manual review required: %d/%d files need review", reviewFiles, len(fileValidations)),
			Summary: "🚫 Manual review required",
			Details: rm.createDetailedSummary(fileValidations),
		}
	}

	// All files approved
	return shared.Decision{
		Type:    shared.Approve,
		Reason:  fmt.Sprintf("All %d files validated and approved", len(fileValidations)),
		Summary: "✅ All files approved",
		Details: rm.createDetailedSummary(fileValidations),
	}
}

// Helper methods
func (rm *SimpleRuleManager) getUniqueFilePaths(changes []gitlab.FileChange) []string {
	fileMap := make(map[string]bool)
	for _, change := range changes {
		if change.NewPath != "" {
			fileMap[change.NewPath] = true
		}
		if change.OldPath != "" && change.OldPath != change.NewPath {
			fileMap[change.OldPath] = true
		}
	}

	var filePaths []string
	for filePath := range fileMap {
		filePaths = append(filePaths, filePath)
	}
	return filePaths
}

// getFileContent fetches file content from GitLab API
func (rm *SimpleRuleManager) getFileContent(filePath string, mrCtx *shared.MRContext) string {
	// For now, return placeholder content until a GitLab client is available in the manager
	// In production, this should use a GitLab client to fetch actual file content

	// The current line-level validation system needs actual file content to work properly
	// This placeholder provides realistic content for testing the validation logic

	// TODO: Add GitLab client to SimpleRuleManager and implement actual file fetching:
	// client.FetchFileContent(mrCtx.ProjectID, filePath, mrCtx.SourceBranch)

	return "# Placeholder file content\nname: test-product\nkind: aggregated\nwarehouses:\n- type: user\n  size: XSMALL\n"
}

// createDetailedSummary creates a detailed summary of file validations
func (rm *SimpleRuleManager) createDetailedSummary(fileValidations map[string]*shared.FileValidationSummary) string {
	var summary strings.Builder
	summary.WriteString("File validation results:\n")

	for filePath, validation := range fileValidations {
		if validation.FileDecision == shared.Approve {
			summary.WriteString(fmt.Sprintf("✅ %s: Approved\n", filePath))
		} else {
			summary.WriteString(fmt.Sprintf("❌ %s: Manual review required\n", filePath))
			if len(validation.UncoveredLines) > 0 {
				summary.WriteString(fmt.Sprintf("   - Uncovered lines: %d ranges\n", len(validation.UncoveredLines)))
			}
			for _, ruleResult := range validation.RuleResults {
				if ruleResult.Decision == shared.ManualReview {
					summary.WriteString(fmt.Sprintf("   - %s: %s\n", ruleResult.RuleName, ruleResult.Reason))
				}
			}
		}
	}

	return summary.String()
}