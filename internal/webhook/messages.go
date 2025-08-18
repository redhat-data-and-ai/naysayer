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
	comment.WriteString("🔍 **Manual review required**\n\n")

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
	summary.WriteString(fmt.Sprintf("• %d rules evaluated, all passed\n", len(result.RuleResults)))
	summary.WriteString(fmt.Sprintf("• Decision: %s\n", result.FinalDecision.Reason))

	return summary.String()
}

// buildDetailedSummary creates a detailed approval summary
func (mb *MessageBuilder) buildDetailedSummary(result *shared.RuleEvaluation) string {
	var summary strings.Builder

	// Analysis results
	summary.WriteString("📊 **Analysis Results:**\n")
	summary.WriteString(mb.buildRulesSummary(result.RuleResults))
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
	summary.WriteString(mb.buildDetailedRulesSummary(result.RuleResults))
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
	summary.WriteString(fmt.Sprintf("• Total rules evaluated: %d\n", len(result.RuleResults)))
	summary.WriteString(fmt.Sprintf("• Final decision: %s\n", result.FinalDecision.Type))
	summary.WriteString(fmt.Sprintf("• Approved at: %s\n", time.Now().Format("2006-01-02 15:04:05 UTC")))

	return summary.String()
}

// buildRulesSummary creates a summary of rule evaluation results
func (mb *MessageBuilder) buildRulesSummary(ruleResults []shared.RuleResult) string {
	var summary strings.Builder

	for _, result := range ruleResults {
		switch result.Decision.Type {
		case shared.Approve:
			summary.WriteString(fmt.Sprintf("• ✅ %s: %s\n", result.RuleName, result.Decision.Reason))
		case shared.ManualReview:
			summary.WriteString(fmt.Sprintf("• 🚫 %s: %s\n", result.RuleName, result.Decision.Reason))
		}
	}

	return summary.String()
}

// buildDetailedRulesSummary creates a detailed summary with metadata
func (mb *MessageBuilder) buildDetailedRulesSummary(ruleResults []shared.RuleResult) string {
	var summary strings.Builder

	for _, result := range ruleResults {
		switch result.Decision.Type {
		case shared.Approve:
			summary.WriteString(fmt.Sprintf("• ✅ **%s**: %s\n", result.RuleName, result.Decision.Reason))
		case shared.ManualReview:
			summary.WriteString(fmt.Sprintf("• 🚫 **%s**: %s\n", result.RuleName, result.Decision.Reason))
		}

		// Add metadata if available
		if len(result.Metadata) > 0 {
			summary.WriteString("  - Metadata: ")
			for key, value := range result.Metadata {
				summary.WriteString(fmt.Sprintf("%s=%v ", key, value))
			}
			summary.WriteString("\n")
		}

		// Add execution time
		if result.ExecutionTime > 0 {
			summary.WriteString(fmt.Sprintf("  - Execution time: %v\n", result.ExecutionTime))
		}
	}

	return summary.String()
}

// buildFilesSummary creates a summary of analyzed files
func (mb *MessageBuilder) buildFilesSummary(result *shared.RuleEvaluation) string {
	var summary strings.Builder
	filesAnalyzed := make(map[string]bool)

	for _, ruleResult := range result.RuleResults {
		// Extract file information from rule results metadata
		if files, ok := ruleResult.Metadata["analyzed_files"].([]string); ok {
			for _, file := range files {
				if !filesAnalyzed[file] {
					summary.WriteString(fmt.Sprintf("• %s\n", file))
					filesAnalyzed[file] = true
				}
			}
		}

		// Add warehouse changes information
		if changes, ok := ruleResult.Metadata["warehouse_changes"].([]interface{}); ok && len(changes) > 0 {
			summary.WriteString(fmt.Sprintf("• %d warehouse changes detected\n", len(changes)))
		}

		// Add dataverse file type information
		if fileTypes, ok := ruleResult.Metadata["dataverse_file_types"].(map[string]int); ok {
			for fileType, count := range fileTypes {
				if count > 0 {
					summary.WriteString(fmt.Sprintf("• %d %s files\n", count, fileType))
				}
			}
		}
	}

	return summary.String()
}

