package masking

import (
	"strings"
	"testing"

	"github.com/redhat-data-and-ai/naysayer/internal/rules/shared"
)

func TestRule_Name(t *testing.T) {
	rule := NewRule(nil)
	if rule.Name() != "masking_policy_rule" {
		t.Errorf("expected 'masking_policy_rule', got '%s'", rule.Name())
	}
}

func TestRule_Description(t *testing.T) {
	rule := NewRule(nil)
	if rule.Description() == "" {
		t.Error("description should not be empty")
	}
}

func TestRule_GetCoveredLines(t *testing.T) {
	rule := NewRule(nil)

	tests := []struct {
		name        string
		filePath    string
		fileContent string
		expectLines bool
	}{
		{"masking yaml file", "dataproducts/source/analytics/sandbox/pii_masking.yaml", "kind: MaskingPolicy", true},
		{"masking yml file", "dataproducts/source/analytics/sandbox/pii_masking.yml", "kind: MaskingPolicy", true},
		{"non-masking file", "dataproducts/source/analytics/sandbox/product.yaml", "name: analytics", false},
		{"empty/deleted masking file - still covered for manual review", "dataproducts/source/analytics/sandbox/pii_masking.yaml", "", true},
		{"tag masking file - not covered", "dataproducts/source/analytics/sandbox/tag_pii_masking.yaml", "kind: Tag", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lines := rule.GetCoveredLines(tt.filePath, tt.fileContent)

			if tt.expectLines && len(lines) == 0 {
				t.Errorf("expected lines to be covered, got none")
			}
			if !tt.expectLines && len(lines) > 0 {
				t.Errorf("expected no lines to be covered, got %d", len(lines))
			}
		})
	}
}

func TestRule_ValidateLines_ValidPolicy(t *testing.T) {
	rule := NewRule(nil)

	validYAML := `kind: MaskingPolicy
name: analytics_pii_string_policy
data_product: analytics
datatype: string
mask: "==MASKED=="
cases:
  - strategy: UNMASKED
    consumers:
      - kind: consumer_group
        name: dataverse-source-analytics
`

	filePath := "dataproducts/source/analytics/sandbox/pii_masking.yaml"
	decision, reason := rule.ValidateLines(filePath, validYAML, nil)

	if decision != shared.Approve {
		t.Errorf("expected Approve, got %s: %s", decision, reason)
	}
}

func TestRule_ValidateLines_InvalidPolicyName(t *testing.T) {
	rule := NewRule(nil)

	invalidYAML := `kind: MaskingPolicy
name: analytics-pii-string-policy
data_product: analytics
datatype: string
mask: "==MASKED=="
cases:
  - strategy: UNMASKED
    consumers:
      - kind: consumer_group
        name: dataverse-source-analytics
`

	filePath := "dataproducts/source/analytics/sandbox/pii_masking.yaml"
	decision, reason := rule.ValidateLines(filePath, invalidYAML, nil)

	if decision != shared.ManualReview {
		t.Errorf("expected ManualReview, got %s: %s", decision, reason)
	}
}

func TestRule_ValidateLines_InvalidStrategy(t *testing.T) {
	rule := NewRule(nil)

	invalidYAML := `kind: MaskingPolicy
name: analytics_pii_float_policy
data_product: analytics
datatype: float
mask: "-9.0"
cases:
  - strategy: HASH_SHA1
    consumers:
      - kind: consumer_group
        name: dataverse-source-analytics
`

	filePath := "dataproducts/source/analytics/sandbox/pii_masking.yaml"
	decision, reason := rule.ValidateLines(filePath, invalidYAML, nil)

	if decision != shared.ManualReview {
		t.Errorf("expected ManualReview for HASH_SHA1 with float, got %s: %s", decision, reason)
	}
}

func TestRule_ValidateLines_WrongStrategyOrder(t *testing.T) {
	rule := NewRule(nil)

	invalidYAML := `kind: MaskingPolicy
name: analytics_pii_string_policy
data_product: analytics
datatype: string
mask: "==MASKED=="
cases:
  - strategy: HASH_SHA1
    consumers:
      - kind: consumer_group
        name: dataverse-aggregate-other
  - strategy: UNMASKED
    consumers:
      - kind: consumer_group
        name: dataverse-source-analytics
`

	filePath := "dataproducts/source/analytics/sandbox/pii_masking.yaml"
	decision, reason := rule.ValidateLines(filePath, invalidYAML, nil)

	if decision != shared.ManualReview {
		t.Errorf("expected ManualReview for wrong strategy order, got %s: %s", decision, reason)
	}
}

