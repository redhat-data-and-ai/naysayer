package tag

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestValidator_ValidTag(t *testing.T) {
	validator := NewValidator()

	tag := &Tag{
		Kind:        "Tag",
		Name:        "analytics_pii",
		Description: "String and Float tag for pii data masking",
		DataProduct: "analytics",
		MaskingPolicies: []Policy{
			{Name: "analytics_pii_string_policy"},
			{Name: "analytics_pii_float_policy"},
		},
		AllowedValues: []string{"default"},
	}

	result := validator.Validate(tag, "analytics")

	assert.True(t, result.IsValid)
	assert.Empty(t, result.Errors)
}

func TestValidator_MissingRequiredFields(t *testing.T) {
	validator := NewValidator()

	tests := []struct {
		name          string
		tag           *Tag
		expectedField string
	}{
		{
			name:          "missing kind",
			tag:           &Tag{Name: "test_tag", Description: "desc", DataProduct: "test", MaskingPolicies: []Policy{{Name: "test_pii_string_policy"}}, AllowedValues: []string{"default"}},
			expectedField: "kind",
		},
		{
			name:          "missing name",
			tag:           &Tag{Kind: "Tag", Description: "desc", DataProduct: "test", MaskingPolicies: []Policy{{Name: "test_pii_string_policy"}}, AllowedValues: []string{"default"}},
			expectedField: "name",
		},
		{
			name:          "missing description",
			tag:           &Tag{Kind: "Tag", Name: "test_pii", DataProduct: "test", MaskingPolicies: []Policy{{Name: "test_pii_string_policy"}}, AllowedValues: []string{"default"}},
			expectedField: "description",
		},
		{
			name:          "missing data_product",
			tag:           &Tag{Kind: "Tag", Name: "test_pii", Description: "desc", MaskingPolicies: []Policy{{Name: "test_pii_string_policy"}}, AllowedValues: []string{"default"}},
			expectedField: "data_product",
		},
		{
			name:          "missing masking_policies",
			tag:           &Tag{Kind: "Tag", Name: "test_pii", Description: "desc", DataProduct: "test", AllowedValues: []string{"default"}},
			expectedField: "masking_policies",
		},
		{
			name:          "missing allowed_values",
			tag:           &Tag{Kind: "Tag", Name: "test_pii", Description: "desc", DataProduct: "test", MaskingPolicies: []Policy{{Name: "test_pii_string_policy"}}},
			expectedField: "allowed_values",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := validator.Validate(tt.tag, "")
			assert.False(t, result.IsValid)
			assert.True(t, hasErrorForField(result, tt.expectedField), "expected error for field: %s", tt.expectedField)
		})
	}
}

func TestValidator_InvalidKind(t *testing.T) {
	validator := NewValidator()

	tag := &Tag{
		Kind:            "MaskingPolicy",
		Name:            "analytics_pii",
		Description:     "desc",
		DataProduct:     "analytics",
		MaskingPolicies: []Policy{{Name: "analytics_pii_string_policy"}},
		AllowedValues:   []string{"default"},
	}

	result := validator.Validate(tag, "analytics")

	assert.False(t, result.IsValid)
	assert.True(t, hasErrorForField(result, "kind"))
}

func TestValidator_InvalidTagNamePattern(t *testing.T) {
	validator := NewValidator()

	tests := []struct {
		name    string
		tagName string
	}{
		{"hyphens instead of underscores", "analytics-pii"},
		{"no underscore", "analyticspii"},
		{"uppercase", "Analytics_pii"},
		{"too short prefix", "ab_pii"},
		{"special characters", "analytics_pii!"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tag := &Tag{
				Kind:            "Tag",
				Name:            tt.tagName,
				Description:     "desc",
				DataProduct:     "analytics",
				MaskingPolicies: []Policy{{Name: "analytics_pii_string_policy"}},
				AllowedValues:   []string{"default"},
			}

			result := validator.Validate(tag, "")
			assert.False(t, result.IsValid)
			assert.True(t, hasErrorForField(result, "name"))
		})
	}
}

func TestValidator_TagNamePrefixMismatch(t *testing.T) {
	validator := NewValidator()

	tag := &Tag{
		Kind:            "Tag",
		Name:            "other_pii",
		Description:     "desc",
		DataProduct:     "analytics",
		MaskingPolicies: []Policy{{Name: "analytics_pii_string_policy"}},
		AllowedValues:   []string{"default"},
	}

	result := validator.Validate(tag, "")

	assert.False(t, result.IsValid)
	assert.True(t, hasErrorForField(result, "name"))
	assert.Contains(t, result.GetErrorMessages()[0], "must start with data product prefix")
}

func TestValidator_DataProductPathMismatch(t *testing.T) {
	validator := NewValidator()

	tag := &Tag{
		Kind:            "Tag",
		Name:            "analytics_pii",
		Description:     "desc",
		DataProduct:     "analytics",
		MaskingPolicies: []Policy{{Name: "analytics_pii_string_policy"}},
		AllowedValues:   []string{"default"},
	}

	result := validator.Validate(tag, "different_dp")

	assert.False(t, result.IsValid)
	assert.True(t, hasErrorForField(result, "data_product"))
	assert.Contains(t, result.GetErrorMessages()[0], "mismatch with file path")
}

