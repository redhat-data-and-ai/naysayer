package rules

import (
	"testing"

	"github.com/redhat-data-and-ai/naysayer/internal/gitlab"
	"github.com/redhat-data-and-ai/naysayer/internal/rules/shared"
)

// TestDocumentationAutoApprovalRule_Basic tests basic documentation rule functionality
func TestDocumentationAutoApprovalRule_Basic(t *testing.T) {
	rule := NewDocumentationAutoApprovalRule()

	tests := []struct {
		name         string
		filePath     string
		shouldApprove bool
	}{
		{"README.md file", "README.md", true},
		{"data_elements.md file", "dataproducts/aggregate/test/data_elements.md", true},
		{"developers.yaml file", "dataproducts/aggregate/test/developers.yaml", true},
		{"promotion_checklist.md file", "dataproducts/source/test/promotion_checklist.md", true},
		{"Non-documentation file", "dataproducts/aggregate/test/prod/product.yaml", false},
		{"Service account file", "serviceaccounts/prod/test_appuser.yaml", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			decision, _ := rule.ValidateLines(tt.filePath, "test content", []shared.LineRange{})
			
			expectedDecision := shared.ManualReview
			if tt.shouldApprove {
				expectedDecision = shared.Approve
			}
			
			if decision != expectedDecision {
				t.Errorf("Expected decision %v for %s, got %v", expectedDecision, tt.name, decision)
			}
		})
	}
}

// TestServiceAccountCommentRule_Basic tests basic service account rule functionality  
func TestServiceAccountCommentRule_Basic(t *testing.T) {
	rule := NewServiceAccountCommentRule()

	tests := []struct {
		name     string
		filePath string
		isServiceAccount bool
	}{
		{"Service account file", "serviceaccounts/prod/test_astro_prod_appuser.yaml", true},
		{"Service account yml file", "serviceaccounts/dev/marketo_workato_dev_appuser.yml", true},
		{"Non-service account file", "dataproducts/aggregate/test/prod/product.yaml", false},
		{"Wrong service account pattern", "serviceaccounts/prod/some_other_file.yaml", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			decision, _ := rule.ValidateLines(tt.filePath, "test content", []shared.LineRange{})
			
			// Currently always requires manual review since comment detection is not implemented
			if decision != shared.ManualReview {
				t.Errorf("Expected ManualReview for %s, got %v", tt.name, decision)
			}
		})
	}
}

// TestAutoApprovalRulesRegistered verifies auto-approval rules are registered
func TestAutoApprovalRulesRegistered(t *testing.T) {
	registry := NewRuleRegistry()

	// Only documentation rule is enabled in production
	expectedEnabledRules := []string{
		"documentation_auto_approval",
	}

	// Service account rule is registered but disabled
	expectedRegisteredRules := []string{
		"documentation_auto_approval",
		"service_account_comment_rule",
	}

	enabledRules := registry.ListEnabledRules()
	allRules := registry.ListRules()
	
	for _, expectedRule := range expectedEnabledRules {
		if _, exists := enabledRules[expectedRule]; !exists {
			t.Errorf("Expected rule '%s' to be enabled", expectedRule)
		}
	}

	for _, expectedRule := range expectedRegisteredRules {
		if _, exists := allRules[expectedRule]; !exists {
			t.Errorf("Expected rule '%s' to be registered", expectedRule)
		}
	}
}

// TestDocumentationAutoApprovalInManager tests documentation auto-approval in rule manager
func TestDocumentationAutoApprovalInManager(t *testing.T) {
	manager := NewSimpleRuleManager()
	manager.AddRule(NewDocumentationAutoApprovalRule())

	mrCtx := &shared.MRContext{
		ProjectID: 123,
		MRIID:     456,
		Changes: []gitlab.FileChange{
			{NewPath: "README.md", NewFile: false, DeletedFile: false},
		},
		MRInfo: &gitlab.MRInfo{
			Title:  "Update documentation",
			Author: "test-user",
		},
		Environment: "test",
	}

	result := manager.EvaluateAll(mrCtx)

	if result.FinalDecision.Type != shared.Approve {
		t.Errorf("Expected approve decision for README.md, got %v", result.FinalDecision.Type)
		t.Errorf("Reason: %s", result.FinalDecision.Reason)
	}

	if result.ExecutionTime <= 0 {
		t.Error("Expected positive execution time")
	}
}