func TestRule_ValidateLines_InvalidMaskFormat(t *testing.T) {
	rule := NewRule(nil)

	invalidYAML := `kind: MaskingPolicy
name: analytics_pii_float_policy
data_product: analytics
datatype: float
mask: "MASKED"
cases:
  - strategy: UNMASKED
    consumers:
      - kind: consumer_group
        name: dataverse-source-analytics
`

	filePath := "dataproducts/source/analytics/sandbox/pii_masking.yaml"
	decision, reason := rule.ValidateLines(filePath, invalidYAML, nil)

	if decision != shared.ManualReview {
		t.Errorf("expected ManualReview for invalid float mask, got %s: %s", decision, reason)
	}
}

func TestRule_ValidateLines_InvalidConsumerKind(t *testing.T) {
	rule := NewRule(nil)

	invalidYAML := `kind: MaskingPolicy
name: analytics_pii_string_policy
data_product: analytics
datatype: string
mask: "==MASKED=="
cases:
  - strategy: UNMASKED
    consumers:
      - kind: user
        name: testuser
`

	filePath := "dataproducts/source/analytics/sandbox/pii_masking.yaml"
	decision, reason := rule.ValidateLines(filePath, invalidYAML, nil)

	if decision != shared.ManualReview {
		t.Errorf("expected ManualReview for invalid consumer kind, got %s: %s", decision, reason)
	}
}

func TestRule_ValidateLines_DuplicateConsumers(t *testing.T) {
	rule := NewRule(nil)

	invalidYAML := `kind: MaskingPolicy
name: analytics_pii_string_policy
data_product: analytics
datatype: string
mask: "==MASKED=="
cases:
  - strategy: UNMASKED
    consumers:
      - kind: consumer_group
        name: dataverse-source-analytics
  - strategy: HASH_SHA1
    consumers:
      - kind: consumer_group
        name: dataverse-source-analytics
`

	filePath := "dataproducts/source/analytics/sandbox/pii_masking.yaml"
	decision, reason := rule.ValidateLines(filePath, invalidYAML, nil)

	if decision != shared.ManualReview {
		t.Errorf("expected ManualReview for duplicate consumers, got %s: %s", decision, reason)
	}
}

// TestRule_ValidateLines_TagFile_NotHandledByThisRule verifies that tag files
// are not handled by the masking policy rule. Even if ValidateLines is called
// with a tag file, it returns Approve with "Not a masking policy file" reason,
// indicating this rule doesn't process tag files (they need their own rule).
func TestRule_ValidateLines_TagFile_NotHandledByThisRule(t *testing.T) {
	rule := NewRule(nil)

	tagYAML := `kind: Tag
name: analytics_pii
description: PII tag
data_product: analytics
masking_policies:
  - name: analytics_pii_string_policy
allowed_values:
  - default
`

	filePath := "dataproducts/source/analytics/sandbox/tag_pii_masking.yaml"
	decision, reason := rule.ValidateLines(filePath, tagYAML, nil)

	// Tag files are not handled by masking_policy_rule - returns Approve meaning "skip"
	if decision != shared.Approve {
		t.Errorf("expected Approve (not handled by this rule) for tag file, got %s: %s", decision, reason)
	}

	// Verify the reason indicates this file is not handled
	if !strings.Contains(reason, "Not a masking policy file") {
		t.Errorf("expected reason to indicate file is not handled, got: %s", reason)
	}
}

func TestRule_ValidateLines_NonMaskingFile(t *testing.T) {
	rule := NewRule(nil)

	productYAML := `name: analytics
kind: source
`

	filePath := "dataproducts/source/analytics/sandbox/product.yaml"
	decision, reason := rule.ValidateLines(filePath, productYAML, nil)

	if decision != shared.Approve {
		t.Errorf("expected Approve for non-masking file, got %s: %s", decision, reason)
	}
}