func TestValidator_InvalidDataProductPattern(t *testing.T) {
	validator := NewValidator()

	tests := []struct {
		name        string
		dataProduct string
	}{
		{"hyphens", "my-analytics"},
		{"uppercase", "Analytics"},
		{"too short", "ab"},
		{"special characters", "analytics!"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tag := &Tag{
				Kind:            "Tag",
				Name:            tt.dataProduct + "_pii",
				Description:     "desc",
				DataProduct:     tt.dataProduct,
				MaskingPolicies: []Policy{{Name: tt.dataProduct + "_pii_string_policy"}},
				AllowedValues:   []string{"default"},
			}

			result := validator.Validate(tag, "")
			assert.False(t, result.IsValid)
			assert.True(t, hasErrorForField(result, "data_product"))
		})
	}
}

func TestValidator_AllowedValuesCount(t *testing.T) {
	validator := NewValidator()

	// Test with too many allowed values (>20)
	tooManyValues := make([]string, 21)
	for i := 0; i < 21; i++ {
		tooManyValues[i] = "value"
	}

	tag := &Tag{
		Kind:            "Tag",
		Name:            "analytics_pii",
		Description:     "desc",
		DataProduct:     "analytics",
		MaskingPolicies: []Policy{{Name: "analytics_pii_string_policy"}},
		AllowedValues:   tooManyValues,
	}

	result := validator.Validate(tag, "")

	assert.False(t, result.IsValid)
	assert.True(t, hasErrorForField(result, "allowed_values"))
}

func TestValidator_InvalidMaskingPolicyPattern(t *testing.T) {
	validator := NewValidator()

	tests := []struct {
		name       string
		policyName string
	}{
		{"hyphens", "analytics-pii-string-policy"},
		{"missing type suffix", "analytics_pii_policy"},
		{"wrong type", "analytics_pii_text_policy"},
		{"uppercase", "Analytics_pii_string_policy"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tag := &Tag{
				Kind:            "Tag",
				Name:            "analytics_pii",
				Description:     "desc",
				DataProduct:     "analytics",
				MaskingPolicies: []Policy{{Name: tt.policyName}},
				AllowedValues:   []string{"default"},
			}

			result := validator.Validate(tag, "")
			assert.False(t, result.IsValid)
			assert.True(t, hasErrorForField(result, "masking_policies[0].name"))
		})
	}
}

func TestValidator_MaskingPolicyOwnershipMismatch(t *testing.T) {
	validator := NewValidator()

	tag := &Tag{
		Kind:            "Tag",
		Name:            "analytics_pii",
		Description:     "desc",
		DataProduct:     "analytics",
		MaskingPolicies: []Policy{{Name: "other_pii_string_policy"}},
		AllowedValues:   []string{"default"},
	}

	result := validator.Validate(tag, "")

	assert.False(t, result.IsValid)
	assert.True(t, hasErrorForField(result, "masking_policies[0].name"))
	assert.Contains(t, result.GetErrorMessages()[0], "must belong to data product")
}

func TestValidator_MultipleMaskingPolicies(t *testing.T) {
	validator := NewValidator()

	tag := &Tag{
		Kind:        "Tag",
		Name:        "analytics_pii",
		Description: "String, Float and Number tag",
		DataProduct: "analytics",
		MaskingPolicies: []Policy{
			{Name: "analytics_pii_string_policy"},
			{Name: "analytics_pii_float_policy"},
			{Name: "analytics_pii_number_policy"},
		},
		AllowedValues: []string{"default"},
	}

	result := validator.Validate(tag, "analytics")

	assert.True(t, result.IsValid)
	assert.Empty(t, result.Errors)
}

func TestValidator_RestrictedTag(t *testing.T) {
	validator := NewValidator()

	tag := &Tag{
		Kind:        "Tag",
		Name:        "cloudoscope_restricted",
		Description: "Float tag for restricted data masking",
		DataProduct: "cloudoscope",
		MaskingPolicies: []Policy{
			{Name: "cloudoscope_restricted_float_policy"},
		},
		AllowedValues: []string{"default"},
	}

	result := validator.Validate(tag, "cloudoscope")

	assert.True(t, result.IsValid)
	assert.Empty(t, result.Errors)
}

func TestValidator_CaseInsensitiveKind(t *testing.T) {
	validator := NewValidator()

	tag := &Tag{
		Kind:            "tag",
		Name:            "analytics_pii",
		Description:     "desc",
		DataProduct:     "analytics",
		MaskingPolicies: []Policy{{Name: "analytics_pii_string_policy"}},
		AllowedValues:   []string{"default"},
	}

	result := validator.Validate(tag, "")

	assert.True(t, result.IsValid)
}

// Helper function to check if an error exists for a specific field
func hasErrorForField(result *ValidationResult, field string) bool {
	for _, err := range result.Errors {
		if err.Field == field {
			return true
		}
	}
	return false
}
