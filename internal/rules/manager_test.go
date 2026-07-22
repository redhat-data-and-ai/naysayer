package rules

import (
	"testing"

	"github.com/redhat-data-and-ai/naysayer/internal/config"
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
	changedLines := manager.extractChangedLinesFromDiff(diff)
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
	result := manager.validateFileWithSections("CODEOWNERS", codeownersContent, 3, parser, changedLines, diff)
	assert.NotNil(t, result)
	assert.Equal(t, shared.ManualReview, result.FileDecision,
		"codeowners_sync_rule not registered → fallback manual review expected")
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

// Verify that a CODEOWNERS file change is correctly picked up: the modified line
// must fall inside the codeowners section so the codeowners_sync_rule is triggered.
func TestExtractChangedLinesFromDiff_CodeownersChangeDetected(t *testing.T) {
	manager := NewSectionRuleManager(&config.GlobalRuleConfig{Files: []config.FileRuleConfig{}}, nil)

	// Typical CODEOWNERS diff: adding a new owner to an existing line
	// Before: /dataproducts/aggregate/analytics/ @alice @bob
	// After:  /dataproducts/aggregate/analytics/ @alice @bob @charlie
	diff := "@@ -1,3 +1,3 @@\n # Data Product Owners\n [Aggregate Data Products]\n-/dataproducts/aggregate/analytics/ @alice @bob\n+/dataproducts/aggregate/analytics/ @alice @bob @charlie\n"

	changedLines := manager.extractChangedLinesFromDiff(diff)

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

	changedLines := manager.extractChangedLinesFromDiff(diff)

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
