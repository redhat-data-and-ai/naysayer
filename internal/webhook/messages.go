package webhook

import (
	"fmt"
	"strings"
	"time"

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

	// Header
	comment.WriteString("✅ **Auto-approved after CI pipeline success**\n\n")

	// Analysis results based on verbosity
	switch mb.config.Comments.CommentVerbosity {
	case "basic":
		comment.WriteString(mb.buildBasicSummary(result))
	case "debug":
		comment.WriteString(mb.buildDebugSummary(result, mrInfo))
	default: // "detailed"
		comment.WriteString(mb.buildDetailedSummary(result))
	}

	// Footer
	comment.WriteString("\n🤖 *Automated by NAYSAYER v1.0.0*")

	return comment.String()
}

// BuildManualReviewComment creates a detailed comment for MRs requiring manual review
func (mb *MessageBuilder) BuildManualReviewComment(result *shared.RuleEvaluation, mrInfo *gitlab.MRInfo) string {
	var comment strings.Builder

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

	summary.WriteString("📊 **Analysis Results:**\n")
	summary.WriteString(fmt.Sprintf("• %d files analyzed, all approved\n", result.ApprovedFiles))
	summary.WriteString(fmt.Sprintf("• Decision: %s\n", result.FinalDecision.Reason))

	return summary.String()
}

// buildDetailedSummary creates a detailed approval summary
func (mb *MessageBuilder) buildDetailedSummary(result *shared.RuleEvaluation) string {
	var summary strings.Builder

	// Analysis results
	summary.WriteString("📊 **Analysis Results:**\n")
	summary.WriteString(mb.buildRulesSummary(result.FileValidations))
	summary.WriteString("\n")

	// File changes summary
	if filesSummary := mb.buildFilesSummary(result); filesSummary != "" {
		summary.WriteString("📄 **Files Analyzed:**\n")
		summary.WriteString(filesSummary)
		summary.WriteString("\n")
	}

	// Timing information
	summary.WriteString("⏱️ **Processing Details:**\n")
	summary.WriteString(fmt.Sprintf("• Rule evaluation time: %v\n", result.ExecutionTime))
	summary.WriteString(fmt.Sprintf("• Approved at: %s\n", time.Now().Format("2006-01-02 15:04:05 UTC")))

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

	// Detailed analysis results
	summary.WriteString("📊 **Detailed Analysis Results:**\n")
	summary.WriteString(mb.buildDetailedRulesSummary(result.FileValidations))
	summary.WriteString("\n")

	// File changes with metadata
	if filesSummary := mb.buildDetailedFilesSummary(result); filesSummary != "" {
		summary.WriteString("📄 **Detailed File Analysis:**\n")
		summary.WriteString(filesSummary)
		summary.WriteString("\n")
	}

	// System information
	summary.WriteString("⚙️ **System Details:**\n")
	summary.WriteString(fmt.Sprintf("• Rule evaluation time: %v\n", result.ExecutionTime))
	summary.WriteString(fmt.Sprintf("• Total files analyzed: %d\n", result.TotalFiles))
	summary.WriteString(fmt.Sprintf("• Final decision: %s\n", result.FinalDecision.Type))
	summary.WriteString(fmt.Sprintf("• Approved at: %s\n", time.Now().Format("2006-01-02 15:04:05 UTC")))

	return summary.String()
}

// buildRulesSummary creates a summary of rule evaluation results from file validations
func (mb *MessageBuilder) buildRulesSummary(fileValidations map[string]*shared.FileValidationSummary) string {
	var summary strings.Builder
	ruleMessages := make(map[string][]string)

	// Collect messages by rule
	for _, fileValidation := range fileValidations {
		for _, ruleResult := range fileValidation.RuleResults {
			switch ruleResult.Decision {
			case shared.Approve:
				ruleMessages[ruleResult.RuleName] = append(ruleMessages[ruleResult.RuleName], 
					fmt.Sprintf("✅ %s: %s", ruleResult.RuleName, ruleResult.Reason))
			case shared.ManualReview:
				ruleMessages[ruleResult.RuleName] = append(ruleMessages[ruleResult.RuleName], 
					fmt.Sprintf("🚫 %s: %s", ruleResult.RuleName, ruleResult.Reason))
			}
		}
	}

	// Output unique rule messages
	for _, messages := range ruleMessages {
		if len(messages) > 0 {
			summary.WriteString(fmt.Sprintf("• %s\n", messages[0]))
		}
	}

	return summary.String()
}

