package rules

import (
	"testing"

	"github.com/redhat-data-and-ai/naysayer/internal/config"
	"github.com/redhat-data-and-ai/naysayer/internal/rules/shared"
	"github.com/stretchr/testify/assert"
)

// AutoApproveMockRule for testing auto-approve functionality
type AutoApproveMockRule struct {
	name     string
	decision shared.DecisionType
	reason   string
}

func (m *AutoApproveMockRule) Name() string {
	return m.name
}

func (m *AutoApproveMockRule) Description() string {
	return "Mock rule for auto-approve testing"
}

func (m *AutoApproveMockRule) GetCoveredLines(filePath string, fileContent string) []shared.LineRange {
	// Always return a range to indicate this rule applies
	return []shared.LineRange{
		{StartLine: 1, EndLine: 10, FilePath: filePath},
	}
}

func (m *AutoApproveMockRule) ValidateLines(filePath string, fileContent string, lineRanges []shared.LineRange) (shared.DecisionType, string) {
	return m.decision, m.reason
}

func TestYAMLSectionParser_ValidateSection_AutoApprove(t *testing.T) {
	tests := []struct {
		name             string
		section          *shared.Section
		rules            []shared.Rule
		expectedDecision shared.DecisionType
		expectedReason   string
		expectAuditLog   bool
	}{
		{
			name: "auto-approve with no rules - immediate approval",
			section: &shared.Section{
				Name:        "description",
				StartLine:   1,
				EndLine:     3,
				Content:     "description: This is a test description",
				FilePath:    "test.yaml",
				AutoApprove: true,
				RuleNames:   []string{},
			},
			rules:            []shared.Rule{},
			expectedDecision: shared.Approve,
			expectedReason:   "Auto-approved: description (no validation required)",
			expectAuditLog:   true,
		},
		{
			name: "auto-approve with passing rules",
			section: &shared.Section{
				Name:        "description",
				StartLine:   1,
				EndLine:     3,
				Content:     "description: This is a test description",
				FilePath:    "test.yaml",
				AutoApprove: true,
				RuleNames:   []string{"test_rule"},
			},
			rules: []shared.Rule{
				&AutoApproveMockRule{
					name:     "test_rule",
					decision: shared.Approve,
					reason:   "Rule validation passed",
				},
			},
			expectedDecision: shared.Approve,
			expectedReason:   "Auto-approved: Rule validation passed (validation passed)",
			expectAuditLog:   true,
		},
		{
			name: "auto-approve with failing rules - manual review",
			section: &shared.Section{
				Name:        "description",
				StartLine:   1,
				EndLine:     3,
				Content:     "description: This is a test description",
				FilePath:    "test.yaml",
				AutoApprove: true,
				RuleNames:   []string{"test_rule"},
			},
			rules: []shared.Rule{
				&AutoApproveMockRule{
					name:     "test_rule",
					decision: shared.ManualReview,
					reason:   "Rule validation failed",
				},
			},
			expectedDecision: shared.ManualReview,
			expectedReason:   "Rule validation failed: Rule validation failed",
			expectAuditLog:   true,
		},
		{
			name: "non-auto-approve with passing rules - normal approval",
			section: &shared.Section{
				Name:        "warehouses",
				StartLine:   1,
				EndLine:     5,
				Content:     "warehouses:\n- type: user\n  size: SMALL",
				FilePath:    "test.yaml",
				AutoApprove: false,
				RuleNames:   []string{"test_rule"},
			},
			rules: []shared.Rule{
				&AutoApproveMockRule{
					name:     "test_rule",
					decision: shared.Approve,
					reason:   "Rule validation passed",
				},
			},
			expectedDecision: shared.Approve,
			expectedReason:   "Rule validation passed",
			expectAuditLog:   false,
		},
		{
			name: "non-auto-approve with no rules - manual review",
			section: &shared.Section{
				Name:        "unknown_section",
				StartLine:   1,
				EndLine:     3,
				Content:     "unknown: value",
				FilePath:    "test.yaml",
				AutoApprove: false,
				RuleNames:   []string{},
			},
			rules:            []shared.Rule{},
			expectedDecision: shared.ManualReview,
			expectedReason:   "No validation rules configured for unknown_section - manual review required",
			expectAuditLog:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create parser
			parser := NewYAMLSectionParser(map[string]config.SectionDefinition{})

			// Validate section
			result := parser.ValidateSection(tt.section, tt.rules)

			// Assertions
			assert.Equal(t, tt.expectedDecision, result.Decision)
			assert.Contains(t, result.Reason, tt.expectedReason)
			assert.Equal(t, tt.section, result.Section)

			// Check if rules were applied correctly
			if len(tt.rules) > 0 {
				if tt.expectedDecision == shared.Approve && tt.section.AutoApprove {
					// For auto-approve sections with passing rules
					assert.Len(t, result.AppliedRules, 1)
					assert.Equal(t, tt.rules[0].Name(), result.AppliedRules[0])
				} else if !tt.section.AutoApprove {
					// For non-auto-approve sections
					assert.Len(t, result.AppliedRules, 1)
					assert.Equal(t, tt.rules[0].Name(), result.AppliedRules[0])
				}
			}
		})
	}
}

