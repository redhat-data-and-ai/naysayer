package rules

import (
	"fmt"
	"strings"
	"testing"

	"github.com/redhat-data-and-ai/naysayer/internal/config"
	"github.com/redhat-data-and-ai/naysayer/internal/gitlab"
	"github.com/redhat-data-and-ai/naysayer/internal/rules/shared"
	"github.com/stretchr/testify/assert"
)

type stubSectionParser struct {
	sections   []shared.Section
	validateFn func(section *shared.Section, rules []shared.Rule) *shared.SectionValidationResult
	parseFn    func(filePath string, content string) ([]shared.Section, error)
}

func (sp *stubSectionParser) ParseSections(filePath string, content string) ([]shared.Section, error) {
	if sp.parseFn != nil {
		return sp.parseFn(filePath, content)
	}
	out := make([]shared.Section, len(sp.sections))
	copy(out, sp.sections)
	for i := range out {
		out[i].Content = content
		if out[i].FilePath == "" {
			out[i].FilePath = filePath
		}
	}
	return out, nil
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
func (r *stubRule) Description() string { return r.name }
func (r *stubRule) Version() string     { return "1.0.0" }
func (r *stubRule) IsEnabled() bool     { return true }
func (r *stubRule) SetEnabled(bool)     {}
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

// A root-level CODEOWNERS file must be picked up by the parser configured with
// path:"**/" filename:"CODEOWNERS" (producing pattern "**/CODEOWNERS").
func TestSectionRuleManager_GetParserForFile_RootCodeowners(t *testing.T) {
	ruleConfig := &config.GlobalRuleConfig{
		Files: []config.FileRuleConfig{
			{
				Name:       "codeowners_file",
				Path:       "**/",
				Filename:   "CODEOWNERS",
				ParserType: "yaml",
				Enabled:    true,
				Sections: []config.SectionDefinition{
					{
						Name:     "codeowners_sync_validation",
						YAMLPath: ".",
						RuleConfigs: []config.RuleConfig{
							{Name: "codeowners_sync_rule", Enabled: true},
						},
						AutoApprove: true,
					},
				},
			},
		},
	}

	manager := NewSectionRuleManager(ruleConfig, nil)

	// Root-level CODEOWNERS (no directory prefix) must match
	parser := manager.getParserForFile("CODEOWNERS")
	assert.NotNil(t, parser, "root-level CODEOWNERS must be picked up by **/CODEOWNERS pattern")

	// Nested CODEOWNERS should also match
	parser = manager.getParserForFile("some/path/CODEOWNERS")
	assert.NotNil(t, parser, "nested CODEOWNERS must also match **/CODEOWNERS pattern")

	// Non-CODEOWNERS files should not match
	parser = manager.getParserForFile("CODEOWNERS.bak")
	assert.Nil(t, parser)
	parser = manager.getParserForFile("product.yaml")
	assert.Nil(t, parser)
}

// End-to-end: a root-level CODEOWNERS change must be picked up and routed through
// section-based validation. Uses the same config shape as rules.yaml.
func TestSectionRuleManager_CodeownersFileValidation(t *testing.T) {
	ruleConfig := &config.GlobalRuleConfig{
		Files: []config.FileRuleConfig{
			{
				Name:       "codeowners_file",
				Path:       "**/",
				Filename:   "CODEOWNERS",
				ParserType: "yaml",
				Enabled:    true,
				Sections: []config.SectionDefinition{
					{
						Name:     "codeowners_sync_validation",
						YAMLPath: ".",
						RuleConfigs: []config.RuleConfig{
							{Name: "codeowners_sync_rule", Enabled: true},
						},
						AutoApprove: true,
					},
				},
			},
		},
	}

	manager := NewSectionRuleManager(ruleConfig, nil)

	// Step 1: Parser must be found for root-level CODEOWNERS
	parser := manager.getParserForFile("CODEOWNERS")
	assert.NotNil(t, parser, "parser must be found for root-level CODEOWNERS")

	// Step 2: Diff parsing must return only the actually changed line
	diff := "@@ -1,3 +1,3 @@\n # Data Product Owners\n [Aggregate Data Products]\n-/dataproducts/aggregate/analytics/ @alice @bob\n+/dataproducts/aggregate/analytics/ @alice @bob @charlie\n"
	changedLines, _ := manager.extractChangedLinesFromDiff(diff)
	assert.Len(t, changedLines, 1)
	assert.Equal(t, 3, changedLines[0].StartLine)
	assert.Equal(t, 3, changedLines[0].EndLine)

	// Step 3: CODEOWNERS content is parseable as YAML (Go yaml.v3 accepts it).
	// The section parser finds the full-file section, and the codeowners_sync_validation
	// section is affected by the change.
	codeownersContent := "# Data Product Owners\n[Aggregate Data Products]\n/dataproducts/aggregate/analytics/ @alice @bob @charlie\n"
	sections, err := parser.ParseSections("CODEOWNERS", codeownersContent)
	assert.NoError(t, err, "CODEOWNERS content must be parseable")
	assert.NotEmpty(t, sections, "full-file section must be found")

	affected := manager.getAffectedSections(sections, changedLines)
	assert.Len(t, affected, 1, "codeowners_sync_validation section must be affected")
	assert.Equal(t, "codeowners_sync_validation", affected[0].Name)

	// Step 4: Full validation — codeowners_sync_rule is not registered in the manager
	// (no AddRule was called), so the fallback mechanism injects a manual review.
	result := manager.validateFileWithSections("CODEOWNERS", codeownersContent, "", parser, changedLines, nil, diff)
	assert.NotNil(t, result)
	assert.Equal(t, shared.ManualReview, result.FileDecision,
		"codeowners_sync_rule not registered → fallback manual review expected")
}

func TestSectionRuleManager_DetermineOverallDecision_ZeroFiles(t *testing.T) {
	ruleConfig := &config.GlobalRuleConfig{
		Files: []config.FileRuleConfig{},
	}

	manager := NewSectionRuleManager(ruleConfig, nil)

	// Test with empty file validations and no ignored files - should require manual review
	emptyValidations := make(map[string]*shared.FileValidationSummary)
	decision := manager.determineOverallDecision(emptyValidations, nil)

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
	decision := manager.determineOverallDecision(approvedValidations, nil)

	assert.Equal(t, shared.Approve, decision.Type)

	// Test with manual review files - should require manual review
	reviewValidations := map[string]*shared.FileValidationSummary{
		"test.yaml": {
			FilePath:     "test.yaml",
			FileDecision: shared.ManualReview,
		},
	}
	decision = manager.determineOverallDecision(reviewValidations, nil)

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
//
//	@@ -1,6 +1,8 @@
//	 name: accountsreceivable
//	+ai_experimental: true
//	 warehouses:
//
// Only lines 4-5 were added. The warehouses section (line 6+) is unchanged.
func TestExtractChangedLinesFromDiff_IncludesContextLinesInRange(t *testing.T) {
	manager := NewSectionRuleManager(&config.GlobalRuleConfig{Files: []config.FileRuleConfig{}}, nil)

	// Actual diff from MR !12606: adds ai_ready/ai_experimental before warehouses
	diff := "@@ -1,6 +1,8 @@\n name: accountsreceivable\n kind: aggregated\n rover_group: dataverse-aggregate-accountsreceivable\n+ai_ready: false\n+ai_experimental: true\n warehouses:\n - type: user\n   size: XSMALL\n"

	changedLines, _ := manager.extractChangedLinesFromDiff(diff)

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

// Verify that a CODEOWNERS file change is correctly picked up: the modified line
// must fall inside the codeowners section so the codeowners_sync_rule is triggered.
func TestExtractChangedLinesFromDiff_CodeownersChangeDetected(t *testing.T) {
	manager := NewSectionRuleManager(&config.GlobalRuleConfig{Files: []config.FileRuleConfig{}}, nil)

	// Typical CODEOWNERS diff: adding a new owner to an existing line
	// Before: /dataproducts/aggregate/analytics/ @alice @bob
	// After:  /dataproducts/aggregate/analytics/ @alice @bob @charlie
	diff := "@@ -1,3 +1,3 @@\n # Data Product Owners\n [Aggregate Data Products]\n-/dataproducts/aggregate/analytics/ @alice @bob\n+/dataproducts/aggregate/analytics/ @alice @bob @charlie\n"

	changedLines, _ := manager.extractChangedLinesFromDiff(diff)

	// The deletion (-) on old line 3 and addition (+) on new line 3
	// should produce a single changed range at line 3
	assert.Len(t, changedLines, 1)
	assert.Equal(t, 3, changedLines[0].StartLine)
	assert.Equal(t, 3, changedLines[0].EndLine)

	// CODEOWNERS is configured as a single full-file section (yaml_path: .)
	// covering lines 1-3. The change at line 3 must overlap.
	codeownersSection := shared.Section{
		Name:      "codeowners_sync_validation",
		StartLine: 1,
		EndLine:   3,
		RuleConfigs: []config.RuleConfig{
			{Name: "codeowners_sync_rule", Enabled: true},
		},
	}
	affected := manager.getAffectedSections([]shared.Section{codeownersSection}, changedLines)
	assert.Len(t, affected, 1, "CODEOWNERS change should be detected in the codeowners section")
	assert.Equal(t, "codeowners_sync_validation", affected[0].Name)
}

// Verify a CODEOWNERS diff that adds a new line is picked up correctly.
func TestExtractChangedLinesFromDiff_CodeownersNewLineAdded(t *testing.T) {
	manager := NewSectionRuleManager(&config.GlobalRuleConfig{Files: []config.FileRuleConfig{}}, nil)

	// Adding a brand new data product entry to CODEOWNERS
	diff := "@@ -1,3 +1,4 @@\n # Data Product Owners\n [Aggregate Data Products]\n /dataproducts/aggregate/analytics/ @alice @bob\n+/dataproducts/aggregate/newproduct/ @dave\n"

	changedLines, _ := manager.extractChangedLinesFromDiff(diff)

	// Only the added line (line 4 in new file) should be reported
	assert.Len(t, changedLines, 1)
	assert.Equal(t, 4, changedLines[0].StartLine)
	assert.Equal(t, 4, changedLines[0].EndLine)

	// Full-file section now spans 1-4, so the addition at line 4 is inside it
	codeownersSection := shared.Section{
		Name:      "codeowners_sync_validation",
		StartLine: 1,
		EndLine:   4,
	}
	affected := manager.getAffectedSections([]shared.Section{codeownersSection}, changedLines)
	assert.Len(t, affected, 1, "new CODEOWNERS line should be detected")
}

// --- Tests for extractChangedLinesFromDiff returning (addedLines, deletedLines) ---

func TestExtractChangedLinesFromDiff_PureAddition(t *testing.T) {
	manager := NewSectionRuleManager(&config.GlobalRuleConfig{Files: []config.FileRuleConfig{}}, nil)

	// Two lines added after line 3 in new file, no deletions
	diff := "@@ -1,3 +1,5 @@\n name: testproduct\n kind: aggregated\n rover_group: test\n+ai_ready: false\n+ai_experimental: true\n"

	addedLines, deletedLines := manager.extractChangedLinesFromDiff(diff)

	// Added lines should be at positions 4-5 in the new file
	assert.Len(t, addedLines, 1)
	assert.Equal(t, 4, addedLines[0].StartLine)
	assert.Equal(t, 5, addedLines[0].EndLine)

	// No deleted lines
	assert.Empty(t, deletedLines)
}

func TestExtractChangedLinesFromDiff_PureDeletion(t *testing.T) {
	manager := NewSectionRuleManager(&config.GlobalRuleConfig{Files: []config.FileRuleConfig{}}, nil)

	// Two consumers deleted from old file at lines 4-5, nothing added
	// @@ -1,5 +1,3 @@ means old file had 5 lines starting at 1, new file has 3 lines starting at 1
	diff := "@@ -1,5 +1,3 @@\n name: testproduct\n kind: aggregated\n rover_group: test\n-  - consumer_one\n-  - consumer_two\n"

	addedLines, deletedLines := manager.extractChangedLinesFromDiff(diff)

	// No added lines
	assert.Empty(t, addedLines)

	// Deleted lines were at positions 4-5 in the old file
	assert.Len(t, deletedLines, 1)
	assert.Equal(t, 4, deletedLines[0].StartLine)
	assert.Equal(t, 5, deletedLines[0].EndLine)
}

func TestExtractChangedLinesFromDiff_ValueChange(t *testing.T) {
	manager := NewSectionRuleManager(&config.GlobalRuleConfig{Files: []config.FileRuleConfig{}}, nil)

	// access_policy changed from rh_internal to public (1 delete + 1 add at same position)
	diff := "@@ -1,4 +1,4 @@\n name: testproduct\n kind: aggregated\n rover_group: test\n-access_policy: rh_internal\n+access_policy: public\n"

	addedLines, deletedLines := manager.extractChangedLinesFromDiff(diff)

	// Added line at position 4 in new file
	assert.Len(t, addedLines, 1)
	assert.Equal(t, 4, addedLines[0].StartLine)
	assert.Equal(t, 4, addedLines[0].EndLine)

	// Deleted line at position 4 in old file
	assert.Len(t, deletedLines, 1)
	assert.Equal(t, 4, deletedLines[0].StartLine)
	assert.Equal(t, 4, deletedLines[0].EndLine)
}

func TestExtractChangedLinesFromDiff_MixedAdditionAndDeletion(t *testing.T) {
	manager := NewSectionRuleManager(&config.GlobalRuleConfig{Files: []config.FileRuleConfig{}}, nil)

	// access_policy removed (line 4 old) and consumers: removed (line 5 old),
	// replaced with consumers: and - new_consumer in new file.
	// Old: name, kind, rover_group, access_policy: rh_internal, consumers:
	// New: name, kind, rover_group, consumers:, - new_consumer
	// Git groups deletions before additions in the same change region.
	diff := "@@ -1,5 +1,5 @@\n name: testproduct\n kind: aggregated\n rover_group: test\n-access_policy: rh_internal\n-consumers:\n+consumers:\n+  - new_consumer\n"

	addedLines, deletedLines := manager.extractChangedLinesFromDiff(diff)

	// Added: lines 4-5 in new file (consumers: and - new_consumer)
	assert.Len(t, addedLines, 1)
	assert.Equal(t, 4, addedLines[0].StartLine)
	assert.Equal(t, 5, addedLines[0].EndLine)

	// Deleted: lines 4-5 in old file (access_policy and consumers:)
	assert.Len(t, deletedLines, 1)
	assert.Equal(t, 4, deletedLines[0].StartLine)
	assert.Equal(t, 5, deletedLines[0].EndLine)
}

func TestExtractChangedLinesFromDiff_MultipleHunks(t *testing.T) {
	manager := NewSectionRuleManager(&config.GlobalRuleConfig{Files: []config.FileRuleConfig{}}, nil)

	// Two separate hunks: one addition at top, one deletion at bottom
	diff := "@@ -1,3 +1,4 @@\n name: testproduct\n kind: aggregated\n+ai_ready: false\n rover_group: test\n@@ -10,3 +11,2 @@\n   - consumer_a\n-  - consumer_b\n   - consumer_c\n"

	addedLines, deletedLines := manager.extractChangedLinesFromDiff(diff)

	// Hunk 1: added line at position 3 in new file
	assert.Len(t, addedLines, 1)
	assert.Equal(t, 3, addedLines[0].StartLine)
	assert.Equal(t, 3, addedLines[0].EndLine)

	// Hunk 2: deleted line at position 11 in old file
	assert.Len(t, deletedLines, 1)
	assert.Equal(t, 11, deletedLines[0].StartLine)
	assert.Equal(t, 11, deletedLines[0].EndLine)
}

func TestExtractChangedLinesFromDiff_NewFile(t *testing.T) {
	manager := NewSectionRuleManager(&config.GlobalRuleConfig{Files: []config.FileRuleConfig{}}, nil)

	// Entirely new file: all lines are additions
	diff := "@@ -0,0 +1,3 @@\n+name: newproduct\n+kind: aggregated\n+rover_group: test\n"

	addedLines, deletedLines := manager.extractChangedLinesFromDiff(diff)

	// All lines added: 1-3 in new file
	assert.Len(t, addedLines, 1)
	assert.Equal(t, 1, addedLines[0].StartLine)
	assert.Equal(t, 3, addedLines[0].EndLine)

	// No deletions
	assert.Empty(t, deletedLines)
}

func TestExtractChangedLinesFromDiff_ConsecutiveDeletedLines(t *testing.T) {
	manager := NewSectionRuleManager(&config.GlobalRuleConfig{Files: []config.FileRuleConfig{}}, nil)

	// Three consecutive lines deleted from old file
	diff := "@@ -1,6 +1,3 @@\n name: testproduct\n kind: aggregated\n rover_group: test\n-access_policy: rh_internal\n-consumers:\n-  - old_consumer\n"

	addedLines, deletedLines := manager.extractChangedLinesFromDiff(diff)

	// No additions
	assert.Empty(t, addedLines)

	// Deleted lines 4-6 in old file
	assert.Len(t, deletedLines, 1)
	assert.Equal(t, 4, deletedLines[0].StartLine)
	assert.Equal(t, 6, deletedLines[0].EndLine)
}

func TestExtractChangedLinesFromDiff_DeletedLinesBrokenByContext(t *testing.T) {
	manager := NewSectionRuleManager(&config.GlobalRuleConfig{Files: []config.FileRuleConfig{}}, nil)

	// Two non-consecutive deleted lines separated by a context line
	diff := "@@ -1,5 +1,3 @@\n name: testproduct\n-access_policy: rh_internal\n kind: aggregated\n-ai_ready: true\n rover_group: test\n"

	addedLines, deletedLines := manager.extractChangedLinesFromDiff(diff)

	// No additions
	assert.Empty(t, addedLines)

	// Two separate deletion ranges in old file: line 2 and line 4
	assert.Len(t, deletedLines, 2)
	assert.Equal(t, 2, deletedLines[0].StartLine)
	assert.Equal(t, 2, deletedLines[0].EndLine)
	assert.Equal(t, 4, deletedLines[1].StartLine)
	assert.Equal(t, 4, deletedLines[1].EndLine)
}

func TestExtractChangedLinesFromDiff_EmptyDiff(t *testing.T) {
	manager := NewSectionRuleManager(&config.GlobalRuleConfig{Files: []config.FileRuleConfig{}}, nil)

	addedLines, deletedLines := manager.extractChangedLinesFromDiff("")

	assert.Empty(t, addedLines)
	assert.Empty(t, deletedLines)
}

func TestExtractChangedLinesFromDiff_NoNewlineMarker(t *testing.T) {
	manager := NewSectionRuleManager(&config.GlobalRuleConfig{Files: []config.FileRuleConfig{}}, nil)

	diff := "@@ -1,2 +1,3 @@\n line1\n-line2\n\\ No newline at end of file\n+line2\n+line3\n"

	addedLines, deletedLines := manager.extractChangedLinesFromDiff(diff)

	assert.Len(t, deletedLines, 1)
	assert.Equal(t, 2, deletedLines[0].StartLine)
	assert.Equal(t, 2, deletedLines[0].EndLine)

	assert.Len(t, addedLines, 1)
	assert.Equal(t, 2, addedLines[0].StartLine)
	assert.Equal(t, 3, addedLines[0].EndLine)
}

func TestEvaluateAll_EmptyDiffFileRequiresManualReview(t *testing.T) {
	manager := NewSectionRuleManager(&config.GlobalRuleConfig{
		Enabled: true,
		Files: []config.FileRuleConfig{
			{
				Name:       "product_configs",
				Path:       "**/",
				Filename:   "product.yaml",
				ParserType: "yaml",
				Enabled:    true,
				Sections: []config.SectionDefinition{
					{
						Name:        "name",
						YAMLPath:    "name",
						AutoApprove: true,
					},
				},
			},
		},
	}, nil)

	result := manager.EvaluateAll(&shared.MRContext{
		ProjectID: 123,
		MRIID:     456,
		Changes: []gitlab.FileChange{
			{NewPath: "product.yaml", Diff: ""},
		},
		MRInfo: &gitlab.MRInfo{
			Title:        "chmod only",
			Author:       "developer",
			SourceBranch: "feature/empty-diff",
			TargetBranch: "main",
		},
	})

	assert.Equal(t, shared.ManualReview, result.FinalDecision.Type)
	assert.Equal(t, 1, result.TotalFiles)
	assert.Equal(t, 0, result.ApprovedFiles)
	assert.Equal(t, 1, result.ReviewFiles)

	fileValidation := result.FileValidations["product.yaml"]
	assert.NotNil(t, fileValidation)
	assert.Equal(t, shared.ManualReview, fileValidation.FileDecision)
}

func TestEvaluateAll_MixedEmptyDiffRequiresManualReview(t *testing.T) {
	mockClient := &forkMRTestGitLabClient{
		targetProjectID: 123,
		sourceProjectID: 123,
		targetBranch:    "main",
		sourceBranch:    "feature/mixed-empty-diff",
		beforeYAML:      "name: oldname\n",
		afterYAML:       "name: newname\n",
	}

	manager := NewSectionRuleManager(&config.GlobalRuleConfig{
		Enabled: true,
		Files: []config.FileRuleConfig{
			{
				Name:       "product_configs",
				Path:       "**/",
				Filename:   "product.yaml",
				ParserType: "yaml",
				Enabled:    true,
				Sections: []config.SectionDefinition{
					{
						Name:     "name",
						YAMLPath: "name",
						RuleConfigs: []config.RuleConfig{
							{Name: "metadata_rule", Enabled: true},
						},
						AutoApprove: true,
					},
				},
			},
		},
	}, mockClient)
	manager.AddRule(&stubRule{name: "metadata_rule", decision: shared.Approve, reason: "metadata ok"})

	result := manager.EvaluateAll(&shared.MRContext{
		ProjectID: 123,
		MRIID:     456,
		Changes: []gitlab.FileChange{
			{
				NewPath: "product.yaml",
				Diff:    "@@ -1,1 +1,1 @@\n-name: oldname\n+name: newname\n",
			},
			{
				NewPath: "other/product.yaml",
				Diff:    "",
			},
		},
		MRInfo: &gitlab.MRInfo{
			Title:        "Mixed empty diff",
			Author:       "developer",
			SourceBranch: "feature/mixed-empty-diff",
			TargetBranch: "main",
		},
	})

	assert.Equal(t, shared.ManualReview, result.FinalDecision.Type)
	assert.Equal(t, 2, result.TotalFiles)

	productValidation := result.FileValidations["product.yaml"]
	assert.NotNil(t, productValidation)
	assert.Equal(t, shared.Approve, productValidation.FileDecision,
		"file with a real diff should still validate normally")

	emptyDiffValidation := result.FileValidations["other/product.yaml"]
	assert.NotNil(t, emptyDiffValidation)
	assert.Equal(t, shared.ManualReview, emptyDiffValidation.FileDecision,
		"listed file with an empty diff must not be auto-approved")
}

func TestEvaluateAll_NilMRInfoRequiresManualReview(t *testing.T) {
	manager := NewSectionRuleManager(&config.GlobalRuleConfig{
		Enabled: true,
		Files: []config.FileRuleConfig{
			{
				Name:       "product_configs",
				Path:       "**/",
				Filename:   "product.yaml",
				ParserType: "yaml",
				Enabled:    true,
				Sections: []config.SectionDefinition{
					{
						Name:        "name",
						YAMLPath:    "name",
						AutoApprove: true,
					},
				},
			},
		},
	}, nil)

	result := manager.EvaluateAll(&shared.MRContext{
		ProjectID: 123,
		MRIID:     456,
		Changes: []gitlab.FileChange{
			{
				NewPath: "product.yaml",
				Diff:    "@@ -1,1 +1,1 @@\n-name: oldname\n+name: newname\n",
			},
		},
		MRInfo: nil,
	})

	assert.Equal(t, shared.ManualReview, result.FinalDecision.Type)
	assert.Equal(t, 1, result.TotalFiles)
	fileValidation := result.FileValidations["product.yaml"]
	assert.NotNil(t, fileValidation)
	assert.Equal(t, shared.ManualReview, fileValidation.FileDecision)
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
		"",
		parser,
		changedLines,
		nil,
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
		"",
		parser,
		changedLines,
		nil,
		"+  version: v1.1.0\n+  owner: team",
	)

	assert.Equal(t, shared.Approve, result.FileDecision,
		"unaffected warehouses section (ManualReview) must not block approval")
}

func TestValidateFileWithSections_DeletedFieldInExistingSectionRequiresManualReview(t *testing.T) {
	manager := NewSectionRuleManager(&config.GlobalRuleConfig{Files: []config.FileRuleConfig{}}, nil)
	manager.AddRule(&stubRule{name: "access_policy_rule", decision: shared.Approve, reason: "unused"})

	parser := &stubSectionParser{
		sections: []shared.Section{
			{
				Name:      "data_product_db_validation",
				StartLine: 5,
				EndLine:   12,
				FilePath:  "product.yaml",
				RuleConfigs: []config.RuleConfig{
					{Name: "access_policy_rule", Enabled: true},
				},
			},
		},
		validateFn: func(section *shared.Section, rules []shared.Rule) *shared.SectionValidationResult {
			decision := shared.Approve
			reason := "No access_policy changes detected"
			if strings.Contains(section.Content, "access_policy:") && len(section.ChangedLines) > 0 {
				decision = shared.ManualReview
				reason = "access_policy changes require manual review"
			}
			return &shared.SectionValidationResult{
				Section:  section,
				Decision: decision,
				Reason:   reason,
				RuleResults: []shared.LineValidationResult{
					{
						RuleName:     "access_policy_rule",
						Decision:     decision,
						Reason:       reason,
						LineRanges:   section.ChangedLines,
						WasEvaluated: true,
					},
				},
			}
		},
	}

	sourceContent := "---\nname: analytics\nkind: source-aligned\nrover_group: example-analytics\ndata_product_db:\n  presentation_schemas:\n  - name: marts\n    consumers:\n    - name: existing_consumer\n      kind: data_product\n    - name: reporting\n      kind: data_product\n"
	targetContent := "---\nname: analytics\nkind: source-aligned\nrover_group: example-analytics\ndata_product_db:\n  presentation_schemas:\n  - name: marts\n    access_policy: rh_internal\n    consumers:\n    - name: existing_consumer\n      kind: data_product\n"

	result := manager.validateFileWithSections(
		"product.yaml",
		sourceContent,
		targetContent,
		parser,
		[]shared.LineRange{{StartLine: 11, EndLine: 12, FilePath: "product.yaml"}},
		[]shared.LineRange{{StartLine: 8, EndLine: 8, FilePath: "product.yaml"}},
		"-    access_policy: rh_internal\n+    - name: reporting",
	)

	assert.Equal(t, shared.ManualReview, result.FileDecision,
		"removing access_policy while adding a consumer in the same section must require manual review")
	found := false
	for _, rr := range result.RuleResults {
		if rr.RuleName == "access_policy_rule" && rr.Decision == shared.ManualReview {
			found = true
			break
		}
	}
	assert.True(t, found, "deleted-side access_policy_rule must contribute ManualReview")
}

func TestValidateFileWithSections_TargetParseFailureRequiresManualReview(t *testing.T) {
	manager := NewSectionRuleManager(&config.GlobalRuleConfig{Files: []config.FileRuleConfig{}}, nil)

	sourceContent := "name: newname\n"
	targetContent := "name: oldname\nnot: valid: yaml: ["

	parser := &stubSectionParser{
		parseFn: func(filePath string, content string) ([]shared.Section, error) {
			if content == targetContent {
				return nil, fmt.Errorf("invalid yaml")
			}
			return []shared.Section{
				{
					Name:      "name",
					StartLine: 1,
					EndLine:   1,
					FilePath:  filePath,
					Content:   content,
				},
			}, nil
		},
	}

	result := manager.validateFileWithSections(
		"product.yaml",
		sourceContent,
		targetContent,
		parser,
		[]shared.LineRange{{StartLine: 1, EndLine: 1, FilePath: "product.yaml"}},
		[]shared.LineRange{{StartLine: 1, EndLine: 1, FilePath: "product.yaml"}},
		"-name: oldname\n+name: newname",
	)

	assert.Equal(t, shared.ManualReview, result.FileDecision)
}

func TestValidateFileWithSections_UncoveredDeletedLinesRequireManualReview(t *testing.T) {
	manager := NewSectionRuleManager(&config.GlobalRuleConfig{Files: []config.FileRuleConfig{}}, nil)

	parser := &stubSectionParser{
		sections: []shared.Section{
			{
				Name:      "name",
				StartLine: 1,
				EndLine:   1,
				FilePath:  "product.yaml",
			},
		},
	}

	sourceContent := ""
	targetContent := "name: foo\norphan: bar\n"

	result := manager.validateFileWithSections(
		"product.yaml",
		sourceContent,
		targetContent,
		parser,
		nil,
		[]shared.LineRange{{StartLine: 2, EndLine: 2, FilePath: "product.yaml"}},
		"-orphan: bar",
	)

	assert.Equal(t, shared.ManualReview, result.FileDecision)
	assert.NotEmpty(t, result.UncoveredLines)
	found := false
	for _, u := range result.UncoveredLines {
		if u.StartLine <= 2 && u.EndLine >= 2 {
			found = true
			break
		}
	}
	assert.True(t, found, "deleted line 2 is outside any section and must be uncovered")
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


// --- Tests for Step 2: getDeletedLinesForFile and getOldFileContent ---

func TestGetChangedLinesForFile_ReturnsBothAddedAndDeleted(t *testing.T) {
	manager := NewSectionRuleManager(&config.GlobalRuleConfig{Files: []config.FileRuleConfig{}}, nil)

	mrCtx := &shared.MRContext{
		Changes: []gitlab.FileChange{
			{
				NewPath: "dataproducts/source/analytics/prod/product.yaml",
				OldPath: "dataproducts/source/analytics/prod/product.yaml",
				Diff:    "@@ -1,5 +1,3 @@\n name: testproduct\n kind: aggregated\n rover_group: test\n-  - consumer_one\n-  - consumer_two\n",
			},
		},
	}

	addedLines, deletedLines := manager.getChangedLinesForFile("dataproducts/source/analytics/prod/product.yaml", mrCtx)

	// Pure deletion: no added lines
	assert.Empty(t, addedLines)

	// Deleted lines at 4-5 in old file
	assert.Len(t, deletedLines, 1)
	assert.Equal(t, 4, deletedLines[0].StartLine)
	assert.Equal(t, 5, deletedLines[0].EndLine)
	assert.Equal(t, "dataproducts/source/analytics/prod/product.yaml", deletedLines[0].FilePath)
}

func TestGetChangedLinesForFile_PureAddition(t *testing.T) {
	manager := NewSectionRuleManager(&config.GlobalRuleConfig{Files: []config.FileRuleConfig{}}, nil)

	mrCtx := &shared.MRContext{
		Changes: []gitlab.FileChange{
			{
				NewPath: "dataproducts/source/analytics/prod/product.yaml",
				OldPath: "dataproducts/source/analytics/prod/product.yaml",
				Diff:    "@@ -1,3 +1,5 @@\n name: testproduct\n kind: aggregated\n rover_group: test\n+ai_ready: false\n+ai_experimental: true\n",
			},
		},
	}

	addedLines, deletedLines := manager.getChangedLinesForFile("dataproducts/source/analytics/prod/product.yaml", mrCtx)

	// Added lines at 4-5 in new file
	assert.Len(t, addedLines, 1)
	assert.Equal(t, 4, addedLines[0].StartLine)
	assert.Equal(t, 5, addedLines[0].EndLine)

	// No deleted lines
	assert.Empty(t, deletedLines)
}

func TestGetChangedLinesForFile_FileNotFound(t *testing.T) {
	manager := NewSectionRuleManager(&config.GlobalRuleConfig{Files: []config.FileRuleConfig{}}, nil)

	mrCtx := &shared.MRContext{
		Changes: []gitlab.FileChange{
			{
				NewPath: "dataproducts/source/other/prod/product.yaml",
				Diff:    "@@ -1,3 +1,3 @@\n name: test\n-old_line\n+new_line\n",
			},
		},
	}

	addedLines, deletedLines := manager.getChangedLinesForFile("dataproducts/source/analytics/prod/product.yaml", mrCtx)

	assert.Empty(t, addedLines)
	assert.Empty(t, deletedLines)
}

func TestFetchFileFromBranch_Success(t *testing.T) {
	oldContent := "name: testproduct\nkind: aggregated\nrover_group: test\n  - consumer_one\n  - consumer_two\n"

	mockClient := &forkMRTestGitLabClient{
		targetProjectID: 100,
		sourceProjectID: 200,
		targetBranch:    "main",
		sourceBranch:    "feature",
		beforeYAML:      oldContent,
		afterYAML:       "name: testproduct\nkind: aggregated\nrover_group: test\n",
	}

	manager := NewSectionRuleManager(&config.GlobalRuleConfig{Files: []config.FileRuleConfig{}}, mockClient)

	content, err := manager.fetchFileFromBranch(100, "dataproducts/source/analytics/prod/product.yaml", "main")

	assert.NoError(t, err)
	assert.Equal(t, oldContent, content)
}

func TestFetchFileFromBranch_EmptyBranch(t *testing.T) {
	mockClient := &forkMRTestGitLabClient{
		targetProjectID: 100,
		sourceProjectID: 100,
		targetBranch:    "main",
		sourceBranch:    "feature",
	}
	manager := NewSectionRuleManager(&config.GlobalRuleConfig{Files: []config.FileRuleConfig{}}, mockClient)

	_, err := manager.fetchFileFromBranch(100, "product.yaml", "")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "branch not specified")
}

func TestFetchFileFromBranch_NoGitLabClient(t *testing.T) {
	manager := NewSectionRuleManager(&config.GlobalRuleConfig{Files: []config.FileRuleConfig{}}, nil)

	_, err := manager.fetchFileFromBranch(100, "product.yaml", "main")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "GitLab client not available")
}

func TestAffectedSectionsForDeletedLines(t *testing.T) {
	manager := NewSectionRuleManager(&config.GlobalRuleConfig{Files: []config.FileRuleConfig{}}, nil)

	// Old file has sections: name(1-1), kind(2-2), rover_group(3-3), consumers(4-6)
	oldSections := []shared.Section{
		{Name: "metadata", StartLine: 1, EndLine: 3},
		{Name: "consumers", StartLine: 4, EndLine: 6},
	}

	// Deleted lines at 4-5 in old file (consumer entries removed)
	deletedLines := []shared.LineRange{
		{StartLine: 4, EndLine: 5, FilePath: "product.yaml"},
	}

	affected := manager.getAffectedSections(oldSections, deletedLines)

	assert.Len(t, affected, 1)
	assert.Equal(t, "consumers", affected[0].Name)
}

func TestIsIgnoredFile(t *testing.T) {
	ruleConfig := &config.GlobalRuleConfig{
		Files: []config.FileRuleConfig{},
		IgnoreFiles: []config.IgnoreFileConfig{
			{Name: "sql_migrations", Path: "**/", Filename: "*.sql"},
			{Name: "ci_config", Path: "", Filename: ".gitlab-ci.yml"},
			{Name: "gitkeep", Path: "**/", Filename: ".gitkeep"},
		},
	}

	manager := NewSectionRuleManager(ruleConfig, nil)

	tests := []struct {
		name     string
		filePath string
		expected bool
	}{
		{"sql file at root", "migration.sql", true},
		{"sql file nested", "dataproducts/source/analytics/migration.sql", true},
		{"gitlab-ci at root", ".gitlab-ci.yml", true},
		{"gitkeep nested", "dataproducts/source/.gitkeep", true},
		{"gitkeep at root", ".gitkeep", true},
		{"yaml file not ignored", "dataproducts/source/product.yaml", false},
		{"markdown not ignored", "README.md", false},
		{"python not ignored", "scripts/process.py", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := manager.isIgnoredFile(tt.filePath)
			assert.Equal(t, tt.expected, result, "isIgnoredFile(%q)", tt.filePath)
		})
	}
}

