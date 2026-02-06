package masking

import (
	"fmt"
	"strings"
)

// Validator validates masking policy configurations
type Validator struct{}

// NewValidator creates a new masking policy validator
func NewValidator() *Validator {
	return &Validator{}
}

// Validate performs all validations on a masking policy
func (v *Validator) Validate(policy *MaskingPolicy, dataProductFromPath string, environment string) *ValidationResult {
	result := NewValidationResult()

	// 1. Required field validations
	v.validateRequiredFields(policy, result)

	// If required fields are missing, skip further validation
	if !result.IsValid {
		return result
	}

	// 2. Kind validation
	v.validateKind(policy, result)

	// 3. Naming convention validation
	v.validateNamingConvention(policy, result)

	// 4. Data product path validation
	v.validateDataProductPath(policy, dataProductFromPath, result)

	// 5. Datatype validation
	v.validateDatatype(policy, result)

	// 6. Datatype matches name suffix validation
	v.validateDatatypeMatchesName(policy, result)

	// 7. Mask format validation
	v.validateMaskFormat(policy, result)

	// 8. Strategy validation
	v.validateStrategies(policy, result)

	// 9. Strategy ordering validation (for string type)
	v.validateStrategyOrdering(policy, result)

	// 10. Consumer validation
	v.validateConsumers(policy, environment, result)

	// 11. Duplicate consumer validation
	v.validateNoDuplicateConsumers(policy, result)

	return result
}

// validateRequiredFields checks all required fields are present
func (v *Validator) validateRequiredFields(policy *MaskingPolicy, result *ValidationResult) {
	if strings.TrimSpace(policy.Kind) == "" {
		result.AddError("kind", "is required")
	}
	if strings.TrimSpace(policy.Name) == "" {
		result.AddError("name", "is required")
	}
	if strings.TrimSpace(policy.DataProduct) == "" {
		result.AddError("data_product", "is required")
	}
	if strings.TrimSpace(policy.DataType) == "" {
		result.AddError("datatype", "is required")
	}
	if strings.TrimSpace(policy.Mask) == "" {
		result.AddError("mask", "is required")
	}
	if len(policy.Cases) == 0 {
		result.AddError("cases", "at least one case is required")
	}

	// Validate each case has required fields
	for i, c := range policy.Cases {
		if strings.TrimSpace(c.Strategy) == "" {
			result.AddError(fmt.Sprintf("cases[%d].strategy", i), "is required")
		}
		if len(c.Consumers) == 0 {
			result.AddError(fmt.Sprintf("cases[%d].consumers", i), "at least one consumer is required")
		}
		for j, consumer := range c.Consumers {
			if strings.TrimSpace(consumer.Kind) == "" {
				result.AddError(fmt.Sprintf("cases[%d].consumers[%d].kind", i, j), "is required")
			}
			if strings.TrimSpace(consumer.Name) == "" {
				result.AddError(fmt.Sprintf("cases[%d].consumers[%d].name", i, j), "is required")
			}
		}
	}
}

// validateKind checks the kind is MaskingPolicy (case-insensitive)
func (v *Validator) validateKind(policy *MaskingPolicy, result *ValidationResult) {
	if !strings.EqualFold(policy.Kind, MaskingPolicyKind) {
		result.AddError("kind", fmt.Sprintf("must be '%s', found: %s", MaskingPolicyKind, policy.Kind))
	}
}

// validateNamingConvention checks the policy name follows the pattern
func (v *Validator) validateNamingConvention(policy *MaskingPolicy, result *ValidationResult) {
	if !MaskingPolicyNameRegex.MatchString(policy.Name) {
		result.AddError("name", fmt.Sprintf("must follow pattern '<dataproduct>_(pii|restricted|restrictedpii)_(string|float|number)_policy', found: %s", policy.Name))
	}
}

// validateDataProductPath checks data_product matches the file path directory
func (v *Validator) validateDataProductPath(policy *MaskingPolicy, dataProductFromPath string, result *ValidationResult) {
	if dataProductFromPath != "" && !strings.EqualFold(policy.DataProduct, dataProductFromPath) {
		result.AddError("data_product", fmt.Sprintf("mismatch with file path: expected '%s', found '%s'", dataProductFromPath, policy.DataProduct))
	}
}

// validateDatatype checks datatype is valid
func (v *Validator) validateDatatype(policy *MaskingPolicy, result *ValidationResult) {
	if !contains(ValidDataTypes, strings.ToLower(policy.DataType)) {
		result.AddError("datatype", fmt.Sprintf("must be one of %v, found: %s", ValidDataTypes, policy.DataType))
	}
}

// validateDatatypeMatchesName checks datatype field matches the suffix in name
func (v *Validator) validateDatatypeMatchesName(policy *MaskingPolicy, result *ValidationResult) {
	// Extract datatype from name (e.g., "hellosource_pii_string_policy" -> "string")
	parts := strings.Split(policy.Name, "_")
	if len(parts) >= 2 {
		// The datatype is the second-to-last part before "policy"
		expectedDatatype := parts[len(parts)-2]
		if !strings.EqualFold(policy.DataType, expectedDatatype) {
			result.AddError("datatype", fmt.Sprintf("must match name suffix: name has '%s', datatype field has '%s'", expectedDatatype, policy.DataType))
		}
	}
}

