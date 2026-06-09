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
