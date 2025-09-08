package rules

import (
	"testing"

	"github.com/redhat-data-and-ai/naysayer/internal/gitlab"
	"github.com/redhat-data-and-ai/naysayer/internal/rules/shared"
	"github.com/stretchr/testify/assert"
)

func TestServiceAccountRule_Name(t *testing.T) {
	rule := NewServiceAccountRule(nil)
	assert.Equal(t, "service_account_rule", rule.Name())
}

func TestServiceAccountRule_Description(t *testing.T) {
	rule := NewServiceAccountRule(nil)
	description := rule.Description()
	assert.Contains(t, description, "Astro service account files")
	assert.Contains(t, description, "_astro_")
	assert.Contains(t, description, "_appuser")
	assert.Contains(t, description, "manual review")
}

func TestServiceAccountRule_isServiceAccountFile(t *testing.T) {
	rule := NewServiceAccountRule(nil)

	tests := []struct {
		name     string
		path     string
		expected bool
	}{
		// Astro service account files (must follow **_astro_<env>_appuser pattern)
		{"astro service account dev", "dataproducts/analytics/sa_astro_dev_appuser.yaml", true},
		{"astro service account prod", "path/to/my_astro_prod_appuser.yml", true},
		{"astro uppercase", "SA_ASTRO_STAGING_APPUSER.YAML", true},
		
		// Generic service account files
		{"serviceaccount file", "configs/myserviceaccount.yaml", true},
		{"service-account file", "configs/my-service-account.yml", true},
		{"serviceaccounts directory", "serviceaccounts/user-sa.yaml", true},
		{"nested serviceaccounts", "dataproducts/prod/serviceaccounts/app.yml", true},
		
		// Invalid Astro patterns
		{"astro without appuser", "sa_astro_dev.yaml", true}, // Still recognized as service account, but won't be Astro type
		{"astro wrong order", "sa_appuser_astro_dev.yaml", true}, // Still recognized as service account
		{"astro missing env", "sa_astro__appuser.yaml", true}, // Still recognized as service account
		
		// Non-service account files
		{"regular yaml", "config.yaml", false},
		{"readme", "README.md", false},
		{"product file", "product.yaml", false},
		{"developers file", "developers.yaml", false},
		{"empty path", "", false},
		{"non-yaml astro", "test_astro_file.txt", false},
		{"non-yaml serviceaccount", "serviceaccount.txt", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := rule.isServiceAccountFile(tt.path)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestServiceAccountRule_getServiceAccountType(t *testing.T) {
	rule := NewServiceAccountRule(nil)

	tests := []struct {
		name         string
		path         string
		expectedType string
	}{
		{"astro type valid", "dataproducts/analytics/sa_astro_dev_appuser.yaml", "astro"},
		{"astro type prod", "path/to/my_astro_prod_appuser.yml", "astro"},
		{"astro uppercase", "SA_ASTRO_STAGING_APPUSER.YAML", "astro"},
		{"astro invalid - no appuser suffix", "sa_astro_dev.yaml", "generic"},
		{"astro invalid - wrong order", "sa_appuser_astro_dev.yaml", "appuser"},
		{"appuser type", "serviceaccounts/user_appuser.yaml", "appuser"},
		{"appuser uppercase", "USER_APPUSER.YAML", "appuser"},
		{"serviceaccounts directory", "serviceaccounts/generic-sa.yaml", "generic"},
		{"generic serviceaccount", "configs/myserviceaccount.yaml", "generic"},
		{"generic service-account", "configs/my-service-account.yml", "generic"},
		{"empty path", "", "unknown"},
		{"unknown type", "some/random/file.yaml", "generic"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := rule.getServiceAccountType(tt.path)
			assert.Equal(t, tt.expectedType, result)
		})
	}
}

func TestServiceAccountRule_GetCoveredLines(t *testing.T) {
	rule := NewServiceAccountRule(nil)

	tests := []struct {
		name        string
		filePath    string
		fileContent string
		expectCover bool
	}{
		{"astro service account file", "dataproducts/analytics/sa_astro_analytics.yaml", "name: sa_astro_analytics\nmetadata:\n  name: test\n", true},
		{"generic service account file", "serviceaccounts/user-sa.yaml", "metadata:\n  name: user-sa\n", true},
		{"service account with minimal content", "myserviceaccount.yaml", "name: test", true},
		{"non-service account file", "README.md", "# README\nThis is a readme file\n", false},
		{"service account file with empty content", "sa_astro_test.yaml", "", false},
		{"service account file with whitespace only", "serviceaccount.yaml", "   \n  \t  \n", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lines := rule.GetCoveredLines(tt.filePath, tt.fileContent)
			if tt.expectCover {
				assert.Len(t, lines, 1, "Should return exactly one line range for service account files")
				assert.Equal(t, tt.filePath, lines[0].FilePath)
				assert.Equal(t, 1, lines[0].StartLine)
				expectedLines := shared.CountLines(tt.fileContent)
				assert.Equal(t, expectedLines, lines[0].EndLine)
			} else {
				assert.Len(t, lines, 0, "Should not cover lines for non-service account files or empty files")
			}
		})
	}
}

func TestServiceAccountRule_validateAstroServiceAccount(t *testing.T) {
	rule := NewServiceAccountRule(nil)

	tests := []struct {
		name             string
		filePath         string
		fileContent      string
		expectedResult   shared.DecisionType
		expectedReason   string
	}{
		{
			name:     "valid astro service account",
			filePath: "dataproducts/analytics/sa_astro_dev_appuser.yaml",
			fileContent: `name: sa_astro_dev_appuser
metadata:
  name: sa_astro_dev_appuser
  namespace: analytics`,
			expectedResult: shared.Approve,
			expectedReason: "Astro service account file follows naming convention and name field matches filename",
		},
		{
			name:     "astro service account with mismatched name",
			filePath: "dataproducts/analytics/sa_astro_prod_appuser.yaml",
			fileContent: `name: different_name
metadata:
  name: different_name`,
			expectedResult: shared.ManualReview,
			expectedReason: "Name field value 'different_name' does not match expected filename-based name 'sa_astro_prod_appuser'",
		},
		{
			name:     "astro service account missing name field",
			filePath: "dataproducts/analytics/sa_astro_staging_appuser.yaml",
			fileContent: `metadata:
  name: sa_astro_staging_appuser
  namespace: analytics`,
			expectedResult: shared.ManualReview,
			expectedReason: "YAML file does not contain a 'name' field",
		},
		{
			name:     "astro service account with non-string name",
			filePath: "dataproducts/analytics/sa_astro_test_appuser.yaml",
			fileContent: `name: 123
metadata:
  name: sa_astro_test_appuser`,
			expectedResult: shared.ManualReview,
			expectedReason: "'name' field is not a string",
		},
		{
			name:           "invalid yaml content",
			filePath:       "dataproducts/analytics/sa_astro_dev_appuser.yaml",
			fileContent:    `invalid: yaml: content: [`,
			expectedResult: shared.ManualReview,
			expectedReason: "Failed to parse YAML content",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			decision, reason := rule.validateAstroServiceAccount(tt.filePath, tt.fileContent)
			assert.Equal(t, tt.expectedResult, decision)
			assert.Contains(t, reason, tt.expectedReason)
		})
	}
}