// buildDetailedRulesSummary creates a detailed summary with metadata from file validations
func (mb *MessageBuilder) buildDetailedRulesSummary(fileValidations map[string]*shared.FileValidationSummary) string {
	var summary strings.Builder
	ruleDetails := make(map[string]*shared.LineValidationResult)

	// Collect detailed rule results from file validations
	for _, fileValidation := range fileValidations {
		for _, ruleResult := range fileValidation.RuleResults {
			// Use the first occurrence of each rule for detailed display
			if _, exists := ruleDetails[ruleResult.RuleName]; !exists {
				ruleDetails[ruleResult.RuleName] = &ruleResult
			}
		}
	}

	// Output detailed rule information
	for ruleName, result := range ruleDetails {
		switch result.Decision {
		case shared.Approve:
			summary.WriteString(fmt.Sprintf("• ✅ **%s**: %s\n", ruleName, result.Reason))
		case shared.ManualReview:
			summary.WriteString(fmt.Sprintf("• 🚫 **%s**: %s\n", ruleName, result.Reason))
		}

		// Add line range information
		if len(result.LineRanges) > 0 {
			summary.WriteString(fmt.Sprintf("  - Covered lines: %d-%d\n", 
				result.LineRanges[0].StartLine, result.LineRanges[0].EndLine))
		}
	}

	return summary.String()
}

// buildFilesSummary creates a summary of analyzed files
func (mb *MessageBuilder) buildFilesSummary(result *shared.RuleEvaluation) string {
	var summary strings.Builder

	for filePath, fileValidation := range result.FileValidations {
		summary.WriteString(fmt.Sprintf("• %s", filePath))
		
		// Add decision status
		switch fileValidation.FileDecision {
		case shared.Approve:
			summary.WriteString(" ✅")
		case shared.ManualReview:
			summary.WriteString(" 🚫")
		}
		
		// Add line coverage info
		if len(fileValidation.UncoveredLines) > 0 {
			summary.WriteString(fmt.Sprintf(" (%d uncovered lines)", len(fileValidation.UncoveredLines)))
		}
		
		summary.WriteString("\n")
	}

	return summary.String()
}

