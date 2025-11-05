package webhook

import (
	"fmt"
	"sort"
	"strings"

	"github.com/redhat-data-and-ai/naysayer/internal/config"
	"github.com/redhat-data-and-ai/naysayer/internal/gitlab"
	"github.com/redhat-data-and-ai/naysayer/internal/rules/shared"
)

// MessageBuilder handles creation of MR comments and approval messages
type MessageBuilder struct {
	config *config.Config
}

// NewMessageBuilder creates a new message builder
func NewMessageBuilder(cfg *config.Config) *MessageBuilder {
	return &MessageBuilder{config: cfg}
}

// BuildApprovalComment creates a detailed comment for the MR explaining the approval decision
func (mb *MessageBuilder) BuildApprovalComment(result *shared.RuleEvaluation, mrInfo *gitlab.MRInfo) string {
	var comment strings.Builder

	// Hidden identifier for comment tracking
	comment.WriteString("<!-- naysayer-comment-id: approval -->\n")

	// Header
	comment.WriteString("✅ **Auto-approved**\n\n")

	// Analysis results based on verbosity
	switch mb.config.Comments.CommentVerbosity {
	case "basic":
		comment.WriteString(mb.buildBasicSummary(result))
	case "debug":
		comment.WriteString(mb.buildDebugSummary(result, mrInfo))
	default: // "detailed"
		comment.WriteString(mb.buildDetailedSummary(result))
	}

	return comment.String()
}

// BuildManualReviewComment creates a detailed comment for MRs requiring manual review
func (mb *MessageBuilder) BuildManualReviewComment(result *shared.RuleEvaluation, mrInfo *gitlab.MRInfo) string {
	var comment strings.Builder

	// Hidden identifier for comment tracking
	comment.WriteString("<!-- naysayer-comment-id: manual-review -->\n")

	// Header
	comment.WriteString("⚠️ **Manual review required**\n\n")

	// Analysis results based on verbosity
	switch mb.config.Comments.CommentVerbosity {
	case "basic":
		comment.WriteString(mb.buildBasicManualReviewSummary(result))
	case "debug":
		comment.WriteString(mb.buildDebugManualReviewSummary(result, mrInfo))
	default: // "detailed"
		comment.WriteString(mb.buildDetailedManualReviewSummary(result))
	}
	return comment.String()
}

// buildBasicSummary creates a basic approval summary
func (mb *MessageBuilder) buildBasicSummary(result *shared.RuleEvaluation) string {
	var summary strings.Builder

	summary.WriteString("**What was checked:**\n")
	summary.WriteString(mb.buildRulesSummary(result.FileValidations))

	return summary.String()
}

// buildDetailedSummary creates a detailed approval summary
func (mb *MessageBuilder) buildDetailedSummary(result *shared.RuleEvaluation) string {
	var summary strings.Builder

	// Collapsible section for cleaner comments
	summary.WriteString("<details>\n")
	summary.WriteString("<summary>📋 <strong>Analysis Details</strong> (click to expand)</summary>\n\n")

	// File list if 3+ files
	if result.TotalFiles >= 3 {
		if filesSummary := mb.buildFilesSummary(result); filesSummary != "" {
			summary.WriteString("**Files in this MR:**\n")
			summary.WriteString(filesSummary)
			summary.WriteString("\n")
		}
	}

	// What was checked
	summary.WriteString("**What was checked:**\n")
	summary.WriteString(mb.buildRulesSummary(result.FileValidations))

	summary.WriteString("\n</details>")

	return summary.String()
}