// TestProductionDataverseManager tests auto-approval rules in production rule manager
func TestProductionDataverseManager(t *testing.T) {
	mockClient := &gitlab.Client{}
	manager := CreateDataverseRuleManager(mockClient)

	// Test documentation file (won't trigger warehouse rule HTTP calls)
	mrCtx := &shared.MRContext{
		ProjectID: 123,
		MRIID:     456,
		Changes: []gitlab.FileChange{
			{NewPath: "README.md", NewFile: false, DeletedFile: false},
		},
		MRInfo: &gitlab.MRInfo{
			Title:  "Update project documentation",
			Author: "developer",
		},
		Environment: "prod",
	}

	result := manager.EvaluateAll(mrCtx)

	// Should be auto-approved by documentation rule
	if result.FinalDecision.Type != shared.Approve {
		t.Errorf("Expected README.md to be auto-approved, got %v", result.FinalDecision.Type)
		t.Errorf("Reason: %s", result.FinalDecision.Reason)
	}

	if result.ExecutionTime <= 0 {
		t.Error("Expected positive execution time indicating rules were processed")
	}
}

// TestAutoApprovalRulesCategory verifies auto-approval rules are categorized correctly
func TestAutoApprovalRulesCategory(t *testing.T) {
	registry := NewRuleRegistry()

	autoApprovalRules := registry.ListRulesByCategory("auto_approval")

	expectedCount := 2 // documentation_auto_approval + service_account_comment_rule (disabled)
	if len(autoApprovalRules) != expectedCount {
		t.Errorf("Expected %d auto-approval rules, got %d", expectedCount, len(autoApprovalRules))
	}

	expectedNames := map[string]bool{
		"documentation_auto_approval":   true,
		"service_account_comment_rule": true,
	}

	for name := range autoApprovalRules {
		if !expectedNames[name] {
			t.Errorf("Unexpected rule '%s' in auto_approval category", name)
		}
		delete(expectedNames, name)
	}

	if len(expectedNames) > 0 {
		for missingRule := range expectedNames {
			t.Errorf("Expected rule '%s' not found in auto_approval category", missingRule)
		}
	}
}

// TestMixedFileTypes tests behavior with multiple file types
func TestMixedFileTypes(t *testing.T) {
	manager := NewSimpleRuleManager()
	manager.AddRule(NewDocumentationAutoApprovalRule())
	manager.AddRule(NewServiceAccountCommentRule())

	testCases := []struct {
		name           string
		changes        []gitlab.FileChange
		expectedResult shared.DecisionType
		description    string
	}{
		{
			name: "Multiple documentation files",
			changes: []gitlab.FileChange{
				{NewPath: "README.md", NewFile: false, DeletedFile: false},
				{NewPath: "dataproducts/source/test/promotion_checklist.md", NewFile: false, DeletedFile: false},
			},
			expectedResult: shared.Approve,
			description:    "All documentation files should be approved",
		},
		{
			name: "Documentation and non-covered files",
			changes: []gitlab.FileChange{
				{NewPath: "README.md", NewFile: false, DeletedFile: false},
				{NewPath: "src/main.go", NewFile: false, DeletedFile: false},
			},
			expectedResult: shared.ManualReview,
			description:    "Mix of covered and uncovered files should require manual review",
		},
		{
			name: "Non-covered files only",
			changes: []gitlab.FileChange{
				{NewPath: "src/main.go", NewFile: false, DeletedFile: false},
			},
			expectedResult: shared.ManualReview,
			description:    "Files not covered by any rule should require manual review",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mrCtx := &shared.MRContext{
				ProjectID: 123,
				MRIID:     456,
				Changes:   tc.changes,
				MRInfo: &gitlab.MRInfo{
					Title:  "Test MR",
					Author: "test-user",
				},
				Environment: "test",
			}

			result := manager.EvaluateAll(mrCtx)

			if result.FinalDecision.Type != tc.expectedResult {
				t.Errorf("Expected decision %v for %s, got %v", 
					tc.expectedResult, tc.name, result.FinalDecision.Type)
				t.Errorf("Reason: %s", result.FinalDecision.Reason)
			}
		})
	}
}