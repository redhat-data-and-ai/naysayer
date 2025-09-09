package rules

import (
	"fmt"
	"strings"
	"time"

	"github.com/redhat-data-and-ai/naysayer/internal/config"
	"github.com/redhat-data-and-ai/naysayer/internal/gitlab"
	"github.com/redhat-data-and-ai/naysayer/internal/logging"
	"github.com/redhat-data-and-ai/naysayer/internal/rules/shared"
)

// SectionRuleManager manages section-based validation
type SectionRuleManager struct {
	rules          []shared.Rule
	sectionParsers map[string]shared.SectionParser // File pattern -> parser
	config         *config.RuleConfig
	ruleRegistry   map[string]shared.Rule // Rule name -> rule instance
}

// NewSectionRuleManager creates a new section-based rule manager
func NewSectionRuleManager(ruleConfig *config.RuleConfig) *SectionRuleManager {
	manager := &SectionRuleManager{
		rules:          make([]shared.Rule, 0),
		sectionParsers: make(map[string]shared.SectionParser),
		config:         ruleConfig,
		ruleRegistry:   make(map[string]shared.Rule),
	}

	// Initialize parsers based on configuration
	manager.initializeParsers()

	return manager
}

// initializeParsers sets up section parsers based on configuration
func (srm *SectionRuleManager) initializeParsers() {
	for _, fileConfig := range srm.config.Files {
		if !fileConfig.Enabled {
			logging.Info("Skipping disabled file configuration: %s", fileConfig.Name)
			continue
		}

		// Combine path and filename to create full pattern
		fullPattern := fileConfig.Path + fileConfig.Filename

		switch fileConfig.ParserType {
		case "yaml":
			// Create section definitions map from the file's sections
			definitionMap := make(map[string]config.SectionDefinition)
			for _, section := range fileConfig.Sections {
				definitionMap[section.Name] = section
			}
			srm.sectionParsers[fullPattern] = NewYAMLSectionParser(definitionMap)
			logging.Info("Initialized YAML parser for pattern: %s (%d sections)", fullPattern, len(definitionMap))
		case "json":
			// TODO: Implement JSON parser when needed
			logging.Warn("JSON section parser not yet implemented for: %s", fileConfig.Name)
		case "markdown":
			// TODO: Implement Markdown parser when needed
			logging.Warn("Markdown section parser not yet implemented for: %s", fileConfig.Name)
		default:
			logging.Warn("Unknown parser type %s for file configuration: %s", fileConfig.ParserType, fileConfig.Name)
		}
	}
}

// AddRule registers a rule with the manager
func (srm *SectionRuleManager) AddRule(rule shared.Rule) {
	srm.rules = append(srm.rules, rule)
	srm.ruleRegistry[rule.Name()] = rule
}