// buildDetailedFilesSummary creates a detailed summary with more metadata
func (mb *MessageBuilder) buildDetailedFilesSummary(result *shared.RuleEvaluation) string {
	var summary strings.Builder

	for filePath, fileValidation := range result.FileValidations {
		summary.WriteString(fmt.Sprintf("**File: %s**\n", filePath))
		summary.WriteString(fmt.Sprintf("• Total lines: %d\n", fileValidation.TotalLines))
		summary.WriteString(fmt.Sprintf("• Covered lines: %d\n", len(fileValidation.CoveredLines)))
		
		if len(fileValidation.UncoveredLines) > 0 {
			summary.WriteString(fmt.Sprintf("• Uncovered lines: %d\n", len(fileValidation.UncoveredLines)))
		}
		
		summary.WriteString(fmt.Sprintf("• Decision: %s\n", fileValidation.FileDecision))
		
		// List rules that validated this file
		if len(fileValidation.RuleResults) > 0 {
			summary.WriteString("• Rules applied:\n")
			for _, ruleResult := range fileValidation.RuleResults {
				summary.WriteString(fmt.Sprintf("  - %s: %s\n", ruleResult.RuleName, ruleResult.Reason))
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
	case mb.isDraftMR(result):
		return "Auto-approved: Draft MR with dataverse-safe changes"
	case mb.isAutomatedUser(result):
		return "Auto-approved: Automated user with passing CI"
	case mb.hasOnlyDataverseFiles(result):
		return "Auto-approved: Only dataverse-safe files modified"
	default:
		return "Auto-approved: All rules passed after CI success"
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

// isDraftMR checks if this was a draft MR approval
func (mb *MessageBuilder) isDraftMR(result *shared.RuleEvaluation) bool {
	return strings.Contains(result.FinalDecision.Reason, "Draft MR")
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

	summary.WriteString("📊 **Analysis Results:**\n")
	summary.WriteString(fmt.Sprintf("• %d files evaluated\n", result.TotalFiles))
	summary.WriteString(fmt.Sprintf("• Decision: %s\n", result.FinalDecision.Reason))

	// Show files requiring manual review
	if result.ReviewFiles > 0 {
		summary.WriteString(fmt.Sprintf("• %d file(s) require manual review\n", result.ReviewFiles))
	}

	return summary.String()
}

// buildDetailedManualReviewSummary creates a detailed manual review summary
func (mb *MessageBuilder) buildDetailedManualReviewSummary(result *shared.RuleEvaluation) string {
	var summary strings.Builder

	// Main decision message
	summary.WriteString(fmt.Sprintf("**Decision:** %s\n\n", result.FinalDecision.Reason))

	// Single expandable section with all details
	summary.WriteString("<details>\n")
	summary.WriteString("<summary>📋 <strong>Analysis Details</strong> (click to expand)</summary>\n\n")

	// Analysis results
	summary.WriteString("**📊 Analysis Results:**\n")
	summary.WriteString(mb.buildRulesSummary(result.FileValidations))
	summary.WriteString("\n")

	// File changes summary
	if filesSummary := mb.buildFilesSummary(result); filesSummary != "" {
		summary.WriteString("**📄 Files Analyzed:**\n")
		summary.WriteString(filesSummary)
		summary.WriteString("\n")
	}

	// Next steps
	summary.WriteString("**👀 Next Steps:**\n")
	summary.WriteString("• A reviewer will evaluate this MR manually\n")
	summary.WriteString("• You can find more details about the requirements in the project documentation\n")
	summary.WriteString("• Feel free to ask questions in the MR comments\n")
	summary.WriteString("\n")

	// Processing details
	summary.WriteString("**⏱️ Processing Details:**\n")
	summary.WriteString(fmt.Sprintf("• **Rule evaluation time:** %v\n", result.ExecutionTime))
	summary.WriteString(fmt.Sprintf("• **Total files analyzed:** %d\n", result.TotalFiles))
	summary.WriteString(fmt.Sprintf("• **Files approved:** %d\n", result.ApprovedFiles))
	summary.WriteString(fmt.Sprintf("• **Files requiring review:** %d\n", result.ReviewFiles))
	if result.UncoveredFiles > 0 {
		summary.WriteString(fmt.Sprintf("• **Uncovered files:** %d\n", result.UncoveredFiles))
	}
	summary.WriteString(fmt.Sprintf("• **Analyzed at:** %s\n", time.Now().Format("2006-01-02 15:04:05 UTC")))

	summary.WriteString("\n</details>")

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

	// Detailed analysis results
	summary.WriteString("📊 **Detailed Analysis Results:**\n")
	summary.WriteString(mb.buildDetailedRulesSummary(result.FileValidations))
	summary.WriteString("\n")

	// File changes with metadata
	if filesSummary := mb.buildDetailedFilesSummary(result); filesSummary != "" {
		summary.WriteString("📄 **Detailed File Analysis:**\n")
		summary.WriteString(filesSummary)
		summary.WriteString("\n")
	}

	// Next steps
	summary.WriteString("👀 **Next Steps:**\n")
	summary.WriteString("• A reviewer will evaluate this MR manually\n")
	summary.WriteString("• Check the detailed rule results above for specific requirements\n")
	summary.WriteString("• Review the project documentation for guidelines\n")
	summary.WriteString("\n")

	// System information
	summary.WriteString("⚙️ **System Details:**\n")
	summary.WriteString(fmt.Sprintf("• Rule evaluation time: %v\n", result.ExecutionTime))
	summary.WriteString(fmt.Sprintf("• Total files analyzed: %d\n", result.TotalFiles))
	summary.WriteString(fmt.Sprintf("• Final decision: %s\n", result.FinalDecision.Type))
	summary.WriteString(fmt.Sprintf("• Analyzed at: %s\n", time.Now().Format("2006-01-02 15:04:05 UTC")))

	return summary.String()
}