func TestRule_ValidateLines_DataProductPathMismatch(t *testing.T) {
	rule := NewRule(nil)

	invalidYAML := `kind: MaskingPolicy
name: ciam_pii_string_policy
data_product: ciam
datatype: string
mask: "==MASKED=="
cases:
  - strategy: UNMASKED
    consumers:
      - kind: consumer_group
        name: dataverse-source-ciam
`

	// Path says "analytics" but policy says "ciam"
	filePath := "dataproducts/source/analytics/sandbox/pii_masking.yaml"
	decision, reason := rule.ValidateLines(filePath, invalidYAML, nil)

	if decision != shared.ManualReview {
		t.Errorf("expected ManualReview for path mismatch, got %s: %s", decision, reason)
	}
}

func TestRule_ExtractDataProductFromPath(t *testing.T) {
	rule := NewRule(nil)

	tests := []struct {
		path     string
		expected string
	}{
		// Valid paths: dataproducts/<type>/<dataproduct>/<env>/<file>
		{"dataproducts/source/analytics/sandbox/pii_masking.yaml", "analytics"},
		{"dataproducts/aggregate/bookingsmaster/prod/masking.yaml", "bookingsmaster"},
		{"dataproducts/platform/ciam/preprod/pii_masking.yaml", "ciam"},
		{"dataproducts/source/hellosource/dev/restricted_float_masking.yaml", "hellosource"},
		// Invalid paths
		{"dataproducts/analytics/prod/pii_masking.yaml", ""}, // missing type directory
		{"some/other/path/file.yaml", ""},
		{"", ""},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			result := rule.extractDataProductFromPath(tt.path)
			if result != tt.expected {
				t.Errorf("expected '%s', got '%s'", tt.expected, result)
			}
		})
	}
}

func TestRule_ExtractEnvironmentFromPath(t *testing.T) {
	rule := NewRule(nil)

	tests := []struct {
		path     string
		expected string
	}{
		// Valid paths: dataproducts/<type>/<dataproduct>/<env>/<file>
		{"dataproducts/source/analytics/sandbox/pii_masking.yaml", "sandbox"},
		{"dataproducts/aggregate/bookingsmaster/prod/masking.yaml", "prod"},
		{"dataproducts/platform/ciam/preprod/pii_masking.yaml", "preprod"},
		{"dataproducts/source/hellosource/dev/restricted_float_masking.yaml", "dev"},
		// Invalid paths
		{"dataproducts/analytics/prod/pii_masking.yaml", ""}, // missing type directory
		{"some/other/path/file.yaml", ""},
		{"", ""},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			result := rule.extractEnvironmentFromPath(tt.path)
			if result != tt.expected {
				t.Errorf("expected '%s', got '%s'", tt.expected, result)
			}
		})
	}
}

func TestRule_ValidateLines_ValidFloatPolicy(t *testing.T) {
	rule := NewRule(nil)

	validYAML := `kind: MaskingPolicy
name: analytics_pii_float_policy
data_product: analytics
datatype: float
mask: "-9.0"
cases:
  - strategy: UNMASKED
    consumers:
      - kind: consumer_group
        name: dataverse-source-analytics
`

	filePath := "dataproducts/source/analytics/sandbox/pii_float_masking.yaml"
	decision, reason := rule.ValidateLines(filePath, validYAML, nil)

	if decision != shared.Approve {
		t.Errorf("expected Approve for valid float policy, got %s: %s", decision, reason)
	}
}

func TestRule_ValidateLines_ValidNumberPolicy(t *testing.T) {
	rule := NewRule(nil)

	validYAML := `kind: MaskingPolicy
name: analytics_pii_number_policy
data_product: analytics
datatype: number
mask: "8888"
cases:
  - strategy: UNMASKED
    consumers:
      - kind: consumer_group
        name: dataverse-source-analytics
`

	filePath := "dataproducts/source/analytics/sandbox/pii_number_masking.yaml"
	decision, reason := rule.ValidateLines(filePath, validYAML, nil)

	if decision != shared.Approve {
		t.Errorf("expected Approve for valid number policy, got %s: %s", decision, reason)
	}
}