// buildDebugSummary creates a verbose debug summary
func (mb *MessageBuilder) buildDebugSummary(result *shared.RuleEvaluation, mrInfo *gitlab.MRInfo) string {
	var summary strings.Builder

	// MR Information
	summary.WriteString("🔍 **MR Information:**\n")
	summary.WriteString(fmt.Sprintf("• Project ID: %d\n", mrInfo.ProjectID))
	summary.WriteString(fmt.Sprintf("• MR IID: %d\n", mrInfo.MRIID))
	summary.WriteString(fmt.Sprintf("• Author: %s\n", mrInfo.Author))
	summary.WriteString(fmt.Sprintf("• Title: %s\n", mrInfo.Title))
	summary.WriteString("\n")

	// File changes with metadata
	if filesSummary := mb.buildDetailedFilesSummary(result); filesSummary != "" {
		summary.WriteString("📄 **Detailed File Analysis:**\n")
		summary.WriteString(filesSummary)
		summary.WriteString("\n")
	}

	// Detailed analysis results
	summary.WriteString("📊 **Detailed Analysis Results:**\n")
	summary.WriteString(mb.buildDetailedRulesSummary(result.FileValidations))

	return summary.String()
}

// buildRulesSummary creates a summary of rule evaluation results, filtering out noise
func (mb *MessageBuilder) buildRulesSummary(fileValidations map[string]*shared.FileValidationSummary) string {
	var summary strings.Builder
	ruleMessages := make(map[string]string)

	// Collect messages by rule, filtering out noise
	for _, fileValidation := range fileValidations {
		for _, ruleResult := range fileValidation.RuleResults {
			// Skip noise messages
			if mb.isNoiseMessage(ruleResult.Reason) {
				continue
			}

			// Skip rules that didn't actually validate anything
			if len(ruleResult.LineRanges) == 0 {
				continue
			}

			ruleName := mb.formatRuleName(ruleResult.RuleName)

			switch ruleResult.Decision {
			case shared.Approve:
				// Only store if not already present
				if _, exists := ruleMessages[ruleName]; !exists {
					explanation := mb.getApprovalExplanation(ruleResult.RuleName, ruleResult.Reason)
					ruleMessages[ruleName] = fmt.Sprintf("✅ %s%s", ruleName, explanation)
				}
			case shared.ManualReview:
				// Manual review messages always override
				ruleMessages[ruleName] = fmt.Sprintf("🚫 %s: %s", ruleName, ruleResult.Reason)
			}
		}
	}

	// Sort rule names alphabetically for consistent output
	var ruleNames []string
	for ruleName := range ruleMessages {
		ruleNames = append(ruleNames, ruleName)
	}
	sort.Strings(ruleNames)

	// Output unique rule messages in sorted order
	for _, ruleName := range ruleNames {
		summary.WriteString(fmt.Sprintf("• %s\n", ruleMessages[ruleName]))
	}

	return summary.String()
}

// isNoiseMessage checks if a message should be filtered out
func (mb *MessageBuilder) isNoiseMessage(message string) bool {
	noisePatterns := []string{
		"Not a ",
		"No warehouse size changes detected",
		"No changes detected",
	}

	for _, pattern := range noisePatterns {
		if strings.HasPrefix(message, pattern) {
			return true
		}
	}
	return false
}

// formatRuleName converts internal rule names to user-friendly descriptions
func (mb *MessageBuilder) formatRuleName(ruleName string) string {
	friendlyNames := map[string]string{
		"warehouse_rule":            "Warehouse configuration validated",
		"service_account_rule":      "Service account validated",
		"toc_approval_rule":         "TOC approval check",
		"metadata_rule":             "Metadata validated",
		"dataproduct_consumer_rule": "Consumer access changes validated",
	}

	if friendly, ok := friendlyNames[ruleName]; ok {
		return friendly
	}
	return ruleName
}

