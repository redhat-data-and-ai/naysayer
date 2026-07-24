package rules

import (
	"testing"

	"github.com/redhat-data-and-ai/naysayer/internal/config"
	"github.com/redhat-data-and-ai/naysayer/internal/gitlab"
	"github.com/redhat-data-and-ai/naysayer/internal/rules/shared"
	"github.com/stretchr/testify/assert"
)

type stubSectionParser struct {
	sections   []shared.Section
	validateFn func(section *shared.Section, rules []shared.Rule) *shared.SectionValidationResult
}

func (sp *stubSectionParser) ParseSections(filePath string, content string) ([]shared.Section, error) {
	return sp.sections, nil
}

func (sp *stubSectionParser) GetSectionAtLine(sections []shared.Section, lineNumber int) *shared.Section {
	return nil
}

func (sp *stubSectionParser) ValidateSection(section *shared.Section, rules []shared.Rule) *shared.SectionValidationResult {
	if sp.validateFn != nil {
		return sp.validateFn(section, rules)
	}
	return &shared.SectionValidationResult{
		Section:     section,
		Decision:    shared.Approve,
		RuleResults: []shared.LineValidationResult{},
	}
}

func (sp *stubSectionParser) GetSectionDefinitions() map[string]config.SectionDefinition {
	return map[string]config.SectionDefinition{}
}

// stubRule is a minimal Rule implementation for unit tests.
type stubRule struct {
	name     string
	decision shared.DecisionType
	reason   string
}

func (r *stubRule) Name() string        { return r.name }
func (r *stubRule) Description() string  { return r.name }
func (r *stubRule) Version() string      { return "1.0.0" }
func (r *stubRule) IsEnabled() bool      { return true }
func (r *stubRule) SetEnabled(bool)      {}
func (r *stubRule) GetCoveredLines(filePath string, fileContent string) []shared.LineRange {
	return []shared.LineRange{{StartLine: 1, EndLine: 1, FilePath: filePath}}
}
func (r *stubRule) ValidateLines(filePath string, fileContent string, lineRanges []shared.LineRange) (shared.DecisionType, string) {
	return r.decision, r.reason
}

func TestNewSectionRuleManager(t *testing.T) {
	ruleConfig := &config.GlobalRuleConfig{
		Files: []config.FileRuleConfig{
			{
				Name:       "test-yaml",
				Path:       "test/",
				Filename:   "*.yaml",
				ParserType: "yaml",
				Enabled:    true,
				Sections: []config.SectionDefinition{
					{
						Name:     "test_section",
						YAMLPath: "spec.test",
						Required: true,
						RuleConfigs: []config.RuleConfig{
							{Name: "test_rule", Enabled: true},
						},
					},
				},
			},
		},
	}

	manager := NewSectionRuleManager(ruleConfig, nil)

	assert.NotNil(t, manager)
	assert.NotNil(t, manager.config)
	assert.Equal(t, ruleConfig, manager.config)
	assert.NotNil(t, manager.sectionParsers)
	assert.NotNil(t, manager.ruleRegistry)
}

func TestSectionRuleManager_GetParserForFile(t *testing.T) {
	ruleConfig := &config.GlobalRuleConfig{
		Files: []config.FileRuleConfig{
			{
				Name:       "yaml-files",
				Path:       "",
				Filename:   "*.yaml",
				ParserType: "yaml",
				Enabled:    true,
				Sections: []config.SectionDefinition{
					{
						Name:     "test_section",
						YAMLPath: "spec.test",
						RuleConfigs: []config.RuleConfig{
							{Name: "test_rule", Enabled: true},
						},
					},
				},
			},
		},
	}

	manager := NewSectionRuleManager(ruleConfig, nil)

	// Should return parser for YAML files
	parser := manager.getParserForFile("test.yaml")
	assert.NotNil(t, parser)

	// Should return nil for non-matching files
	parser = manager.getParserForFile("test.txt")
	assert.Nil(t, parser)
}