// validateMaskFormat checks mask value format based on datatype
func (v *Validator) validateMaskFormat(policy *MaskingPolicy, result *ValidationResult) {
	mask := strings.TrimSpace(policy.Mask)
	datatype := strings.ToLower(policy.DataType)

	switch datatype {
	case DataTypeString:
		// For string, any non-empty value is valid
		if mask == "" {
			result.AddError("mask", "cannot be empty for string datatype")
		}
	case DataTypeFloat:
		// For float, must be a valid decimal number
		if !FloatMaskRegex.MatchString(mask) {
			result.AddError("mask", fmt.Sprintf("must be a valid float number for float datatype (e.g., '-9.0', '0.0'), found: %s", mask))
		}
	case DataTypeNumber:
		// For number, must be a valid integer
		if !NumberMaskRegex.MatchString(mask) {
			result.AddError("mask", fmt.Sprintf("must be a valid integer for number datatype (e.g., '8888', '-9'), found: %s", mask))
		}
	}
}

// validateStrategies checks all strategies are valid
func (v *Validator) validateStrategies(policy *MaskingPolicy, result *ValidationResult) {
	datatype := strings.ToLower(policy.DataType)

	for i, c := range policy.Cases {
		strategy := strings.ToUpper(c.Strategy)

		// Check strategy is valid
		if !contains(ValidStrategies, strategy) {
			result.AddError(fmt.Sprintf("cases[%d].strategy", i), fmt.Sprintf("must be one of %v, found: %s", ValidStrategies, c.Strategy))
			continue
		}

		// Check HASH_SHA1 is only used with string datatype
		if strategy == StrategyHashSha1 && datatype != DataTypeString {
			result.AddError(fmt.Sprintf("cases[%d].strategy", i), fmt.Sprintf("HASH_SHA1 is only supported for string datatype, found datatype: %s", policy.DataType))
		}
	}
}

// validateStrategyOrdering checks UNMASKED comes before HASH_SHA1 for string policies
func (v *Validator) validateStrategyOrdering(policy *MaskingPolicy, result *ValidationResult) {
	// Only applies to string datatype with both strategies
	if strings.ToLower(policy.DataType) != DataTypeString {
		return
	}

	hasUnmasked := false
	hasHashSha1 := false
	unmaskedIndex := -1
	hashSha1Index := -1

	for i, c := range policy.Cases {
		strategy := strings.ToUpper(c.Strategy)
		if strategy == StrategyUnmasked {
			hasUnmasked = true
			unmaskedIndex = i
		}
		if strategy == StrategyHashSha1 {
			hasHashSha1 = true
			hashSha1Index = i
		}
	}

	// If both strategies exist, UNMASKED must come first
	if hasUnmasked && hasHashSha1 && hashSha1Index < unmaskedIndex {
		result.AddError("cases", "for string policies, UNMASKED strategy must come before HASH_SHA1 to ensure correct precedence")
	}
}

// validateConsumers checks all consumers are valid
func (v *Validator) validateConsumers(policy *MaskingPolicy, environment string, result *ValidationResult) {
	for i, c := range policy.Cases {
		for j, consumer := range c.Consumers {
			kind := strings.ToLower(consumer.Kind)
			name := strings.ToLower(consumer.Name)

			// Check consumer kind is valid
			if !contains(ValidConsumerKinds, kind) {
				result.AddError(fmt.Sprintf("cases[%d].consumers[%d].kind", i, j), fmt.Sprintf("must be one of %v, found: %s", ValidConsumerKinds, consumer.Kind))
				continue
			}

			// Validate consumer name based on kind
			switch kind {
			case ConsumerKindGroup:
				if !ConsumerGroupRegex.MatchString(name) {
					result.AddError(fmt.Sprintf("cases[%d].consumers[%d].name", i, j), fmt.Sprintf("consumer_group name must follow pattern 'dataverse-(source|aggregate|consumer)-<dataproduct>(-<suffix>)?', found: %s", consumer.Name))
				}
			case ConsumerKindServiceAccount:
				if !ServiceAccountRegex.MatchString(name) {
					result.AddError(fmt.Sprintf("cases[%d].consumers[%d].name", i, j), fmt.Sprintf("service_account name must follow pattern '<name>_<appname>_<env>_appuser', found: %s", consumer.Name))
				}
				// Additional check: environment in name should match file path environment
				if environment != "" {
					expectedSuffix := "_" + environment + "_appuser"
					if !strings.HasSuffix(name, expectedSuffix) {
						result.AddError(fmt.Sprintf("cases[%d].consumers[%d].name", i, j), fmt.Sprintf("service_account name should end with '_%s_appuser' for this environment, found: %s", environment, consumer.Name))
					}
				}
			}
		}
	}
}

// validateNoDuplicateConsumers checks the same consumer is not in multiple cases
func (v *Validator) validateNoDuplicateConsumers(policy *MaskingPolicy, result *ValidationResult) {
	seen := make(map[string]int) // consumer identifier -> case index

	for i, c := range policy.Cases {
		for _, consumer := range c.Consumers {
			// Create unique identifier: kind:name (lowercase for comparison)
			id := strings.ToLower(consumer.Kind) + ":" + strings.ToLower(consumer.Name)

			if existingCase, found := seen[id]; found {
				result.AddError("consumers", fmt.Sprintf("consumer '%s' (kind: %s) appears in multiple cases: case %d and case %d", consumer.Name, consumer.Kind, existingCase, i))
			} else {
				seen[id] = i
			}
		}
	}
}