// getApprovalExplanation provides meaningful context for why a rule auto-approved
func (mb *MessageBuilder) getApprovalExplanation(ruleName string, reason string) string {
	switch ruleName {
	case "metadata_rule":
		return ": Product metadata changes (name, tags, kind) are safe and don't affect infrastructure"
	case "dataproduct_consumer_rule":
		return ": Data product owner can grant consumer access without TOC approval"
	case "warehouse_rule":
		if strings.Contains(reason, "decrease") {
			return ": Warehouse size decrease saves costs and is safe to auto-approve"
		}
		return ": Warehouse configuration changes validated"
	case "toc_approval_rule":
		return ": Changes to existing products in non-critical environments"
	case "service_account_rule":
		return ": Service account configuration is valid"
	default:
		return ""
	}
}

// hasUncoveredFiles checks if there are files without validation rules
func (mb *MessageBuilder) hasUncoveredFiles(result *shared.RuleEvaluation) bool {
	for _, fileValidation := range result.FileValidations {
		if len(fileValidation.RuleResults) == 0 && fileValidation.FileDecision == shared.ManualReview {
			return true
		}
	}
	return false
}

// getUncoveredReason returns a user-friendly reason for why a file is uncovered
func (mb *MessageBuilder) getUncoveredReason(filePath string) string {
	if strings.HasSuffix(filePath, ".sql") {
		return "No validation rules configured for SQL migrations"
	} else if strings.HasSuffix(filePath, ".sh") {
		return "No validation rules configured for shell scripts"
	} else if strings.HasSuffix(filePath, ".py") {
		return "No validation rules configured for Python scripts"
	}
	return "No validation rules configured for this file type"
}

// buildDetailedRulesSummary creates a detailed summary with metadata from file validations
func (mb *MessageBuilder) buildDetailedRulesSummary(fileValidations map[string]*shared.FileValidationSummary) string {
	var summary strings.Builder
	ruleDetails := make(map[string]*shared.LineValidationResult)

	// Collect detailed rule results from file validations
	for _, fileValidation := range fileValidations {
		for _, ruleResult := range fileValidation.RuleResults {
			// Skip noise messages
			if mb.isNoiseMessage(ruleResult.Reason) {
				continue
			}

			// Skip rules that didn't validate anything
			if len(ruleResult.LineRanges) == 0 {
				continue
			}

			// Use the first occurrence of each rule for detailed display
			if _, exists := ruleDetails[ruleResult.RuleName]; !exists {
				ruleDetails[ruleResult.RuleName] = &ruleResult
			}
		}
	}

	// Sort rule names alphabetically for consistent output
	var ruleNames []string
	for ruleName := range ruleDetails {
		ruleNames = append(ruleNames, ruleName)
	}
	sort.Strings(ruleNames)

	// Output detailed rule information in sorted order
	for _, ruleName := range ruleNames {
		result := ruleDetails[ruleName]
		friendlyName := mb.formatRuleName(ruleName)

		switch result.Decision {
		case shared.Approve:
			summary.WriteString(fmt.Sprintf("• ✅ **%s**\n", friendlyName))
		case shared.ManualReview:
			summary.WriteString(fmt.Sprintf("• 🚫 **%s**: %s\n", friendlyName, result.Reason))
		}
	}

	return summary.String()
}

// buildFilesSummary creates a summary of analyzed files
func (mb *MessageBuilder) buildFilesSummary(result *shared.RuleEvaluation) string {
	var summary strings.Builder

	// Collect and sort file paths
	var filePaths []string
	for filePath := range result.FileValidations {
		filePaths = append(filePaths, filePath)
	}
	sort.Strings(filePaths)

	// Output sorted file list
	for _, filePath := range filePaths {
		fileValidation := result.FileValidations[filePath]
		summary.WriteString(fmt.Sprintf("• `%s`", filePath))

		// Add decision status
		switch fileValidation.FileDecision {
		case shared.Approve:
			summary.WriteString(" ✅")
		case shared.ManualReview:
			summary.WriteString(" 🚫")
		}

		summary.WriteString("\n")
	}

	return summary.String()
}