// buildDetailedFilesSummary creates a detailed summary with more metadata
func (mb *MessageBuilder) buildDetailedFilesSummary(result *shared.RuleEvaluation) string {
	var summary strings.Builder

	for _, ruleResult := range result.RuleResults {
		if ruleResult.Metadata == nil {
			continue
		}

		summary.WriteString(fmt.Sprintf("**%s Rule Analysis:**\n", ruleResult.RuleName))

		// Files analyzed
		if files, ok := ruleResult.Metadata["analyzed_files"].([]string); ok {
			for _, file := range files {
				summary.WriteString(fmt.Sprintf("• File: %s\n", file))
			}
		}

		// Warehouse changes details
		if changes, ok := ruleResult.Metadata["warehouse_changes"]; ok {
			summary.WriteString(fmt.Sprintf("• Warehouse changes: %v\n", changes))
		}

		// Additional metadata
		for key, value := range ruleResult.Metadata {
			if key != "analyzed_files" && key != "warehouse_changes" {
				summary.WriteString(fmt.Sprintf("• %s: %v\n", key, value))
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
	for _, ruleResult := range result.RuleResults {
		if ruleResult.RuleName == "warehouse_rule" && ruleResult.Decision.Type == shared.Approve {
			if changes, ok := ruleResult.Metadata["warehouse_changes"]; ok && changes != nil {
				return true
			}
		}
	}
	return false
}

// isDraftMR checks if this was a draft MR approval
func (mb *MessageBuilder) isDraftMR(result *shared.RuleEvaluation) bool {
	for _, ruleResult := range result.RuleResults {
		if draft, ok := ruleResult.Metadata["is_draft"].(bool); ok && draft {
			return true
		}
	}
	return false
}

// isAutomatedUser checks if this was an automated user approval
func (mb *MessageBuilder) isAutomatedUser(result *shared.RuleEvaluation) bool {
	for _, ruleResult := range result.RuleResults {
		if automated, ok := ruleResult.Metadata["is_automated"].(bool); ok && automated {
			return true
		}
	}
	return false
}

// hasOnlyDataverseFiles checks if only dataverse files were modified
func (mb *MessageBuilder) hasOnlyDataverseFiles(result *shared.RuleEvaluation) bool {
	for _, ruleResult := range result.RuleResults {
		if ruleResult.RuleName == "warehouse_rule" && ruleResult.Decision.Type == shared.Approve {
			return true
		}
	}
	return false
}

// buildBasicManualReviewSummary creates a basic manual review summary
func (mb *MessageBuilder) buildBasicManualReviewSummary(result *shared.RuleEvaluation) string {
	var summary strings.Builder

	summary.WriteString("📊 **Analysis Results:**\n")
	summary.WriteString(fmt.Sprintf("• %d rules evaluated\n", len(result.RuleResults)))
	summary.WriteString(fmt.Sprintf("• Decision: %s\n", result.FinalDecision.Reason))

	// Count rules requiring manual review
	manualReviewCount := 0
	for _, ruleResult := range result.RuleResults {
		if ruleResult.Decision.Type == shared.ManualReview {
			manualReviewCount++
		}
	}

	if manualReviewCount > 0 {
		summary.WriteString(fmt.Sprintf("• %d rule(s) require manual review\n", manualReviewCount))
	}

	return summary.String()
}

// buildDetailedManualReviewSummary creates a detailed manual review summary
func (mb *MessageBuilder) buildDetailedManualReviewSummary(result *shared.RuleEvaluation) string {
	var summary strings.Builder

	// Analysis results
	summary.WriteString("📊 **Analysis Results:**\n")
	summary.WriteString(mb.buildRulesSummary(result.RuleResults))
	summary.WriteString("\n")

	// File changes summary
	if filesSummary := mb.buildFilesSummary(result); filesSummary != "" {
		summary.WriteString("📄 **Files Analyzed:**\n")
		summary.WriteString(filesSummary)
		summary.WriteString("\n")
	}

	// Next steps
	summary.WriteString("👀 **Next Steps:**\n")
	summary.WriteString("• A reviewer will evaluate this MR manually\n")
	summary.WriteString("• You can find more details about the requirements in the project documentation\n")
	summary.WriteString("• Feel free to ask questions in the MR comments\n")
	summary.WriteString("\n")

	// Timing information
	summary.WriteString("⏱️ **Processing Details:**\n")
	summary.WriteString(fmt.Sprintf("• Rule evaluation time: %v\n", result.ExecutionTime))
	summary.WriteString(fmt.Sprintf("• Analyzed at: %s\n", time.Now().Format("2006-01-02 15:04:05 UTC")))

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
	summary.WriteString(mb.buildDetailedRulesSummary(result.RuleResults))
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
	summary.WriteString(fmt.Sprintf("• Total rules evaluated: %d\n", len(result.RuleResults)))
	summary.WriteString(fmt.Sprintf("• Final decision: %s\n", result.FinalDecision.Type))
	summary.WriteString(fmt.Sprintf("• Analyzed at: %s\n", time.Now().Format("2006-01-02 15:04:05 UTC")))

	return summary.String()
}