func TestDetermineOverallDecision_AllIgnored(t *testing.T) {
	ruleConfig := &config.GlobalRuleConfig{
		Files: []config.FileRuleConfig{},
	}

	manager := NewSectionRuleManager(ruleConfig, nil)

	// When all files are ignored and no file validations exist, decision is CommentOnly
	emptyValidations := make(map[string]*shared.FileValidationSummary)
	ignoredFiles := []string{"migration.sql", ".gitlab-ci.yml"}
	decision := manager.determineOverallDecision(emptyValidations, ignoredFiles)

	assert.Equal(t, shared.CommentOnly, decision.Type)
	assert.Contains(t, decision.Reason, "ignore list")
	assert.Contains(t, decision.Summary, "ignored")
}

func TestDetermineOverallDecision_SomeIgnoredSomeValidated(t *testing.T) {
	ruleConfig := &config.GlobalRuleConfig{
		Files: []config.FileRuleConfig{},
	}

	manager := NewSectionRuleManager(ruleConfig, nil)

	// When some files are validated (approved) and some ignored, decision is based on validated files
	validations := map[string]*shared.FileValidationSummary{
		"product.yaml": {
			FilePath:     "product.yaml",
			FileDecision: shared.Approve,
		},
	}
	ignoredFiles := []string{"migration.sql"}
	decision := manager.determineOverallDecision(validations, ignoredFiles)

	assert.Equal(t, shared.Approve, decision.Type)
}

func TestIgnorePatternInitialization(t *testing.T) {
	ruleConfig := &config.GlobalRuleConfig{
		Files: []config.FileRuleConfig{},
		IgnoreFiles: []config.IgnoreFileConfig{
			{Name: "sql_migrations", Path: "**/", Filename: "*.sql"},
			{Name: "ci_config", Path: "", Filename: ".gitlab-ci.yml"},
		},
	}

	manager := NewSectionRuleManager(ruleConfig, nil)

	assert.Equal(t, 2, len(manager.ignorePatterns))
	assert.Contains(t, manager.ignorePatterns, "**/*.sql")
	assert.Contains(t, manager.ignorePatterns, ".gitlab-ci.yml")
}
