package rules

import (
	"fmt"
	"sort"
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
	config         *config.GlobalRuleConfig
	ruleRegistry   map[string]shared.Rule // Rule name -> rule instance
	gitlabClient   gitlab.GitLabClient    // GitLab client for fetching file content
	ignorePatterns []string               // Compiled patterns from ignore_files config
}

// NewSectionRuleManager creates a new section-based rule manager
func NewSectionRuleManager(ruleConfig *config.GlobalRuleConfig, client gitlab.GitLabClient) *SectionRuleManager {
	manager := &SectionRuleManager{
		rules:          make([]shared.Rule, 0),
		sectionParsers: make(map[string]shared.SectionParser),
		config:         ruleConfig,
		ruleRegistry:   make(map[string]shared.Rule),
		gitlabClient:   client,
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

	// Initialize ignore patterns from ignore_files configuration
	for _, ignoreConfig := range srm.config.IgnoreFiles {
		pattern := ignoreConfig.Path + ignoreConfig.Filename
		srm.ignorePatterns = append(srm.ignorePatterns, pattern)
		logging.Info("Registered ignore pattern: %s (%s)", pattern, ignoreConfig.Name)
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
	fileValidations, overallDecision, ignoredFiles := srm.validateFilesWithSections(mrCtx)

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
		IgnoredFiles:    ignoredFiles,
	}
}

// validateFilesWithSections performs section-based validation for each file
func (srm *SectionRuleManager) validateFilesWithSections(mrCtx *shared.MRContext) (map[string]*shared.FileValidationSummary, shared.Decision, []string) {
	fileValidations := make(map[string]*shared.FileValidationSummary)
	var ignoredFiles []string

	// Get unique file paths from changes
	filePaths := srm.getUniqueFilePaths(mrCtx.Changes)

	// Source branch files for fork MRs live on the fork project, not the target (same as warehouse analyzer).
	sourceProjectID := srm.sourceProjectIDForMR(mrCtx)

	for _, filePath := range filePaths {
		// Check ignore patterns first (takes precedence over all other classification)
		if srm.isIgnoredFile(filePath) {
			logging.Info("File ignored by configuration: %s", filePath)
			ignoredFiles = append(ignoredFiles, filePath)
			continue
		}

		// Check if file was deleted in this MR
		if srm.isDeletedFile(filePath, mrCtx.Changes) {
			reason := srm.getDeletionReason(filePath)
			logging.Info("File deleted in MR: %s - requiring manual review", filePath)
			fileValidations[filePath] = srm.createDeletionValidation(filePath, reason)
			continue
		}

		// Get changed lines for the file
		addedLines, deletedLines := srm.getChangedLinesForFile(filePath, mrCtx)

		if len(addedLines) == 0 && len(deletedLines) == 0 {
			logging.Warn("No changed lines for %s - requiring manual review", filePath)
			fileValidations[filePath] = srm.createManualReviewValidation(
				filePath, 0, "No added or deleted lines found in the diff; cannot validate this file")
			continue
		}

		if mrCtx.MRInfo == nil {
			logging.Warn("MR info not available for %s - requiring manual review", filePath)
			fileValidations[filePath] = srm.createManualReviewValidation(
				filePath, 0, "MR branch info not available; cannot fetch file content")
			continue
		}

		// Get file content from source branch
		var sourceFileContent, targetFileContent string
		if len(addedLines) > 0 {
			fileContent, fetchErr := srm.fetchFileFromBranch(sourceProjectID, filePath, mrCtx.MRInfo.SourceBranch)
			if fetchErr != nil {
				logging.Warn("Cannot load source-branch file for validation (requiring manual review): %s: %v", filePath, fetchErr)
				fileValidations[filePath] = srm.createManualReviewValidation(filePath, 0, fmt.Sprintf("Could not load file from source branch: %v", fetchErr))
				continue
			}
			sourceFileContent = fileContent
		}
		if len(deletedLines) > 0 {
			fileContent, fetchErr := srm.fetchFileFromBranch(mrCtx.ProjectID, filePath, mrCtx.MRInfo.TargetBranch)
			if fetchErr != nil {
				logging.Warn("Cannot load target-branch file for validation (requiring manual review): %s: %v", filePath, fetchErr)
				fileValidations[filePath] = srm.createManualReviewValidation(filePath, 0, fmt.Sprintf("Could not load file from target branch: %v", fetchErr))
				continue
			}
			targetFileContent = fileContent
		}

		// Extract changed lines from the diff for delta validation
		diffText := srm.getDiffForFile(filePath, mrCtx)

		// Check if this file has section-based validation
		parser := srm.getParserForFile(filePath)
		if parser != nil {
			logging.Info("Using section-based validation for file: %s", filePath)
			// Use section-based validation with delta approach
			fileValidation := srm.validateFileWithSections(filePath, sourceFileContent, targetFileContent, parser, addedLines, deletedLines, diffText)
			fileValidations[filePath] = fileValidation
		} else {
			logging.Info("No parser found for file: %s - requiring manual review", filePath)
			// No section configuration found - require manual review
			fileValidation := srm.createManualReviewValidation(filePath, shared.CountLines(sourceFileContent), "No section-based validation configuration found for this file type")
			fileValidations[filePath] = fileValidation
		}
	}

	// Determine overall decision
	overallDecision := srm.determineOverallDecision(fileValidations, ignoredFiles)
	return fileValidations, overallDecision, ignoredFiles
}

// getChangedLinesForFile extracts both added and deleted line ranges for a specific file.
// Returns addedLines (positions in new/source branch file) and deletedLines (positions in old/target branch file).
func (srm *SectionRuleManager) getChangedLinesForFile(filePath string, mrCtx *shared.MRContext) (addedLines []shared.LineRange, deletedLines []shared.LineRange) {
	for _, change := range mrCtx.Changes {
		if change.NewPath == filePath && change.Diff != "" {
			added, deleted := srm.extractChangedLinesFromDiff(change.Diff)
			for i := range added {
				added[i].FilePath = filePath
			}
			for i := range deleted {
				deleted[i].FilePath = filePath
			}
			return added, deleted
		}
	}
	return []shared.LineRange{}, []shared.LineRange{}
}

// fetchFileFromBranch fetches file content from a specific project and branch.
func (srm *SectionRuleManager) fetchFileFromBranch(projectID int, filePath, branch string) (string, error) {
	if srm.gitlabClient == nil {
		return "", fmt.Errorf("GitLab client not available")
	}
	if branch == "" {
		return "", fmt.Errorf("branch not specified for file fetch: %s", filePath)
	}
	fileContent, err := srm.gitlabClient.FetchFileContent(projectID, filePath, branch)
	if err != nil {
		return "", fmt.Errorf("failed to fetch %s from project %d branch %s: %w", filePath, projectID, branch, err)
	}
	if fileContent == nil {
		return "", fmt.Errorf("empty response when fetching %s", filePath)
	}
	return fileContent.Content, nil
}

func (srm *SectionRuleManager) getDiffForFile(filePath string, mrCtx *shared.MRContext) string {
	for _, change := range mrCtx.Changes {
		if change.NewPath == filePath {
			return change.Diff
		}
	}
	return ""
}

// validateFileWithSections validates a file using section-based approach with delta validation.
// addedLines are positions in the new file (source branch), deletedLines are positions in the old file (target branch).
func (srm *SectionRuleManager) validateFileWithSections(filePath, sourceFileContent, targetFileContent string,
	parser shared.SectionParser, addedLines, deletedLines []shared.LineRange, diffText string) *shared.FileValidationSummary {

	// Track which sections were affected by additions (new file sections)
	affectedSections := make(map[string]bool)
	var addedSections []shared.Section
	var deletedSections []shared.Section
	var deletedAffected []shared.Section

	if len(addedLines) > 0 && sourceFileContent != "" {
		var err error
		addedSections, err = parser.ParseSections(filePath, sourceFileContent)
		if err != nil {
			logging.Error("Failed to parse sections for %s: %v", filePath, err)
			return srm.createManualReviewValidation(filePath, shared.CountLines(sourceFileContent), fmt.Sprintf("Failed to parse file sections: %v", err))
		}
		affected := srm.getAffectedSections(addedSections, addedLines)
		for _, section := range affected {
			affectedSections[section.Name] = true
		}
	}

	// Track which sections were affected by deletions (old file sections).
	// Validate those even when the same section name still exists in the new file
	// (e.g. access_policy removed while a consumer is added under data_product_db).
	if len(deletedLines) > 0 && targetFileContent != "" {
		var err error
		deletedSections, err = parser.ParseSections(filePath, targetFileContent)
		if err != nil {
			logging.Error("Failed to parse old file sections for %s: %v", filePath, err)
			return srm.createManualReviewValidation(filePath, shared.CountLines(targetFileContent), fmt.Sprintf("Failed to parse old file sections: %v", err))
		}
		deletedAffected = srm.getAffectedSections(deletedSections, deletedLines)
		for _, section := range deletedAffected {
			affectedSections[section.Name] = true
		}
	}

	if len(affectedSections) > 0 {
		var names []string
		for name := range affectedSections {
			names = append(names, name)
		}
		sort.Strings(names)
		logging.Info("Delta validation for %s: %d affected sections: %s", filePath, len(affectedSections), strings.Join(names, ", "))
	}

	if !affectedSections["warehouses"] && diffMentionsWarehouses(diffText) {
		affectedSections["warehouses"] = true
		logging.Info("Delta validation for %s: warehouses section flagged as affected (diff heuristic)", filePath)
	}

	var allCoveredLines []shared.LineRange
	var ruleResults []shared.LineValidationResult
	var sectionResults []shared.SectionValidationResult

	sectionResults, ruleResults, allCoveredLines = srm.validateSections(parser, addedSections, addedLines, sectionResults, ruleResults, allCoveredLines)
	sectionResults, ruleResults, allCoveredLines = srm.validateSections(parser, deletedAffected, deletedLines, sectionResults, ruleResults, allCoveredLines)

	// Generic defense-in-depth:
	// if a changed section expects a rule that did not produce a result,
	// inject a manual-review fallback without overwriting existing reasons.
	allSections := append(addedSections, deletedAffected...)
	expectedRules := srm.getExpectedRulesForAffectedSections(allSections, affectedSections)
	ruleResults = srm.appendMissingExpectedRuleFallbacks(ruleResults, expectedRules, addedLines)

	// Check for uncovered lines (lines not in any section).
	// Added and deleted ranges use their own file coordinates so they are not mixed.
	uncoveredLines := srm.getUncoveredLinesInChanges(shared.CountLines(sourceFileContent), addedSections, addedLines)
	if len(deletedLines) > 0 {
		uncoveredLines = append(uncoveredLines, srm.getUncoveredLinesInChanges(shared.CountLines(targetFileContent), deletedSections, deletedLines)...)
	}

	// Filter results: only affected sections influence the decision.
	// Unaffected sections are still validated (for MR comment display) but
	// their outcomes must not block auto-approval.
	var decisionRuleResults []shared.LineValidationResult
	var decisionSectionResults []shared.SectionValidationResult

	if len(affectedSections) == 0 {
		decisionRuleResults = ruleResults
		decisionSectionResults = sectionResults
	} else {
		for i, section := range allSections {
			if affectedSections[section.Name] {
				decisionSectionResults = append(decisionSectionResults, sectionResults[i])
				decisionRuleResults = append(decisionRuleResults, sectionResults[i].RuleResults...)
			}
		}
		// Include defense-in-depth fallback results (they target affected sections by design)
		for _, rr := range ruleResults {
			if !rr.WasEvaluated {
				decisionRuleResults = append(decisionRuleResults, rr)
			}
		}
	}

	fileDecision := srm.determineFileDecisionWithSections(decisionRuleResults, uncoveredLines, decisionSectionResults)

	return &shared.FileValidationSummary{
		FilePath:       filePath,
		TotalLines:     shared.CountLines(sourceFileContent),
		CoveredLines:   shared.MergeLineRanges(allCoveredLines),
		UncoveredLines: uncoveredLines,
		RuleResults:    ruleResults,
		FileDecision:   fileDecision,
	}
}

// validateSections runs configured rules for each section using the given line coordinates
// (addedLines in new-file space, or deletedLines in old-file space).
func (srm *SectionRuleManager) validateSections(parser shared.SectionParser, sections []shared.Section, changedLines []shared.LineRange, sectionResults []shared.SectionValidationResult, ruleResults []shared.LineValidationResult, allCoveredLines []shared.LineRange) ([]shared.SectionValidationResult, []shared.LineValidationResult, []shared.LineRange) {
	for i := range sections {
		section := &sections[i]
		section.ChangedLines = getSectionChangedLines(changedLines, section.StartLine, section.EndLine)

		sectionRules := srm.getEnabledRulesForSection(section.RuleConfigs)
		sectionResult := parser.ValidateSection(section, sectionRules)
		sectionResults = append(sectionResults, *sectionResult)

		for _, ruleResult := range sectionResult.RuleResults {
			ruleResults = append(ruleResults, ruleResult)
			allCoveredLines = append(allCoveredLines, ruleResult.LineRanges...)
		}
	}
	return sectionResults, ruleResults, allCoveredLines
}

// getSectionChangedLines converts file-level changedLines to section-relative
// coordinates, filtering to only lines within the section bounds.
func getSectionChangedLines(changedLines []shared.LineRange, sectionStart, sectionEnd int) []shared.LineRange {
	var result []shared.LineRange
	for _, cl := range changedLines {
		if cl.EndLine < sectionStart || cl.StartLine > sectionEnd {
			continue
		}
		start := cl.StartLine
		if start < sectionStart {
			start = sectionStart
		}
		end := cl.EndLine
		if end > sectionEnd {
			end = sectionEnd
		}
		result = append(result, shared.LineRange{
			StartLine: start - sectionStart + 1,
			EndLine:   end - sectionStart + 1,
			FilePath:  cl.FilePath,
		})
	}
	return result
}

func diffMentionsWarehouses(diffText string) bool {
	if diffText == "" {
		return false
	}

	if strings.Contains(diffText, "\nwarehouses:") || strings.HasPrefix(diffText, "warehouses:") {
		return true
	}
	if strings.Contains(diffText, "+warehouses:") || strings.Contains(diffText, "-warehouses:") {
		return true
	}
	return false
}

// Get all rules (enabled and disabled) defined for the affected sections
func (srm *SectionRuleManager) getExpectedRulesForAffectedSections(sections []shared.Section, affectedSections map[string]bool) []string {
	if len(affectedSections) == 0 {
		return nil
	}

	ruleSet := make(map[string]bool)
	for _, section := range sections {
		if !affectedSections[section.Name] {
			continue
		}
		for _, rc := range section.RuleConfigs {
			if rc.Enabled && rc.Name != "" {
				ruleSet[rc.Name] = true
			}
		}
	}

	var expected []string
	for name := range ruleSet {
		expected = append(expected, name)
	}
	sort.Strings(expected)
	return expected
}

// Rule that were either disabled or not evaluated should result in a manual review.
func (srm *SectionRuleManager) appendMissingExpectedRuleFallbacks(
	ruleResults []shared.LineValidationResult,
	expectedRules []string,
	changedLines []shared.LineRange,
) []shared.LineValidationResult {
	if len(expectedRules) == 0 {
		return ruleResults
	}

	seen := make(map[string]bool)
	for _, rr := range ruleResults {
		if rr.RuleName != "" {
			seen[rr.RuleName] = true
		}
	}

	for _, ruleName := range expectedRules {
		if seen[ruleName] {
			continue
		}

		ruleResults = append(ruleResults, shared.LineValidationResult{
			RuleName:     ruleName,
			LineRanges:   changedLines,
			Decision:     shared.ManualReview,
			Reason:       fmt.Sprintf("Manual review required: '%s' was not evaluated for changed section(s)", ruleName),
			WasEvaluated: false,
		})
	}

	return ruleResults
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
		CoveredLines:   []shared.LineRange{},            // No lines covered
		UncoveredLines: uncoveredLines,                  // Entire file uncovered
		RuleResults:    []shared.LineValidationResult{}, // No rule results
		FileDecision:   shared.ManualReview,             // Require manual review
	}
}

