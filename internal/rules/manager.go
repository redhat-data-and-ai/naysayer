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

	// Source branch files for fork MRs live on the fork project, not the target (same as warehouse analyzer).
	sourceProjectID := srm.sourceProjectIDForMR(mrCtx)

	for _, filePath := range filePaths {
		// Get file content from source branch
		fileContent, fetchErr := srm.getFileContent(filePath, mrCtx, sourceProjectID)
		if fetchErr != nil {
			logging.Warn("Cannot load source-branch file for validation (requiring manual review): %s: %v", filePath, fetchErr)
			fileValidations[filePath] = srm.createManualReviewValidation(filePath, 0, fmt.Sprintf("Could not load file from source branch: %v", fetchErr))
			continue
		}
		totalLines := shared.CountLines(fileContent)

		// Extract changed lines from the diff for delta validation
		changedLines := srm.getChangedLinesForFile(filePath, mrCtx)
		diffText := srm.getDiffForFile(filePath, mrCtx)

		// Check if this file has section-based validation
		parser := srm.getParserForFile(filePath)
		if parser != nil {
			logging.Info("Using section-based validation for file: %s", filePath)
			// Use section-based validation with delta approach
			fileValidation := srm.validateFileWithSections(filePath, fileContent, totalLines, parser, changedLines, diffText)
			fileValidations[filePath] = fileValidation
		} else {
			logging.Info("No parser found for file: %s - requiring manual review", filePath)
			// No section configuration found - require manual review
			fileValidation := srm.createManualReviewValidation(filePath, totalLines, "No section-based validation configuration found for this file type")
			fileValidations[filePath] = fileValidation
		}
	}

	// Determine overall decision
	overallDecision := srm.determineOverallDecision(fileValidations)
	return fileValidations, overallDecision
}

// getChangedLinesForFile extracts changed line ranges for a specific file from MR context
func (srm *SectionRuleManager) getChangedLinesForFile(filePath string, mrCtx *shared.MRContext) []shared.LineRange {
	for _, change := range mrCtx.Changes {
		if change.NewPath == filePath && change.Diff != "" {
			changedLines := srm.extractChangedLinesFromDiff(change.Diff)
			// Set file path for each line range
			for i := range changedLines {
				changedLines[i].FilePath = filePath
			}
			return changedLines
		}
	}
	return []shared.LineRange{}
}

func (srm *SectionRuleManager) getDiffForFile(filePath string, mrCtx *shared.MRContext) string {
	for _, change := range mrCtx.Changes {
		if change.NewPath == filePath {
			return change.Diff
		}
	}
	return ""
}