// buildDetailedFilesSummary creates a detailed summary with more metadata
func (mb *MessageBuilder) buildDetailedFilesSummary(result *shared.RuleEvaluation) string {
	var summary strings.Builder

	// Collect and sort file paths
	var filePaths []string
	for filePath := range result.FileValidations {
		filePaths = append(filePaths, filePath)
	}
	sort.Strings(filePaths)

	// Output sorted detailed file list
	for _, filePath := range filePaths {
		fileValidation := result.FileValidations[filePath]
		summary.WriteString(fmt.Sprintf("**File: `%s`**\n", filePath))
		summary.WriteString(fmt.Sprintf("• Total lines: %d\n", fileValidation.TotalLines))
		summary.WriteString(fmt.Sprintf("• Decision: %s\n", fileValidation.FileDecision))

		// List rules that validated this file (filtered)
		if len(fileValidation.RuleResults) > 0 {
			summary.WriteString("• Rules applied:\n")
			for _, ruleResult := range fileValidation.RuleResults {
				if !mb.isNoiseMessage(ruleResult.Reason) && len(ruleResult.LineRanges) > 0 {
					friendlyName := mb.formatRuleName(ruleResult.RuleName)
					summary.WriteString(fmt.Sprintf("  - %s: %s\n", friendlyName, ruleResult.Reason))
				}
			}
		}

		summary.WriteString("\n")
	}

	return summary.String()
}

// BuildApprovalMessage creates a short message for the approval API
func (mb *MessageBuilder) BuildApprovalMessage(result *shared.RuleEvaluation) string {
	// Analyze the results to create a meaningful short message
	switch {
	case mb.hasWarehouseChanges(result):
		return "Auto-approved: Warehouse changes are safe (decreases only)"
	case mb.isAutomatedUser(result):
		return "Auto-approved: Automated user with passing CI"
	case mb.hasOnlyDataverseFiles(result):
		return "Auto-approved: Only dataverse-safe files modified"
	default:
		return "Auto-approved: All rules passed"
	}
}

// hasWarehouseChanges checks if warehouse changes were detected and approved
func (mb *MessageBuilder) hasWarehouseChanges(result *shared.RuleEvaluation) bool {
	for _, fileValidation := range result.FileValidations {
		for _, ruleResult := range fileValidation.RuleResults {
			if ruleResult.RuleName == "warehouse_rule" && ruleResult.Decision == shared.Approve {
				return true
			}
		}
	}
	return false
}

// isAutomatedUser checks if this was an automated user approval
func (mb *MessageBuilder) isAutomatedUser(result *shared.RuleEvaluation) bool {
	return strings.Contains(result.FinalDecision.Reason, "Automated user")
}

// hasOnlyDataverseFiles checks if only dataverse files were modified
func (mb *MessageBuilder) hasOnlyDataverseFiles(result *shared.RuleEvaluation) bool {
	return result.ApprovedFiles == result.TotalFiles && result.TotalFiles > 0
}

// buildBasicManualReviewSummary creates a basic manual review summary
func (mb *MessageBuilder) buildBasicManualReviewSummary(result *shared.RuleEvaluation) string {
	var summary strings.Builder

	summary.WriteString(fmt.Sprintf("**Why manual review is needed:**\n%s\n\n", result.FinalDecision.Reason))

	summary.WriteString("**What was checked:**\n")
	summary.WriteString(mb.buildRulesSummary(result.FileValidations))

	return summary.String()
}