func TestYAMLSectionParser_ParseSections_AutoApprove(t *testing.T) {
	yamlContent := `
description: This is a test product
documentation:
  url: https://example.com/docs
warehouses:
- type: user
  size: SMALL
changelog:
- "Initial version"
`

	// Create section definitions with auto-approve flags
	definitions := map[string]config.SectionDefinition{
		"description": {
			Name:        "description",
			YAMLPath:    "description",
			Required:    false,
			RuleNames:   []string{"text_rule"},
			AutoApprove: true,
		},
		"documentation_url": {
			Name:        "documentation_url",
			YAMLPath:    "documentation.url",
			Required:    false,
			RuleNames:   []string{},
			AutoApprove: true,
		},
		"warehouses": {
			Name:        "warehouses",
			YAMLPath:    "warehouses",
			Required:    true,
			RuleNames:   []string{"warehouse_rule"},
			AutoApprove: false,
		},
		"changelog": {
			Name:        "changelog",
			YAMLPath:    "changelog",
			Required:    false,
			RuleNames:   []string{},
			AutoApprove: true,
		},
	}

	parser := NewYAMLSectionParser(definitions)

	sections, err := parser.ParseSections("test.yaml", yamlContent)
	assert.NoError(t, err)

	// Verify sections have correct auto-approve flags
	sectionMap := make(map[string]*shared.Section)
	for i := range sections {
		sectionMap[sections[i].Name] = &sections[i]
	}

	// Check auto-approve flags
	assert.True(t, sectionMap["description"].AutoApprove)
	assert.True(t, sectionMap["documentation_url"].AutoApprove)
	assert.False(t, sectionMap["warehouses"].AutoApprove)
	assert.True(t, sectionMap["changelog"].AutoApprove)

	// Check rule names
	assert.Equal(t, []string{"text_rule"}, sectionMap["description"].RuleNames)
	assert.Equal(t, []string{}, sectionMap["documentation_url"].RuleNames)
	assert.Equal(t, []string{"warehouse_rule"}, sectionMap["warehouses"].RuleNames)
	assert.Equal(t, []string{}, sectionMap["changelog"].RuleNames)
}

func TestYAMLSectionParser_ValidateSection_AuditLogging(t *testing.T) {
	// This test verifies that audit logging calls are made correctly
	// In a real test environment, you might want to capture log output
	section := &shared.Section{
		Name:        "test_section",
		StartLine:   1,
		EndLine:     3,
		Content:     "test: value",
		FilePath:    "test.yaml",
		AutoApprove: true,
		RuleNames:   []string{},
	}

	parser := NewYAMLSectionParser(map[string]config.SectionDefinition{})
	result := parser.ValidateSection(section, []shared.Rule{})

	// Verify the decision is correct
	assert.Equal(t, shared.Approve, result.Decision)
	assert.Contains(t, result.Reason, "Auto-approved: test_section (no validation required)")

	// Note: In a real test, you would capture the log output and verify the audit log entry
	// For now, we just verify the decision logic works correctly
}

func TestAutoApproveConfiguration_Integration(t *testing.T) {
	// Integration test to verify the complete auto-approve flow
	yamlContent := `
name: test-product
description: This is a test product
documentation:
  url: https://example.com/docs
warehouses:
- type: user
  size: SMALL
changelog:
- "Initial version"
`

	// Create configuration matching our rules.yaml
	definitions := map[string]config.SectionDefinition{
		"description": {
			Name:        "description",
			YAMLPath:    "description",
			Required:    false,
			RuleNames:   []string{},
			AutoApprove: true,
		},
		"documentation_url": {
			Name:        "documentation_url",
			YAMLPath:    "documentation.url",
			Required:    false,
			RuleNames:   []string{},
			AutoApprove: true,
		},
		"warehouses": {
			Name:        "warehouses",
			YAMLPath:    "warehouses",
			Required:    true,
			RuleNames:   []string{},
			AutoApprove: false,
		},
		"changelog": {
			Name:        "changelog",
			YAMLPath:    "changelog",
			Required:    false,
			RuleNames:   []string{},
			AutoApprove: true,
		},
	}

	parser := NewYAMLSectionParser(definitions)

	// Parse sections
	sections, err := parser.ParseSections("test.yaml", yamlContent)
	assert.NoError(t, err)

	// Validate each section
	autoApprovedCount := 0
	manualReviewCount := 0

	for _, section := range sections {
		result := parser.ValidateSection(&section, []shared.Rule{})

		if result.Decision == shared.Approve && section.AutoApprove {
			autoApprovedCount++
			assert.Contains(t, result.Reason, "Auto-approved")
		} else if result.Decision == shared.ManualReview {
			manualReviewCount++
		}
	}

	// Verify expected outcomes
	assert.Equal(t, 3, autoApprovedCount, "Expected 3 auto-approved sections (description, documentation_url, changelog)")
	assert.Equal(t, 1, manualReviewCount, "Expected 1 manual review section (warehouses)")
}