func (srm *SectionRuleManager) isDeletedFile(filePath string, changes []gitlab.FileChange) bool {
	for _, change := range changes {
		if (change.OldPath == filePath || change.NewPath == filePath) && change.DeletedFile {
			return true
		}
	}
	return false
}

func (srm *SectionRuleManager) getDeletionReason(filePath string) string {
	for _, fileConfig := range srm.config.Files {
		if !fileConfig.Enabled {
			continue
		}
		fullPattern := fileConfig.Path + fileConfig.Filename
		if shared.MatchesPattern(filePath, fullPattern) {
			return fmt.Sprintf("Deletion requires manual review: %s", fileConfig.Name)
		}
	}
	return fmt.Sprintf("Deletion requires manual review: %s", filePath)
}

func (srm *SectionRuleManager) createDeletionValidation(filePath string, reason string) *shared.FileValidationSummary {
	return &shared.FileValidationSummary{
		FilePath:       filePath,
		TotalLines:     0,
		CoveredLines:   []shared.LineRange{},
		UncoveredLines: []shared.LineRange{},
		RuleResults: []shared.LineValidationResult{
			{
				RuleName:     "deletion_check",
				Decision:     shared.ManualReview,
				Reason:       reason,
				WasEvaluated: true,
			},
		},
		FileDecision: shared.ManualReview,
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

// isIgnoredFile checks if a file matches any ignore_files pattern
func (srm *SectionRuleManager) isIgnoredFile(filePath string) bool {
	for _, pattern := range srm.ignorePatterns {
		if shared.MatchesPattern(filePath, pattern) {
			return true
		}
	}
	return false
}

// getEnabledRulesForSection returns enabled rules that apply to a specific section
func (srm *SectionRuleManager) getEnabledRulesForSection(ruleConfigs []config.RuleConfig) []shared.Rule {
	var sectionRules []shared.Rule

	for _, ruleConfig := range ruleConfigs {
		if !ruleConfig.Enabled {
			logging.Info("Skipping disabled rule: %s", ruleConfig.Name)
			continue
		}

		if rule, exists := srm.ruleRegistry[ruleConfig.Name]; exists {
			sectionRules = append(sectionRules, rule)
		} else {
			logging.Warn("Rule %s not found in registry", ruleConfig.Name)
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

// getUncoveredLinesInChanges calculates uncovered lines only within the changed line ranges
// This prevents unchanged lines from being marked as uncovered
func (srm *SectionRuleManager) getUncoveredLinesInChanges(totalLines int, sections []shared.Section, changedLines []shared.LineRange) []shared.LineRange {
	// If no changes detected, fall back to checking all lines
	if len(changedLines) == 0 {
		return srm.getUncoveredLinesFromSections(totalLines, sections)
	}

	var sectionRanges []shared.LineRange
	for _, section := range sections {
		sectionRanges = append(sectionRanges, shared.LineRange{
			StartLine: section.StartLine,
			EndLine:   section.EndLine,
			FilePath:  section.FilePath,
		})
	}

	// Get uncovered line ranges (lines not in any section)
	allUncovered := shared.GetUncoveredLines(totalLines, sectionRanges)

	// Filter to only include uncovered lines that are within changed ranges
	var uncoveredInChanges []shared.LineRange
	for _, uncovered := range allUncovered {
		for _, changed := range changedLines {
			// Find the intersection between uncovered and changed ranges
			intersection := srm.getLineRangeIntersection(uncovered, changed)
			if intersection != nil {
				uncoveredInChanges = append(uncoveredInChanges, *intersection)
			}
		}
	}

	return uncoveredInChanges
}

// getLineRangeIntersection returns the intersection of two line ranges, or nil if they don't overlap
func (srm *SectionRuleManager) getLineRangeIntersection(range1, range2 shared.LineRange) *shared.LineRange {
	// Find the overlapping part
	start := range1.StartLine
	if range2.StartLine > start {
		start = range2.StartLine
	}

	end := range1.EndLine
	if range2.EndLine < end {
		end = range2.EndLine
	}

	// If start > end, there's no overlap
	if start > end {
		return nil
	}

	return &shared.LineRange{
		StartLine: start,
		EndLine:   end,
		FilePath:  range1.FilePath,
	}
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

	// Finally, check if there are uncovered lines (strict coverage policy)
	if len(uncoveredLines) > 0 {
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

// sourceProjectIDForMR returns the GitLab project ID where the MR source branch exists.
// For same-repository MRs this is mrCtx.ProjectID; for fork MRs it is the fork's project ID.
func (srm *SectionRuleManager) sourceProjectIDForMR(mrCtx *shared.MRContext) int {
	projectID := mrCtx.ProjectID
	if srm.gitlabClient == nil {
		return projectID
	}
	mrDetails, err := srm.gitlabClient.GetMRDetails(projectID, mrCtx.MRIID)
	if err != nil {
		logging.Warn("Failed to get MR details for source project resolution (MR %d): %v", mrCtx.MRIID, err)
		return projectID
	}
	if mrDetails != nil && mrDetails.SourceProjectID != 0 && mrDetails.SourceProjectID != projectID {
		logging.Info("Fork MR: source-branch file fetches use project %d (target project %d)", mrDetails.SourceProjectID, projectID)
		return mrDetails.SourceProjectID
	}
	return projectID
}

// extractChangedLinesFromDiff extracts the line ranges that were modified in a Git diff.
// Only lines actually added/modified ('+' prefix) are included; context lines are skipped
// so that sections appearing only as diff context aren't flagged as affected.
func (srm *SectionRuleManager) extractChangedLinesFromDiff(diff string) (addedLines []shared.LineRange, deletedLines []shared.LineRange) {
	lines := strings.Split(diff, "\n")

	newLineNum := 0
	oldLineNum := 0
	addStart := 0
	delStart := 0

	flushAdded := func() {
		if addStart > 0 {
			addedLines = append(addedLines, shared.LineRange{StartLine: addStart, EndLine: newLineNum - 1})
			addStart = 0
		}
	}

	flushDeleted := func() {
		if delStart > 0 {
			deletedLines = append(deletedLines, shared.LineRange{StartLine: delStart, EndLine: oldLineNum - 1})
			delStart = 0
		}
	}

	for _, line := range lines {
		if strings.HasPrefix(line, "@@") {
			flushAdded()
			flushDeleted()
			newStart, oldStart := srm.parseHunkHeader(line)
			newLineNum = newStart
			oldLineNum = oldStart
			continue
		}

		if newLineNum == 0 && oldLineNum == 0 {
			continue
		}

		// "\ No newline at end of file" is a marker, not file content.
		if strings.HasPrefix(line, "\\") {
			continue
		}

		if strings.HasPrefix(line, "+") {
			flushDeleted()
			if addStart == 0 {
				addStart = newLineNum
			}
			newLineNum++
		} else if strings.HasPrefix(line, "-") {
			flushAdded()
			if delStart == 0 {
				delStart = oldLineNum
			}
			oldLineNum++
		} else {
			flushAdded()
			flushDeleted()
			newLineNum++
			oldLineNum++
		}
	}

	flushAdded()
	flushDeleted()
	return addedLines, deletedLines
}

// parseHunkHeader extracts both new and old start positions from a hunk header.
// Returns (newStart, oldStart). Returns (0, 0) if parsing fails.
func (srm *SectionRuleManager) parseHunkHeader(hunkHeader string) (newStart int, oldStart int) {
	parts := strings.Fields(hunkHeader)
	if len(parts) < 3 {
		return 0, 0
	}

	// Parse old part: -old_start,old_count
	oldPart := parts[1]
	if strings.HasPrefix(oldPart, "-") {
		oldInfo := strings.TrimPrefix(oldPart, "-")
		oldRangeParts := strings.Split(oldInfo, ",")
		if len(oldRangeParts) > 0 {
			if n, err := fmt.Sscanf(oldRangeParts[0], "%d", &oldStart); n != 1 || err != nil {
				oldStart = 0
			}
		}
	}

	// Parse new part: +new_start,new_count
	newPart := parts[2]
	if strings.HasPrefix(newPart, "+") {
		newInfo := strings.TrimPrefix(newPart, "+")
		newRangeParts := strings.Split(newInfo, ",")
		if len(newRangeParts) > 0 {
			if n, err := fmt.Sscanf(newRangeParts[0], "%d", &newStart); n != 1 || err != nil {
				newStart = 0
			}
		}
	}

	return newStart, oldStart
}

// getAffectedSections returns only the sections that contain changed lines
func (srm *SectionRuleManager) getAffectedSections(sections []shared.Section, changedLines []shared.LineRange) []shared.Section {
	var affectedSections []shared.Section

	for _, section := range sections {
		for _, changedRange := range changedLines {
			// Check if this section overlaps with any changed line range
			if srm.sectionsOverlap(section, changedRange) {
				affectedSections = append(affectedSections, section)
				break // Don't add the same section multiple times
			}
		}
	}

	return affectedSections
}

// sectionsOverlap checks if a section overlaps with a changed line range
func (srm *SectionRuleManager) sectionsOverlap(section shared.Section, changedRange shared.LineRange) bool {
	// Sections overlap if there's any line in common
	return section.StartLine <= changedRange.EndLine && section.EndLine >= changedRange.StartLine
}

func (srm *SectionRuleManager) determineOverallDecision(fileValidations map[string]*shared.FileValidationSummary, ignoredFiles []string) shared.Decision {
	// If there are no file validations, check if all files were ignored
	if len(fileValidations) == 0 {
		if len(ignoredFiles) > 0 {
			logging.Info("All changed files are in the ignore list - posting comment only, no decision")
			return shared.Decision{
				Type:    shared.CommentOnly,
				Reason:  "All changed files are in the ignore list - no decision made",
				Summary: "ℹ️ All files ignored",
				Details: fmt.Sprintf("All %d changed file(s) match ignore patterns. No validation performed, no approval decision made.", len(ignoredFiles)),
			}
		}
		logging.Warn("No files to validate - requiring manual review for safety")
		return shared.Decision{
			Type:    shared.ManualReview,
			Reason:  "MR has no files to validate",
			Summary: "⚠️ No files to validate",
			Details: "Cannot auto-approve an MR with zero validated files. This may indicate net-zero changes or an edge case.",
		}
	}

	var manualReviewFiles []string
	var approvedFiles []string
	var warehouseManualReasons []string
	var manualReviewReasons []string
	var hasUncoveredLines bool

	// Collect file results
	for _, fileValidation := range fileValidations {
		if fileValidation.FileDecision == shared.ManualReview {
			manualReviewFiles = append(manualReviewFiles, fileValidation.FilePath)
		} else {
			approvedFiles = append(approvedFiles, fileValidation.FilePath)
		}

		if fileValidation != nil && len(fileValidation.UncoveredLines) > 0 {
			hasUncoveredLines = true
		}

		if fileValidation != nil {
			for _, rr := range fileValidation.RuleResults {
				if rr.Decision == shared.ManualReview {
					if rr.RuleName == "warehouse_rule" {
						warehouseManualReasons = append(warehouseManualReasons, rr.Reason)
					} else {
						manualReviewReasons = append(manualReviewReasons, rr.Reason)
					}
				}
			}
		}
	}

	// If any file requires manual review, the entire MR requires manual review
	if len(manualReviewFiles) > 0 {
		details := fmt.Sprintf("Files requiring manual review: %s", strings.Join(manualReviewFiles, ", "))
		if len(approvedFiles) > 0 {
			details += fmt.Sprintf(". Files auto-approved: %s", strings.Join(approvedFiles, ", "))
		}

		reason := "One or more files require manual review"
		if len(warehouseManualReasons) > 0 {
			seen := make(map[string]bool)
			var uniq []string
			for _, r := range warehouseManualReasons {
				if r == "" || seen[r] {
					continue
				}
				seen[r] = true
				uniq = append(uniq, r)
			}
			if len(uniq) > 0 {
				details += fmt.Sprintf(". Warehouse: %s", strings.Join(uniq, " | "))
			}
		} else if len(manualReviewReasons) > 0 {
			details += fmt.Sprintf(". Manual review reasons: %s", strings.Join(manualReviewReasons, ", "))
		}

		logging.Info("MR requires manual review (files=%d, warehouse=%t, uncovered_lines=%t): %v",
			len(manualReviewFiles), len(warehouseManualReasons) > 0, hasUncoveredLines, manualReviewFiles)

		return shared.Decision{
			Type:    shared.ManualReview,
			Reason:  reason,
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