// buildDetailedManualReviewSummary creates a detailed manual review summary
func (mb *MessageBuilder) buildDetailedManualReviewSummary(result *shared.RuleEvaluation) string {
	var summary strings.Builder

	// Enhanced decision with WHY explanation
	if mb.hasUncoveredFiles(result) {
		summary.WriteString("**Why manual review is needed:**\n\n")
		summary.WriteString("This MR contains files that Naysayer doesn't know how to validate.\n\n")

		// Group files by reason
		filesByReason := make(map[string][]string)
		for filePath, fileValidation := range result.FileValidations {
			if fileValidation.FileDecision == shared.ManualReview && len(fileValidation.RuleResults) == 0 {
				reason := mb.getUncoveredReason(filePath)
				filesByReason[reason] = append(filesByReason[reason], filePath)
			}
		}

		// Sort reasons alphabetically
		var reasons []string
		for reason := range filesByReason {
			reasons = append(reasons, reason)
		}
		sort.Strings(reasons)

		// Use collapsible details for long lists
		totalFiles := 0
		for _, files := range filesByReason {
			totalFiles += len(files)
		}

		// Always use collapsible section for file lists
		summary.WriteString("<details>\n")
		summary.WriteString(fmt.Sprintf("<summary><b>Files requiring manual approval (no automated rules configured)</b> (%d file", totalFiles))
		if totalFiles != 1 {
			summary.WriteString("s")
		}
		summary.WriteString(")</summary>\n\n")

		for _, reason := range reasons {
			files := filesByReason[reason]
			sort.Strings(files)
			summary.WriteString(fmt.Sprintf("**%s** (%d file", reason, len(files)))
			if len(files) != 1 {
				summary.WriteString("s")
			}
			summary.WriteString(")\n")
			for _, filePath := range files {
				summary.WriteString(fmt.Sprintf("- `%s`\n", filePath))
			}
			summary.WriteString("\n")
		}
		summary.WriteString("</details>\n")
	} else {
		summary.WriteString(fmt.Sprintf("**Why manual review is needed:**\n%s\n\n", result.FinalDecision.Reason))

		// Collapsible section for analysis details
		summary.WriteString("<details>\n")
		summary.WriteString("<summary>📋 <strong>Analysis Details</strong> (click to expand)</summary>\n\n")

		// Show file list if 3+ files
		if result.TotalFiles >= 3 {
			summary.WriteString("**Files in this MR:**\n")
			summary.WriteString(mb.buildFilesSummary(result))
			summary.WriteString("\n")
		}

		summary.WriteString("**What was checked:**\n")
		summary.WriteString(mb.buildRulesSummary(result.FileValidations))

		summary.WriteString("\n</details>")
	}

	return summary.String()
}

// buildDebugManualReviewSummary creates a verbose debug summary for manual review
func (mb *MessageBuilder) buildDebugManualReviewSummary(result *shared.RuleEvaluation, mrInfo *gitlab.MRInfo) string {
	var summary strings.Builder

	// MR Information
	summary.WriteString("🔍 **MR Information:**\n")
	summary.WriteString(fmt.Sprintf("• Project ID: %d\n", mrInfo.ProjectID))
	summary.WriteString(fmt.Sprintf("• MR IID: %d\n", mrInfo.MRIID))
	summary.WriteString(fmt.Sprintf("• Author: %s\n", mrInfo.Author))
	summary.WriteString(fmt.Sprintf("• Title: %s\n", mrInfo.Title))
	summary.WriteString("\n")

	// File changes with metadata
	if filesSummary := mb.buildDetailedFilesSummary(result); filesSummary != "" {
		summary.WriteString("📄 **Detailed File Analysis:**\n")
		summary.WriteString(filesSummary)
		summary.WriteString("\n")
	}

	// Detailed analysis results
	summary.WriteString("📊 **Detailed Analysis Results:**\n")
	summary.WriteString(mb.buildDetailedRulesSummary(result.FileValidations))
	summary.WriteString("\n")

	// System information (debug mode keeps some details)
	summary.WriteString("⚙️ **System Details:**\n")
	summary.WriteString(fmt.Sprintf("• Rule evaluation time: %v\n", result.ExecutionTime))
	summary.WriteString(fmt.Sprintf("• Total files analyzed: %d\n", result.TotalFiles))
	summary.WriteString(fmt.Sprintf("• Final decision: %s\n", result.FinalDecision.Type))

	return summary.String()
}