func TestServiceAccountRule_ValidateLines(t *testing.T) {
	rule := NewServiceAccountRule(nil)

	tests := []struct {
		name             string
		filePath         string
		fileContent      string
		expectedResult   shared.DecisionType
		expectedReason   string
	}{
		// Astro service account tests - only these should be auto-approved
		{
			name:     "valid astro service account",
			filePath: "dataproducts/analytics/sa_astro_dev_appuser.yaml",
			fileContent: `name: sa_astro_dev_appuser
metadata:
  name: sa_astro_dev_appuser`,
			expectedResult: shared.Approve,
			expectedReason: "Astro service account file follows naming convention and name field matches filename",
		},
		
		// All non-Astro service account files should require manual review
		{
			name:     "generic service account - manual review required",
			filePath: "serviceaccounts/user-sa.yaml",
			fileContent: `metadata:
  name: user-sa
spec:
  type: service-account`,
			expectedResult: shared.ManualReview,
			expectedReason: "Only Astro service account files (*_astro_*.yaml/yml) are auto-approved - other service account files require manual review",
		},
		
		{
			name:     "basic service account - manual review required",
			filePath: "configs/myserviceaccount.yaml",
			fileContent: `apiVersion: v1
kind: ServiceAccount
metadata:
  name: my-service-account`,
			expectedResult: shared.ManualReview,
			expectedReason: "Only Astro service account files (*_astro_*.yaml/yml) are auto-approved - other service account files require manual review",
		},
		
		{
			name:     "appuser service account - manual review required",
			filePath: "serviceaccounts/user_appuser.yaml",
			fileContent: `metadata:
  name: user_appuser`,
			expectedResult: shared.ManualReview,
			expectedReason: "Only Astro service account files (*_astro_*.yaml/yml) are auto-approved - other service account files require manual review",
		},
		
		// Non-service account file
		{
			name:           "non-service account file",
			filePath:       "README.md",
			fileContent:    "# README\nThis is a readme file",
			expectedResult: shared.ManualReview,
			expectedReason: "Not a service account file",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lineRanges := []shared.LineRange{{StartLine: 1, EndLine: 10, FilePath: tt.filePath}}
			decision, reason := rule.ValidateLines(tt.filePath, tt.fileContent, lineRanges)
			assert.Equal(t, tt.expectedResult, decision)
			assert.Contains(t, reason, tt.expectedReason)
		})
	}
}