// validateFileWithSections validates a file using section-based approach with delta validation
func (srm *SectionRuleManager) validateFileWithSections(filePath, fileContent string, totalLines int, parser shared.SectionParser, changedLines []shared.LineRange, diffText string) *shared.FileValidationSummary {
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

	// Validate ALL sections to ensure all rules appear in comments
	// Track which sections were actually affected for potential future optimizations
	affectedSections := make(map[string]bool)
	if len(changedLines) > 0 {
		affected := srm.getAffectedSections(sections, changedLines)
		for _, section := range affected {
			affectedSections[section.Name] = true
		}
		logging.Info("Delta validation for %s: %d affected sections out of %d total", filePath, len(affectedSections), len(sections))
	}

	if len(affectedSections) > 0 {
		var names []string
		for name := range affectedSections {
			names = append(names, name)
		}
		sort.Strings(names)
		logging.Info("Delta validation for %s affected sections: %s", filePath, strings.Join(names, ", "))
	}

	if !affectedSections["warehouses"] && diffMentionsWarehouses(diffText) {
		affectedSections["warehouses"] = true
		logging.Info("Delta validation for %s: warehouses section flagged as affected (diff heuristic)", filePath)
	}

	// Validate all sections (not just affected ones) to show complete rule evaluation
	for _, section := range sections {
		// Get enabled rules for this section
		sectionRules := srm.getEnabledRulesForSection(section.RuleConfigs)

		// Validate the section
		sectionResult := parser.ValidateSection(&section, sectionRules)
		sectionResults = append(sectionResults, *sectionResult)

		// Add to overall results
		for _, ruleResult := range sectionResult.RuleResults {
			ruleResults = append(ruleResults, ruleResult)
			allCoveredLines = append(allCoveredLines, ruleResult.LineRanges...)
		}
	}

	// Defense-in-depth: any changes touching the warehouses section must require manual review.
	if affectedSections["warehouses"] {
		var sawWarehouseRule bool
		var sawWarehouseManual bool
		for i := range ruleResults {
			if ruleResults[i].RuleName != "warehouse_rule" {
				continue
			}
			sawWarehouseRule = true
			if ruleResults[i].Decision == shared.ManualReview {
				sawWarehouseManual = true
				break
			}
		}

		if !sawWarehouseManual {
			reason := "Warehouses section changed - manual review required"
			if sawWarehouseRule {
				for i := range ruleResults {
					if ruleResults[i].RuleName == "warehouse_rule" {
						ruleResults[i].Decision = shared.ManualReview
						ruleResults[i].Reason = reason
						ruleResults[i].WasEvaluated = true
					}
				}
			} else {
				ruleResults = append(ruleResults, shared.LineValidationResult{
					RuleName:     "warehouse_rule",
					LineRanges:   changedLines,
					Decision:     shared.ManualReview,
					Reason:       reason,
					WasEvaluated: true,
				})
			}
		}
	}

	// Check for uncovered lines (lines not in any section)
	// Only consider lines that were actually changed in this MR
	uncoveredLines := srm.getUncoveredLinesInChanges(totalLines, sections, changedLines)

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

// getParserForFile returns the appropriate section parser for a file
func (srm *SectionRuleManager) getParserForFile(filePath string) shared.SectionParser {
	for pattern, parser := range srm.sectionParsers {
		if shared.MatchesPattern(filePath, pattern) {
			return parser
		}
	}
	return nil
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

func (srm *SectionRuleManager) getFileContent(filePath string, mrCtx *shared.MRContext, sourceProjectID int) (string, error) {
	if srm.gitlabClient == nil {
		logging.Warn("GitLab client not available, cannot fetch file content for: %s", filePath)
		return "", fmt.Errorf("GitLab client not available")
	}
	if mrCtx.MRInfo == nil || mrCtx.MRInfo.SourceBranch == "" {
		return "", fmt.Errorf("source branch not available in MR context")
	}
	sourceBranch := mrCtx.MRInfo.SourceBranch
	fileContent, err := srm.gitlabClient.FetchFileContent(sourceProjectID, filePath, sourceBranch)
	if err != nil {
		logging.Warn("Failed to fetch file content for %s from project %d branch %s: %v", filePath, sourceProjectID, sourceBranch, err)
		return "", err
	}
	if fileContent == nil {
		return "", fmt.Errorf("empty response when fetching %s", filePath)
	}
	return fileContent.Content, nil
}

// extractChangedLinesFromDiff extracts the line ranges that were modified in a Git diff
func (srm *SectionRuleManager) extractChangedLinesFromDiff(diff string) []shared.LineRange {
	var changedRanges []shared.LineRange
	lines := strings.Split(diff, "\n")

	for _, line := range lines {
		if strings.HasPrefix(line, "@@") {
			// Parse hunk header like "@@ -1,4 +1,6 @@"
			if lineRange := srm.parseHunkHeader(line); lineRange != nil {
				changedRanges = append(changedRanges, *lineRange)
			}
		}
	}

	return changedRanges
}

// parseHunkHeader parses a Git diff hunk header to extract the new file line range
func (srm *SectionRuleManager) parseHunkHeader(hunkHeader string) *shared.LineRange {
	// Format: @@ -old_start,old_count +new_start,new_count @@
	parts := strings.Fields(hunkHeader)
	if len(parts) < 3 {
		return nil
	}

	newPart := parts[2] // +new_start,new_count
	if !strings.HasPrefix(newPart, "+") {
		return nil
	}

	newInfo := strings.TrimPrefix(newPart, "+")
	rangeParts := strings.Split(newInfo, ",")

	startLine := 0
	count := 1

	// Parse start line
	if len(rangeParts) > 0 {
		if n, err := fmt.Sscanf(rangeParts[0], "%d", &startLine); n != 1 || err != nil {
			return nil
		}
	}

	// Parse count if present
	if len(rangeParts) > 1 {
		if n, err := fmt.Sscanf(rangeParts[1], "%d", &count); n != 1 || err != nil {
			count = 1
		}
	}

	if startLine <= 0 || count <= 0 {
		return nil
	}

	return &shared.LineRange{
		StartLine: startLine,
		EndLine:   startLine + count - 1,
	}
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

func (srm *SectionRuleManager) determineOverallDecision(fileValidations map[string]*shared.FileValidationSummary) shared.Decision {
	// Safety check: if there are no file validations, require manual review
	// This catches edge cases like net-zero changes that slip through earlier checks
	if len(fileValidations) == 0 {
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
				if rr.RuleName == "warehouse_rule" && rr.Decision == shared.ManualReview {
					warehouseManualReasons = append(warehouseManualReasons, rr.Reason)
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
			reason = "Warehouse changes require manual review"
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
		} else if hasUncoveredLines {
			reason = "Uncovered changes require manual review"
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
