package serviceaccount

import (
	"testing"

	"github.com/redhat-data-and-ai/naysayer/internal/gitlab"
	"github.com/redhat-data-and-ai/naysayer/internal/rules/shared"
	"github.com/stretchr/testify/assert"
)

// IntegrationTest tests the complete service account rule workflow with key scenarios
func TestServiceAccountRule_CompleteWorkflow(t *testing.T) {
	tests := []struct {
		name             string
		files            map[string]string
		expectedDecision shared.DecisionType
		expectedInReason []string
	}{
		{
			name: "valid_service_account_auto_approve",
			files: map[string]string{
				"serviceaccounts/prod/marketo_tableau_prod_appuser.yaml": `---
name: marketo_tableau_prod_appuser
comment: "service account for marketo data"
email: john.doe@redhat.com
`,
			},
			expectedDecision: shared.Approve,
			expectedInReason: []string{"Service account validation passed"},
		},
		{
			name: "group_email_manual_review",
			files: map[string]string{
				"serviceaccounts/dev/marketo_tableau_dev_appuser.yaml": `---
name: marketo_tableau_dev_appuser
comment: "service account for marketo data"
email: dataverse-platform-team@redhat.com
`,
			},
			expectedDecision: shared.ManualReview,
			expectedInReason: []string{"Group email addresses are not allowed"},
		},
		{
			name: "astro_in_dev_manual_review",
			files: map[string]string{
				"serviceaccounts/dev/marketo_astro_dev_appuser.yaml": `---
name: marketo_astro_dev_appuser
comment: "service account for marketo astro"
email: john.doe@redhat.com
`,
			},
			expectedDecision: shared.ManualReview,
			expectedInReason: []string{"Astro service accounts are only allowed in preprod and prod"},
		},
		{
			name: "invalid_naming_manual_review",
			files: map[string]string{
				"serviceaccounts/prod/Invalid-Service-Account_appuser.yaml": `---
name: Invalid-Service-Account
comment: "service account"
email: john.doe@redhat.com
`,
			},
			expectedDecision: shared.ManualReview,
			expectedInReason: []string{"naming convention"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := &MockGitLabClient{fileContents: tt.files}
			var changes []gitlab.FileChange
			for filePath := range tt.files {
				changes = append(changes, gitlab.FileChange{NewPath: filePath})
			}

			rule := NewRule(mockClient)
			mrCtx := &shared.MRContext{ProjectID: 123, MRIID: 456, Changes: changes}
			decision, reason := rule.ShouldApprove(mrCtx)

			assert.Equal(t, tt.expectedDecision, decision)
			for _, expectedString := range tt.expectedInReason {
				assert.Contains(t, reason, expectedString)
			}
		})
	}
}

func TestServiceAccountRule_FilePatternDetection(t *testing.T) {
	rule := NewRule(&MockGitLabClient{})

	// Test valid patterns
	validPaths := []string{
		"serviceaccounts/dev/marketo_astro_dev_appuser.yaml",
		"serviceaccounts/prod/dataverse_operator_prod_appuser.yml",
	}
	for _, path := range validPaths {
		assert.True(t, rule.isServiceAccountFile(path), "Should match: %s", path)
	}

	// Test invalid patterns
	invalidPaths := []string{
		"serviceaccounts/dev/marketo_astro_dev.yaml", // missing _appuser
		"dataproducts/agg/test/product.yaml",           // wrong directory
		"serviceaccounts/README.md",                    // not YAML
	}
	for _, path := range invalidPaths {
		assert.False(t, rule.isServiceAccountFile(path), "Should not match: %s", path)
	}
}

func TestServiceAccountRule_PathParsing(t *testing.T) {
	rule := NewRule(&MockGitLabClient{})

	// Test standard path parsing
	result := rule.parseServiceAccountFile("serviceaccounts/dev/marketo_astro_dev_appuser.yaml")
	expected := ServiceAccountFile{
		Path: "serviceaccounts/dev/marketo_astro_dev_appuser.yaml",
		Environment: "dev", DataProduct: "marketo", Integration: "astro", FileType: "appuser",
	}
	assert.Equal(t, expected, result)

	// Test prod environment
	result = rule.parseServiceAccountFile("serviceaccounts/prod/dataverse_operator_prod_appuser.yaml")
	assert.Equal(t, "prod", result.Environment)
	assert.Equal(t, "dataverse", result.DataProduct)
	assert.Equal(t, "operator", result.Integration)

	// Test fallback for invalid paths
	result = rule.parseServiceAccountFile("invalid/path/structure.yaml")
	assert.Equal(t, "unknown", result.Environment)
	assert.Equal(t, "unknown", result.DataProduct)
}


func TestServiceAccountRule_EdgeCases(t *testing.T) {
	// Test file fetch error
	mockClient := &MockGitLabClient{returnError: true}
	rule := NewRule(mockClient)
	mrCtx := &shared.MRContext{
		Changes: []gitlab.FileChange{{NewPath: "serviceaccounts/dev/test_service_dev_appuser.yaml"}},
	}
	decision, _ := rule.ShouldApprove(mrCtx)
	assert.Equal(t, shared.ManualReview, decision, "File fetch error should require manual review")

	// Test no service account files
	mockClient = &MockGitLabClient{}
	rule = NewRule(mockClient)
	mrCtx = &shared.MRContext{
		Changes: []gitlab.FileChange{{NewPath: "dataproducts/agg/test/product.yaml"}},
	}
	decision, _ = rule.ShouldApprove(mrCtx)
	assert.Equal(t, shared.Approve, decision, "No service account files should auto-approve")
}