func TestServiceAccountRule_getExpectedNameFromFilename(t *testing.T) {
	rule := NewServiceAccountRule(nil)

	tests := []struct {
		name         string
		filePath     string
		expectedName string
	}{
		{"yaml extension", "dataproducts/analytics/sa_astro_analytics.yaml", "sa_astro_analytics"},
		{"yml extension", "path/to/my_astro_service.yml", "my_astro_service"},
		{"uppercase extensions", "SA_ASTRO_TEST.YAML", "SA_ASTRO_TEST"},
		{"no extension", "service_account", ""},
		{"wrong extension", "service_account.txt", ""},
		{"empty path", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := rule.getExpectedNameFromFilename(tt.filePath)
			assert.Equal(t, tt.expectedName, result)
		})
	}
}

// Integration tests showing the complete workflow
func TestServiceAccountRule_IntegrationScenarios(t *testing.T) {
	tests := []struct {
		name               string
		filePath           string
		fileContent        string
		expectCoverage     bool
		expectedDecision   shared.DecisionType
		expectedReasonPart string
	}{
		{
			name:     "Astro service account - valid scenario (auto-approved)",
			filePath: "dataproducts/source/fivetranplatform/sandbox/sa_astro_sandbox_appuser.yaml",
			fileContent: `name: sa_astro_sandbox_appuser
metadata:
  name: sa_astro_sandbox_appuser
  namespace: sandbox
spec:
  type: astro-service-account`,
			expectCoverage:     true,
			expectedDecision:   shared.Approve,
			expectedReasonPart: "Astro service account file follows naming convention",
		},
		{
			name:     "Generic service account - requires manual review",
			filePath: "serviceaccounts/user-service-account.yaml",
			fileContent: `metadata:
  name: user-service-account
  namespace: default
spec:
  automountServiceAccountToken: false`,
			expectCoverage:     true,
			expectedDecision:   shared.ManualReview,
			expectedReasonPart: "Only Astro service account files (*_astro_*.yaml/yml) are auto-approved - other service account files require manual review",
		},
		{
			name:     "Basic service account - requires manual review",
			filePath: "configs/myserviceaccount.yaml",
			fileContent: `apiVersion: v1
kind: ServiceAccount
metadata:
  name: my-service-account
  namespace: production`,
			expectCoverage:     true,
			expectedDecision:   shared.ManualReview,
			expectedReasonPart: "Only Astro service account files (*_astro_*.yaml/yml) are auto-approved - other service account files require manual review",
		},
		{
			name:     "Astro service account - name mismatch (manual review)",
			filePath: "dataproducts/analytics/sa_astro_dev_appuser.yaml",
			fileContent: `name: wrong_name
metadata:
  name: wrong_name`,
			expectCoverage:     true,
			expectedDecision:   shared.ManualReview,
			expectedReasonPart: "does not match expected filename-based name",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rule := NewServiceAccountRule(nil)
			
			// Test coverage
			lines := rule.GetCoveredLines(tt.filePath, tt.fileContent)
			if tt.expectCoverage {
				assert.NotEmpty(t, lines, "Should cover service account files")
			} else {
				assert.Empty(t, lines, "Should not cover non-service account files")
			}

			// Test validation
			lineRanges := []shared.LineRange{{StartLine: 1, EndLine: 20, FilePath: tt.filePath}}
			decision, reason := rule.ValidateLines(tt.filePath, tt.fileContent, lineRanges)

			assert.Equal(t, tt.expectedDecision, decision)
			assert.Contains(t, reason, tt.expectedReasonPart)
		})
	}
}

func TestServiceAccountRule_SetMRContext(t *testing.T) {
	rule := NewServiceAccountRule(nil)

	mrCtx := &shared.MRContext{
		ProjectID: 123,
		MRIID:     456,
		Changes: []gitlab.FileChange{
			{NewPath: "serviceaccounts/test-sa.yaml"},
		},
	}

	rule.SetMRContext(mrCtx)
	assert.Equal(t, mrCtx, rule.GetMRContext())
}