// EvaluateAll runs section-based validation on all files
func (srm *SectionRuleManager) EvaluateAll(mrCtx *shared.MRContext) *shared.RuleEvaluation {
	start := time.Now()

	// Note: Draft MR filtering is now handled at the webhook level to avoid any processing

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
	srm.setMRContextForRules(mrCtx)

	// Perform section-based validation
	fileValidations, overallDecision := srm.validateFilesWithSections(mrCtx)

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
		}

		if len(fileValidation.UncoveredLines) > 0 {
			uncoveredFiles++
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

// validateFilesWithSections performs section-based validation for each file
func (srm *SectionRuleManager) validateFilesWithSections(mrCtx *shared.MRContext) (map[string]*shared.FileValidationSummary, shared.Decision) {
	fileValidations := make(map[string]*shared.FileValidationSummary)

	// Get unique file paths from changes
	filePaths := srm.getUniqueFilePaths(mrCtx.Changes)

	for _, filePath := range filePaths {
		// Get file content (in production, this would fetch from GitLab API)
		fileContent := srm.getFileContent(filePath, mrCtx)
		totalLines := shared.CountLines(fileContent)

		// Check if this file has section-based validation
		parser := srm.getParserForFile(filePath)
		if parser != nil {
			// Use section-based validation
			fileValidation := srm.validateFileWithSections(filePath, fileContent, totalLines, parser)
			fileValidations[filePath] = fileValidation
		} else {
			// No section configuration found - require manual review
			fileValidation := srm.createManualReviewValidation(filePath, totalLines, "No section-based validation configuration found for this file type")
			fileValidations[filePath] = fileValidation
		}
	}

	// Determine overall decision
	overallDecision := srm.determineOverallDecision(fileValidations)
	return fileValidations, overallDecision
}

// validateFileWithSections validates a file using section-based approach
func (srm *SectionRuleManager) validateFileWithSections(filePath, fileContent string, totalLines int, parser shared.SectionParser) *shared.FileValidationSummary {
	// Parse file into sections
	sections, err := parser.ParseSections(filePath, fileContent)
	if err != nil {
		logging.Error("Failed to parse sections for %s: %v", filePath, err)
		// Section parsing failed - require manual review
		return srm.createManualReviewValidation(filePath, totalLines, fmt.Sprintf("Failed to parse file sections: %v", err))
	}

	var allCoveredLines []shared.LineRange
	var ruleResults []shared.LineValidationResult
	var sectionResults []shared.SectionValidationResult

	// Validate each section
	for _, section := range sections {
		// Get rules for this section
		sectionRules := srm.getRulesForSection(section.RuleNames)

		// Validate the section
		sectionResult := parser.ValidateSection(&section, sectionRules)
		sectionResults = append(sectionResults, *sectionResult)

		// Add to overall results
		for _, ruleResult := range sectionResult.RuleResults {
			ruleResults = append(ruleResults, ruleResult)
			allCoveredLines = append(allCoveredLines, ruleResult.LineRanges...)
		}
	}

	// Check for uncovered lines (lines not in any section)
	uncoveredLines := srm.getUncoveredLinesFromSections(totalLines, sections)

	// If there are uncovered lines and config requires manual review
	fileDecision := srm.determineFileDecisionWithSections(ruleResults, uncoveredLines, sectionResults)

	return &shared.FileValidationSummary{
		FilePath:       filePath,
		TotalLines:     totalLines,
		CoveredLines:   shared.MergeLineRanges(allCoveredLines),
		UncoveredLines: uncoveredLines,
		RuleResults:    ruleResults,
		FileDecision:   fileDecision,
	}
}

// createManualReviewValidation creates a validation summary that requires manual review
func (srm *SectionRuleManager) createManualReviewValidation(filePath string, totalLines int, reason string) *shared.FileValidationSummary {
	// Create uncovered lines for the entire file
	uncoveredLines := []shared.LineRange{{
		StartLine: 1,
		EndLine:   totalLines,
		FilePath:  filePath,
	}}

	return &shared.FileValidationSummary{
		FilePath:       filePath,
		TotalLines:     totalLines,
		CoveredLines:   []shared.LineRange{}, // No lines covered
		UncoveredLines: uncoveredLines,       // Entire file uncovered
		RuleResults:    []shared.LineValidationResult{}, // No rule results
		FileDecision:   shared.ManualReview, // Require manual review
	}
}

// getParserForFile returns the appropriate section parser for a file
func (srm *SectionRuleManager) getParserForFile(filePath string) shared.SectionParser {
	for pattern, parser := range srm.sectionParsers {
		if shared.MatchesPattern(filePath, pattern) {
			return parser
		}
	}
	return nil
}

// getRulesForSection returns rules that apply to a specific section
func (srm *SectionRuleManager) getRulesForSection(ruleNames []string) []shared.Rule {
	var sectionRules []shared.Rule

	for _, ruleName := range ruleNames {
		if rule, exists := srm.ruleRegistry[ruleName]; exists {
			sectionRules = append(sectionRules, rule)
		} else {
			logging.Warn("Rule %s not found in registry", ruleName)
		}
	}

	return sectionRules
}

// getUncoveredLinesFromSections calculates lines not covered by any section
func (srm *SectionRuleManager) getUncoveredLinesFromSections(totalLines int, sections []shared.Section) []shared.LineRange {
	var sectionRanges []shared.LineRange

	for _, section := range sections {
		sectionRanges = append(sectionRanges, shared.LineRange{
			StartLine: section.StartLine,
			EndLine:   section.EndLine,
			FilePath:  section.FilePath,
		})
	}

	return shared.GetUncoveredLines(totalLines, sectionRanges)
}

// determineFileDecisionWithSections determines file decision considering sections
func (srm *SectionRuleManager) determineFileDecisionWithSections(ruleResults []shared.LineValidationResult, uncoveredLines []shared.LineRange, sectionResults []shared.SectionValidationResult) shared.DecisionType {
	// First, check if any rule explicitly failed/rejected
	for _, result := range ruleResults {
		if result.Decision == shared.ManualReview {
			return shared.ManualReview
		}
	}

	// Then, check if any section explicitly failed/rejected
	for _, sectionResult := range sectionResults {
		if sectionResult.Decision == shared.ManualReview {
			return shared.ManualReview
		}
	}

	// Finally, check if there are uncovered lines (only if config requires it)
	if len(uncoveredLines) > 0 && srm.config.ManualReviewOnUncovered {
		return shared.ManualReview
	}

	// If we reach here, all rules approved their sections and all lines are covered
	return shared.Approve
}

// Helper methods (similar to existing manager)

func (srm *SectionRuleManager) setMRContextForRules(mrCtx *shared.MRContext) {
	for _, rule := range srm.rules {
		if contextRule, ok := rule.(shared.ContextAwareRule); ok {
			contextRule.SetMRContext(mrCtx)
		}
	}
}

func (srm *SectionRuleManager) getUniqueFilePaths(changes []gitlab.FileChange) []string {
	// Extract unique file paths from GitLab changes
	pathMap := make(map[string]bool)
	var filePaths []string

	for _, change := range changes {
		if change.NewPath != "" && !pathMap[change.NewPath] {
			pathMap[change.NewPath] = true
			filePaths = append(filePaths, change.NewPath)
		}
		if change.OldPath != "" && change.OldPath != change.NewPath && !pathMap[change.OldPath] {
			pathMap[change.OldPath] = true
			filePaths = append(filePaths, change.OldPath)
		}
	}

	return filePaths
}

func (srm *SectionRuleManager) getFileContent(filePath string, mrCtx *shared.MRContext) string {
	// For now, extract content from the diff in the changes
	// In a full implementation, this would fetch the current file content from GitLab API
	for _, change := range mrCtx.Changes {
		if change.NewPath == filePath {
			// Try to reconstruct file content from diff
			if change.Diff != "" {
				return srm.extractFileContentFromDiff(change.Diff)
			}
		}
	}
	return ""
}

// extractFileContentFromDiff attempts to extract file content from a Git diff
func (srm *SectionRuleManager) extractFileContentFromDiff(diff string) string {
	// This is a simplified implementation
	// In production, we should fetch the actual file content from GitLab API
	// For now, we'll try to extract content from the diff

	lines := strings.Split(diff, "\n")
	var contentLines []string
	inContent := false

	for _, line := range lines {
		if strings.HasPrefix(line, "@@") {
			inContent = true
			continue
		}
		if !inContent {
			continue
		}

		// Add lines that are not diff markers
		if len(line) > 0 {
			switch line[0] {
			case '+':
				// Added line - include in content
				contentLines = append(contentLines, line[1:])
			case ' ':
				// Unchanged line - include in content
				contentLines = append(contentLines, line[1:])
			case '-':
				// Removed line - skip for new content
				continue
			default:
				// Regular line
				contentLines = append(contentLines, line)
			}
		}
	}

	return strings.Join(contentLines, "\n")
}


func (srm *SectionRuleManager) determineOverallDecision(fileValidations map[string]*shared.FileValidationSummary) shared.Decision {
	var manualReviewFiles []string
	var approvedFiles []string

	// Collect file results
	for _, fileValidation := range fileValidations {
		if fileValidation.FileDecision == shared.ManualReview {
			manualReviewFiles = append(manualReviewFiles, fileValidation.FilePath)
		} else {
			approvedFiles = append(approvedFiles, fileValidation.FilePath)
		}
	}

	// If any file requires manual review, the entire MR requires manual review
	if len(manualReviewFiles) > 0 {
		details := fmt.Sprintf("Files requiring manual review: %s", strings.Join(manualReviewFiles, ", "))
		if len(approvedFiles) > 0 {
			details += fmt.Sprintf(". Files auto-approved: %s", strings.Join(approvedFiles, ", "))
		}

		return shared.Decision{
			Type:    shared.ManualReview,
			Reason:  "One or more files require manual review",
			Summary: "⚠️ Manual review required",
			Details: details,
		}
	}

	// All files approved - provide detailed summary
	return shared.Decision{
		Type:    shared.Approve,
		Reason:  "All files passed validation - all changes covered by approved rules",
		Summary: "✅ Auto-approved",
		Details: fmt.Sprintf("All %d files passed section-based validation with complete coverage", len(fileValidations)),
	}
}
