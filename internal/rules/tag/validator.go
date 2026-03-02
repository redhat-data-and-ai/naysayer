package tag

import (
	"fmt"
	"strings"
)

// Validator validates tag configurations
type Validator struct{}

// NewValidator creates a new tag validator
func NewValidator() *Validator {
	return &Validator{}
}

// Validate performs all validations on a tag
func (v *Validator) Validate(tag *Tag, dataProductFromPath string) *ValidationResult {
	result := NewValidationResult()

	// 1. Required field validations
	v.validateRequiredFields(tag, result)

	// If required fields are missing, skip further validation
	if !result.IsValid {
		return result
	}

	// 2. Kind validation
	v.validateKind(tag, result)

	// 3. Tag name validation
	v.validateTagName(tag, result)

	// 4. Tag name prefix validation (must start with data_product)
	v.validateTagNamePrefix(tag, result)

	// 5. Data product name validation
	v.validateDataProductName(tag, result)

	// 6. Data product path validation
	v.validateDataProductPath(tag, dataProductFromPath, result)

	// 7. Allowed values validation
	v.validateAllowedValues(tag, result)

	// 8. Masking policies validation (name patterns and ownership)
	v.validateMaskingPolicies(tag, result)

	return result
}

// validateRequiredFields checks all required fields are present
func (v *Validator) validateRequiredFields(tag *Tag, result *ValidationResult) {
	if strings.TrimSpace(tag.Kind) == "" {
		result.AddError("kind", "is required")
	}
	if strings.TrimSpace(tag.Name) == "" {
		result.AddError("name", "is required")
	}
	if strings.TrimSpace(tag.DataProduct) == "" {
		result.AddError("data_product", "is required")
	}
	if strings.TrimSpace(tag.Description) == "" {
		result.AddError("description", "is required")
	}
	if len(tag.MaskingPolicies) == 0 {
		result.AddError("masking_policies", "at least one masking policy is required")
	}
	if len(tag.AllowedValues) == 0 {
		result.AddError("allowed_values", "at least one allowed value is required")
	}

	// Validate each masking policy has a name
	for i, policy := range tag.MaskingPolicies {
		if strings.TrimSpace(policy.Name) == "" {
			result.AddError(fmt.Sprintf("masking_policies[%d].name", i), "is required")
		}
	}
}

// validateKind checks the kind is Tag (case-insensitive)
func (v *Validator) validateKind(tag *Tag, result *ValidationResult) {
	if !strings.EqualFold(tag.Kind, TagKind) {
		result.AddError("kind", fmt.Sprintf("must be '%s', found: %s", TagKind, tag.Kind))
	}
}

// validateTagName checks the tag name follows the regex pattern
func (v *Validator) validateTagName(tag *Tag, result *ValidationResult) {
	if !TagNameRegex.MatchString(tag.Name) {
		result.AddError("name", fmt.Sprintf("must follow pattern '%s' (e.g., 'analytics_pii'), found: %s", TagNameRegex.String(), tag.Name))
	}
}

// validateTagNamePrefix ensures tag name starts with data_product + "_"
func (v *Validator) validateTagNamePrefix(tag *Tag, result *ValidationResult) {
	if tag.Name == "" || tag.DataProduct == "" {
		return
	}
	expectedPrefix := tag.DataProduct + "_"
	if !strings.HasPrefix(tag.Name, expectedPrefix) {
		result.AddError("name", fmt.Sprintf("must start with data product prefix '%s', found: %s", expectedPrefix, tag.Name))
	}
}

// validateDataProductName checks the data product name follows the regex pattern
func (v *Validator) validateDataProductName(tag *Tag, result *ValidationResult) {
	if !DataProductRegex.MatchString(tag.DataProduct) {
		result.AddError("data_product", fmt.Sprintf("must follow pattern '%s' (3-30 lowercase alphanumeric), found: %s", DataProductRegex.String(), tag.DataProduct))
	}
}

// validateDataProductPath checks data_product matches the file path directory
func (v *Validator) validateDataProductPath(tag *Tag, dataProductFromPath string, result *ValidationResult) {
	if dataProductFromPath != "" && !strings.EqualFold(tag.DataProduct, dataProductFromPath) {
		result.AddError("data_product", fmt.Sprintf("mismatch with file path: expected '%s', found '%s'", dataProductFromPath, tag.DataProduct))
	}
}

// validateAllowedValues checks allowed_values has 1-20 items
func (v *Validator) validateAllowedValues(tag *Tag, result *ValidationResult) {
	count := len(tag.AllowedValues)
	if count < MinAllowedValues || count > MaxAllowedValues {
		result.AddError("allowed_values", fmt.Sprintf("must have %d-%d values, found: %d", MinAllowedValues, MaxAllowedValues, count))
	}
}

// validateMaskingPolicies validates all masking policy names and ownership
func (v *Validator) validateMaskingPolicies(tag *Tag, result *ValidationResult) {
	for i, policy := range tag.MaskingPolicies {
		fieldName := fmt.Sprintf("masking_policies[%d].name", i)

		// Validate name pattern
		if !MaskingPolicyNameRegex.MatchString(policy.Name) {
			result.AddError(fieldName, fmt.Sprintf("must follow pattern '%s', found: %s", MaskingPolicyNameRegex.String(), policy.Name))
			continue
		}

		// Validate ownership (policy name starts with data_product + "_")
		expectedPrefix := tag.DataProduct + "_"
		if !strings.HasPrefix(policy.Name, expectedPrefix) {
			result.AddError(fieldName, fmt.Sprintf("must belong to data product '%s' (should start with '%s'), found: %s", tag.DataProduct, expectedPrefix, policy.Name))
		}
	}
}