func TestRule_ValidateLines_ValidRestrictedPolicy(t *testing.T) {
	rule := NewRule(nil)

	validYAML := `kind: MaskingPolicy
name: analytics_restricted_string_policy
data_product: analytics
datatype: string
mask: "==RESTRICTED=="
cases:
  - strategy: UNMASKED
    consumers:
      - kind: consumer_group
        name: dataverse-source-analytics
`

	filePath := "dataproducts/source/analytics/sandbox/restricted_masking.yaml"
	decision, reason := rule.ValidateLines(filePath, validYAML, nil)

	if decision != shared.Approve {
		t.Errorf("expected Approve for valid restricted policy, got %s: %s", decision, reason)
	}
}

func TestRule_ValidateLines_ValidRestrictedPiiPolicy(t *testing.T) {
	rule := NewRule(nil)

	validYAML := `kind: MaskingPolicy
name: analytics_restrictedpii_string_policy
data_product: analytics
datatype: string
mask: "==RESTRICTED_PII=="
cases:
  - strategy: UNMASKED
    consumers:
      - kind: consumer_group
        name: dataverse-source-analytics
`

	filePath := "dataproducts/source/analytics/sandbox/restrictedpii_masking.yaml"
	decision, reason := rule.ValidateLines(filePath, validYAML, nil)

	if decision != shared.Approve {
		t.Errorf("expected Approve for valid restrictedpii policy, got %s: %s", decision, reason)
	}
}

func TestRule_ValidateLines_ValidServiceAccount(t *testing.T) {
	rule := NewRule(nil)

	validYAML := `kind: MaskingPolicy
name: analytics_pii_string_policy
data_product: analytics
datatype: string
mask: "==MASKED=="
cases:
  - strategy: UNMASKED
    consumers:
      - kind: service_account
        name: analytics_dbt_sandbox_appuser
`

	filePath := "dataproducts/source/analytics/sandbox/pii_masking.yaml"
	decision, reason := rule.ValidateLines(filePath, validYAML, nil)

	if decision != shared.Approve {
		t.Errorf("expected Approve for valid service account, got %s: %s", decision, reason)
	}
}

func TestRule_ValidateLines_CorrectStrategyOrder(t *testing.T) {
	rule := NewRule(nil)

	validYAML := `kind: MaskingPolicy
name: analytics_pii_string_policy
data_product: analytics
datatype: string
mask: "==MASKED=="
cases:
  - strategy: UNMASKED
    consumers:
      - kind: consumer_group
        name: dataverse-source-analytics
  - strategy: HASH_SHA1
    consumers:
      - kind: consumer_group
        name: dataverse-aggregate-other
`

	filePath := "dataproducts/source/analytics/sandbox/pii_masking.yaml"
	decision, reason := rule.ValidateLines(filePath, validYAML, nil)

	if decision != shared.Approve {
		t.Errorf("expected Approve for correct strategy order, got %s: %s", decision, reason)
	}
}

// TestRule_ValidateLines_DeletedMaskingFile verifies that deleted masking policy files
// require manual review since deleting a masking policy removes data protection.
func TestRule_ValidateLines_DeletedMaskingFile(t *testing.T) {
	rule := NewRule(nil)

	filePath := "dataproducts/source/analytics/sandbox/pii_masking.yaml"
	emptyContent := "" // Simulates deleted file (no content in source branch)

	decision, reason := rule.ValidateLines(filePath, emptyContent, nil)

	if decision != shared.ManualReview {
		t.Errorf("expected ManualReview for deleted masking file, got %s: %s", decision, reason)
	}

	if !strings.Contains(reason, "deletion") {
		t.Errorf("expected reason to mention deletion, got: %s", reason)
	}
}

// TestRule_GetCoveredLines_DeletedMaskingFile verifies that deleted masking files
// are still "covered" by this rule so that ValidateLines is called.
func TestRule_GetCoveredLines_DeletedMaskingFile(t *testing.T) {
	rule := NewRule(nil)

	filePath := "dataproducts/source/analytics/sandbox/pii_masking.yaml"
	emptyContent := "" // Simulates deleted file

	lines := rule.GetCoveredLines(filePath, emptyContent)

	if len(lines) == 0 {
		t.Errorf("expected deleted masking file to be covered for manual review")
	}

	// Verify it returns a minimal range for the deleted file
	if lines[0].StartLine != 1 || lines[0].EndLine != 1 {
		t.Errorf("expected lines 1-1 for deleted file, got %d-%d", lines[0].StartLine, lines[0].EndLine)
	}
}