func TestPatternMatching(t *testing.T) {
	tests := []struct {
		name     string
		filePath string
		pattern  string
		expected bool
	}{
		{"exact match", "test.yaml", "test.yaml", true},
		{"wildcard match", "test.yaml", "*.yaml", true},
		{"no match", "test.txt", "*.yaml", false},
		{"directory pattern", "dir/test.yaml", "dir/*.yaml", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := shared.MatchesPattern(tt.filePath, tt.pattern)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestSectionRuleManager_DetermineOverallDecision_ZeroFiles(t *testing.T) {
	ruleConfig := &config.GlobalRuleConfig{
		Files: []config.FileRuleConfig{},
	}

	manager := NewSectionRuleManager(ruleConfig, nil)

	// Test with empty file validations - should require manual review
	emptyValidations := make(map[string]*shared.FileValidationSummary)
	decision := manager.determineOverallDecision(emptyValidations)

	assert.Equal(t, shared.ManualReview, decision.Type)
	assert.Contains(t, decision.Reason, "no files to validate")
	assert.Contains(t, decision.Summary, "No files to validate")
}

func TestSectionRuleManager_DetermineOverallDecision_WithFiles(t *testing.T) {
	ruleConfig := &config.GlobalRuleConfig{
		Files: []config.FileRuleConfig{},
	}

	manager := NewSectionRuleManager(ruleConfig, nil)

	// Test with approved files - should approve
	approvedValidations := map[string]*shared.FileValidationSummary{
		"test.yaml": {
			FilePath:     "test.yaml",
			FileDecision: shared.Approve,
		},
	}
	decision := manager.determineOverallDecision(approvedValidations)

	assert.Equal(t, shared.Approve, decision.Type)

	// Test with manual review files - should require manual review
	reviewValidations := map[string]*shared.FileValidationSummary{
		"test.yaml": {
			FilePath:     "test.yaml",
			FileDecision: shared.ManualReview,
		},
	}
	decision = manager.determineOverallDecision(reviewValidations)

	assert.Equal(t, shared.ManualReview, decision.Type)
}

func TestSectionRuleManager_GetExpectedRulesForAffectedSections(t *testing.T) {
	manager := NewSectionRuleManager(&config.GlobalRuleConfig{Files: []config.FileRuleConfig{}}, nil)

	sections := []shared.Section{
		{
			Name: "warehouses",
			RuleConfigs: []config.RuleConfig{
				{Name: "warehouse_rule", Enabled: true},
				{Name: "disabled_rule", Enabled: false},
				{Name: "", Enabled: true},
			},
		},
		{
			Name: "workload",
			RuleConfigs: []config.RuleConfig{
				{Name: "warehouse_rule", Enabled: true},
				{Name: "second_rule", Enabled: true},
			},
		},
		{
			Name: "metadata",
			RuleConfigs: []config.RuleConfig{
				{Name: "metadata_rule", Enabled: true},
			},
		},
	}

	affectedSections := map[string]bool{
		"warehouses": true,
		"workload":   true,
	}

	expected := manager.getExpectedRulesForAffectedSections(sections, affectedSections)
	assert.Equal(t, []string{"second_rule", "warehouse_rule"}, expected)
}

func TestSectionRuleManager_AppendMissingExpectedRuleFallbacks_AddsMissingRule(t *testing.T) {
	manager := NewSectionRuleManager(&config.GlobalRuleConfig{Files: []config.FileRuleConfig{}}, nil)

	changedLines := []shared.LineRange{
		{StartLine: 12, EndLine: 14, FilePath: "product.yaml"},
	}

	ruleResults := []shared.LineValidationResult{
		{
			RuleName:     "metadata_rule",
			LineRanges:   []shared.LineRange{{StartLine: 1, EndLine: 3, FilePath: "product.yaml"}},
			Decision:     shared.Approve,
			Reason:       "metadata section is valid",
			WasEvaluated: true,
		},
	}

	got := manager.appendMissingExpectedRuleFallbacks(
		ruleResults,
		[]string{"metadata_rule", "warehouse_rule"},
		changedLines,
	)

	assert.Len(t, got, 2)
	assert.Equal(t, "metadata_rule", got[0].RuleName)

	fallback := got[1]
	assert.Equal(t, "warehouse_rule", fallback.RuleName)
	assert.Equal(t, shared.ManualReview, fallback.Decision)
	assert.Equal(t, changedLines, fallback.LineRanges)
	assert.False(t, fallback.WasEvaluated)
	assert.Contains(t, fallback.Reason, "warehouse_rule")
	assert.Contains(t, fallback.Reason, "not evaluated")
}

func TestSectionRuleManager_AppendMissingExpectedRuleFallbacks_DoesNotOverwriteExistingRule(t *testing.T) {
	manager := NewSectionRuleManager(&config.GlobalRuleConfig{Files: []config.FileRuleConfig{}}, nil)

	originalReason := "Warehouse size increase detected: user warehouse: SMALL -> MEDIUM"
	ruleResults := []shared.LineValidationResult{
		{
			RuleName:     "warehouse_rule",
			Decision:     shared.ManualReview,
			Reason:       originalReason,
			WasEvaluated: true,
		},
	}

	got := manager.appendMissingExpectedRuleFallbacks(
		ruleResults,
		[]string{"warehouse_rule"},
		[]shared.LineRange{{StartLine: 8, EndLine: 12, FilePath: "product.yaml"}},
	)

	assert.Len(t, got, 1)
	assert.Equal(t, "warehouse_rule", got[0].RuleName)
	assert.Equal(t, originalReason, got[0].Reason)
	assert.True(t, got[0].WasEvaluated)
}

// extractChangedLinesFromDiff must return only actually-added lines, not the full
// hunk range. When a section like "warehouses:" appears only as a context line in
// the diff (not modified), it must NOT be flagged as an affected section.
//
// Regression: adding ai_experimental above the warehouses key produces a diff like:
//   @@ -1,6 +1,8 @@
//    name: accountsreceivable
//   +ai_experimental: true
//    warehouses:
//
// Only lines 4-5 were added. The warehouses section (line 6+) is unchanged.
func TestExtractChangedLinesFromDiff_IncludesContextLinesInRange(t *testing.T) {
	manager := NewSectionRuleManager(&config.GlobalRuleConfig{Files: []config.FileRuleConfig{}}, nil)

	// Actual diff from MR !12606: adds ai_ready/ai_experimental before warehouses
	diff := "@@ -1,6 +1,8 @@\n name: accountsreceivable\n kind: aggregated\n rover_group: dataverse-aggregate-accountsreceivable\n+ai_ready: false\n+ai_experimental: true\n warehouses:\n - type: user\n   size: XSMALL\n"

	changedLines := manager.extractChangedLinesFromDiff(diff)

	// Only the two added lines (lines 4-5 in the new file) should be reported
	assert.Len(t, changedLines, 1)
	assert.Equal(t, 4, changedLines[0].StartLine)
	assert.Equal(t, 5, changedLines[0].EndLine)

	// The warehouses section starts at line 6 in the new file.
	// Since changed range is 4-5, it does NOT overlap with warehouses (6-8).
	warehouseSection := shared.Section{Name: "warehouses", StartLine: 6, EndLine: 8}
	affected := manager.getAffectedSections([]shared.Section{warehouseSection}, changedLines)
	assert.Len(t, affected, 0, "warehouses was not modified, should not be affected")
}

func TestSectionRuleManager_ValidateFileWithSections_AddsFallbackForMissingExpectedRule(t *testing.T) {
	manager := NewSectionRuleManager(&config.GlobalRuleConfig{Files: []config.FileRuleConfig{}}, nil)

	parser := &stubSectionParser{
		sections: []shared.Section{
			{
				Name:      "warehouses",
				StartLine: 10,
				EndLine:   20,
				FilePath:  "product.yaml",
				RuleConfigs: []config.RuleConfig{
					{Name: "warehouse_rule", Enabled: true},
				},
			},
		},
		validateFn: func(section *shared.Section, rules []shared.Rule) *shared.SectionValidationResult {
			// No registered rules means section validation emits no rule results.
			assert.Empty(t, rules)
			return &shared.SectionValidationResult{
				Section:     section,
				Decision:    shared.Approve,
				RuleResults: []shared.LineValidationResult{},
			}
		},
	}

	changedLines := []shared.LineRange{
		{StartLine: 12, EndLine: 12, FilePath: "product.yaml"},
	}

	result := manager.validateFileWithSections(
		"product.yaml",
		"name: test",
		30,
		parser,
		changedLines,
		"+warehouses:",
	)

	assert.Equal(t, shared.ManualReview, result.FileDecision)
	assert.Len(t, result.UncoveredLines, 0)
	assert.Len(t, result.RuleResults, 1)

	fallback := result.RuleResults[0]
	assert.Equal(t, "warehouse_rule", fallback.RuleName)
	assert.Equal(t, shared.ManualReview, fallback.Decision)
	assert.Equal(t, changedLines, fallback.LineRanges)
	assert.False(t, fallback.WasEvaluated)
	assert.Contains(t, fallback.Reason, "not evaluated")
}

func TestValidateFileWithSections_UnaffectedSectionDoesNotBlockApproval(t *testing.T) {
	manager := NewSectionRuleManager(&config.GlobalRuleConfig{Files: []config.FileRuleConfig{}}, nil)

	// Register a stub rule that always returns ManualReview
	manager.AddRule(&stubRule{name: "blocking_rule", decision: shared.ManualReview, reason: "blocked"})
	// Register a stub rule that always approves
	manager.AddRule(&stubRule{name: "metadata_rule", decision: shared.Approve, reason: "approved"})

	parser := &stubSectionParser{
		sections: []shared.Section{
			{
				Name:      "tags",
				StartLine: 4,
				EndLine:   7,
				FilePath:  "product.yaml",
				RuleConfigs: []config.RuleConfig{
					{Name: "metadata_rule", Enabled: true},
				},
				AutoApprove: true,
			},
			{
				Name:      "warehouses",
				StartLine: 8,
				EndLine:   12,
				FilePath:  "product.yaml",
				RuleConfigs: []config.RuleConfig{
					{Name: "blocking_rule", Enabled: true},
				},
				AutoApprove: false,
			},
		},
		validateFn: func(section *shared.Section, rules []shared.Rule) *shared.SectionValidationResult {
			if len(rules) == 0 {
				return &shared.SectionValidationResult{Section: section, Decision: shared.Approve}
			}
			decision, reason := rules[0].ValidateLines(section.FilePath, section.Content, nil)
			return &shared.SectionValidationResult{
				Section:  section,
				Decision: decision,
				Reason:   reason,
				RuleResults: []shared.LineValidationResult{
					{
						RuleName:     rules[0].Name(),
						Decision:     decision,
						Reason:       reason,
						LineRanges:   []shared.LineRange{{StartLine: section.StartLine, EndLine: section.EndLine, FilePath: section.FilePath}},
						WasEvaluated: true,
					},
				},
			}
		},
	}

	// Changed lines only overlap with 'tags' (lines 6-7), not 'warehouses' (lines 8-12)
	changedLines := []shared.LineRange{
		{StartLine: 6, EndLine: 7, FilePath: "product.yaml"},
	}

	result := manager.validateFileWithSections(
		"product.yaml",
		"kind: DataProduct\nname: analytics\nrover_group: team\ntags:\n  env: prod\n  version: v1.1.0\n  owner: team\nwarehouses:\n  - name: wh\n    type: user\n    size: XSMALL\n",
		12,
		parser,
		changedLines,
		"+  version: v1.1.0\n+  owner: team",
	)

	assert.Equal(t, shared.Approve, result.FileDecision,
		"unaffected warehouses section (ManualReview) must not block approval")
}

func TestIsDeletedFile(t *testing.T) {
	manager := NewSectionRuleManager(&config.GlobalRuleConfig{Files: []config.FileRuleConfig{}}, nil)

	changes := []gitlab.FileChange{
		{OldPath: "dataproducts/prod/pii_masking.yaml", NewPath: "", DeletedFile: true, Diff: "@@ -1,5 +0,0 @@"},
		{OldPath: "dataproducts/prod/product.yaml", NewPath: "dataproducts/prod/product.yaml", DeletedFile: false, Diff: "@@ -1,3 +1,4 @@"},
	}

	assert.True(t, manager.isDeletedFile("dataproducts/prod/pii_masking.yaml", changes))
	assert.False(t, manager.isDeletedFile("dataproducts/prod/product.yaml", changes))
	assert.False(t, manager.isDeletedFile("nonexistent.yaml", changes))
}

func TestGetDeletionReason(t *testing.T) {
	ruleConfig := &config.GlobalRuleConfig{
		Files: []config.FileRuleConfig{
			{Name: "masking_files", Path: "dataproducts/**/", Filename: "*masking.{yaml,yml}", Enabled: true},
			{Name: "tag_files", Path: "dataproducts/**/", Filename: "*tag*.{yaml,yml}", Enabled: true},
			{Name: "product_configs", Path: "dataproducts/**/", Filename: "product.{yaml,yml}", Enabled: true},
		},
	}
	manager := NewSectionRuleManager(ruleConfig, nil)

	assert.Equal(t, "Deletion requires manual review: masking_files",
		manager.getDeletionReason("dataproducts/source/analytics/prod/pii_masking.yaml"))
	assert.Equal(t, "Deletion requires manual review: tag_files",
		manager.getDeletionReason("dataproducts/source/analytics/sandbox/tag_pii.yaml"))
	assert.Equal(t, "Deletion requires manual review: product_configs",
		manager.getDeletionReason("dataproducts/source/analytics/prod/product.yaml"))
	assert.Equal(t, "Deletion requires manual review: dataproducts/source/analytics/unknown_file.yaml",
		manager.getDeletionReason("dataproducts/source/analytics/unknown_file.yaml"))
}

// func TestParseHunkHeader(t *testing.T) {
// 	manager := NewSectionRuleManager(&config.GlobalRuleConfig{Files: []config.FileRuleConfig{}}, nil)
// 	tests := []struct {
// 		name     string
// 		header   string
// 		expected *shared.LineRange
// 	} {
// 		{
// 			name:     "lines added",
// 			header: "@@ -574,0 +577,3 @@ func (srm *SectionRuleManager) extractChangedLinesFromDiff(diff string) []shared",
// 			expected: &shared.LineRange{StartLine: 577, EndLine: 580},
// 		},
// 		{
// 			name:     "line replaced",
// 			header: "@@ -570 +570,3 @@ func (srm *SectionRuleManager) getFileContent(filePath string, mrCtx *shared.MRC",
// 			expected: &shared.LineRange{StartLine: 570, EndLine: 573},
// 		},
// 		{
// 			name:     "lines replaced",
// 			header: "@@ -577,3 +582,18 @@ func (srm *SectionRuleManager) extractChangedLinesFromDiff(diff string) []shared",
// 			expected: &shared.LineRange{StartLine: 582, EndLine: 599},
// 		},
// 		{
// 			name:     "lines removed",
// 			header: "@@ -11,17 +10,0 @@ Naysayer provides three core capabilities through webhook endpoints:",
// 			expected: &shared.LineRange{StartLine: 10, EndLine: 10},
// 		},
// 		{
// 			name:     "line updated",
// 			header: "@@ -238 +221 @@ kubectl apply -f config/",
// 			expected: &shared.LineRange{StartLine: 221, EndLine: 222},
// 		},
// 		{
// 			name:     "new file",
// 			header: "@@ -0,0 +1,3 @@ this is untracked",
// 			expected: &shared.LineRange{StartLine: 1, EndLine: 3},
// 		},
// 		{
// 			name:     "deleted file",
// 			header: "@@ -1,6 +0,0 @@ this is deleted",
// 			expected: &shared.LineRange{StartLine: 0, EndLine: 0},
// 		},
// 	}
// 		// @@ -570 +570,3 @@ func (srm *SectionRuleManager) getFileContent(filePath string, mrCtx *shared.MRC
// 		// 	@@ -577,3 +582,18 @@ func (srm *SectionRuleManager) extractChangedLinesFromDiff(diff string) []shared

// 		// 	@@ -11,17 +10,0 @@ Naysayer provides three core capabilities through webhook endpoints:
// 		// 	@@ -238 +221 @@ kubectl apply -f config/



// 	for _, tt := range tests {
// 		t.Run(tt.name, func(t *testing.T) {
// 			result := manager.parseHunkHeader(tt.header)
// 			assert.Equal(t, tt.expected, result)
// 		})
// 	}
// }
