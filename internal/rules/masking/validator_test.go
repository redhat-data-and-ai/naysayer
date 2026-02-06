package masking

import (
	"testing"
)

func TestValidator_ValidateRequiredFields(t *testing.T) {
	validator := NewValidator()

	tests := []struct {
		name        string
		policy      *MaskingPolicy
		expectValid bool
		expectError string
	}{
		{
			name: "valid complete policy",
			policy: &MaskingPolicy{
				Kind:        "MaskingPolicy",
				Name:        "analytics_pii_string_policy",
				DataProduct: "analytics",
				DataType:    "string",
				Mask:        "==MASKED==",
				Cases: []Case{
					{
						Strategy: "UNMASKED",
						Consumers: []Consumer{
							{Kind: "consumer_group", Name: "dataverse-source-analytics"},
						},
					},
				},
			},
			expectValid: true,
		},
		{
			name: "missing kind",
			policy: &MaskingPolicy{
				Name:        "analytics_pii_string_policy",
				DataProduct: "analytics",
				DataType:    "string",
				Mask:        "==MASKED==",
				Cases: []Case{
					{Strategy: "UNMASKED", Consumers: []Consumer{{Kind: "consumer_group", Name: "dataverse-source-analytics"}}},
				},
			},
			expectValid: false,
			expectError: "kind: is required",
		},
		{
			name: "missing name",
			policy: &MaskingPolicy{
				Kind:        "MaskingPolicy",
				DataProduct: "analytics",
				DataType:    "string",
				Mask:        "==MASKED==",
				Cases: []Case{
					{Strategy: "UNMASKED", Consumers: []Consumer{{Kind: "consumer_group", Name: "dataverse-source-analytics"}}},
				},
			},
			expectValid: false,
			expectError: "name: is required",
		},
		{
			name: "missing data_product",
			policy: &MaskingPolicy{
				Kind:     "MaskingPolicy",
				Name:     "analytics_pii_string_policy",
				DataType: "string",
				Mask:     "==MASKED==",
				Cases: []Case{
					{Strategy: "UNMASKED", Consumers: []Consumer{{Kind: "consumer_group", Name: "dataverse-source-analytics"}}},
				},
			},
			expectValid: false,
			expectError: "data_product: is required",
		},
		{
			name: "missing datatype",
			policy: &MaskingPolicy{
				Kind:        "MaskingPolicy",
				Name:        "analytics_pii_string_policy",
				DataProduct: "analytics",
				Mask:        "==MASKED==",
				Cases: []Case{
					{Strategy: "UNMASKED", Consumers: []Consumer{{Kind: "consumer_group", Name: "dataverse-source-analytics"}}},
				},
			},
			expectValid: false,
			expectError: "datatype: is required",
		},
		{
			name: "missing mask",
			policy: &MaskingPolicy{
				Kind:        "MaskingPolicy",
				Name:        "analytics_pii_string_policy",
				DataProduct: "analytics",
				DataType:    "string",
				Cases: []Case{
					{Strategy: "UNMASKED", Consumers: []Consumer{{Kind: "consumer_group", Name: "dataverse-source-analytics"}}},
				},
			},
			expectValid: false,
			expectError: "mask: is required",
		},
		{
			name: "empty cases",
			policy: &MaskingPolicy{
				Kind:        "MaskingPolicy",
				Name:        "analytics_pii_string_policy",
				DataProduct: "analytics",
				DataType:    "string",
				Mask:        "==MASKED==",
				Cases:       []Case{},
			},
			expectValid: false,
			expectError: "cases: at least one case is required",
		},
		{
			name: "case without strategy",
			policy: &MaskingPolicy{
				Kind:        "MaskingPolicy",
				Name:        "analytics_pii_string_policy",
				DataProduct: "analytics",
				DataType:    "string",
				Mask:        "==MASKED==",
				Cases: []Case{
					{Consumers: []Consumer{{Kind: "consumer_group", Name: "dataverse-source-analytics"}}},
				},
			},
			expectValid: false,
			expectError: "cases[0].strategy: is required",
		},
		{
			name: "case without consumers",
			policy: &MaskingPolicy{
				Kind:        "MaskingPolicy",
				Name:        "analytics_pii_string_policy",
				DataProduct: "analytics",
				DataType:    "string",
				Mask:        "==MASKED==",
				Cases: []Case{
					{Strategy: "UNMASKED", Consumers: []Consumer{}},
				},
			},
			expectValid: false,
			expectError: "cases[0].consumers: at least one consumer is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := validator.Validate(tt.policy, "", "")

			if tt.expectValid && !result.IsValid {
				t.Errorf("expected valid, got errors: %v", result.GetErrorMessages())
			}

			if !tt.expectValid {
				if result.IsValid {
					t.Errorf("expected invalid, got valid")
				}
				found := false
				for _, msg := range result.GetErrorMessages() {
					if msg == tt.expectError {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("expected error containing '%s', got: %v", tt.expectError, result.GetErrorMessages())
				}
			}
		})
	}
}

func TestValidator_ValidateNamingConvention(t *testing.T) {
	validator := NewValidator()

	tests := []struct {
		name        string
		policyName  string
		expectValid bool
	}{
		{"valid pii string", "analytics_pii_string_policy", true},
		{"valid restricted float", "ciam_restricted_float_policy", true},
		{"valid restrictedpii number", "mydata_restrictedpii_number_policy", true},
		{"valid long name", "bookingsmaster_pii_string_policy", true},
		{"invalid - hyphens", "analytics-pii-string-policy", false},
		{"invalid - wrong classification", "analytics_sensitive_string_policy", false},
		{"invalid - wrong datatype", "analytics_pii_varchar_policy", false},
		{"invalid - missing classification", "analytics_string_policy", false},
		{"invalid - uppercase", "Analytics_PII_String_policy", false},
		{"invalid - too short dataproduct", "ab_pii_string_policy", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			policy := &MaskingPolicy{
				Kind:        "MaskingPolicy",
				Name:        tt.policyName,
				DataProduct: "analytics",
				DataType:    "string",
				Mask:        "==MASKED==",
				Cases: []Case{
					{Strategy: "UNMASKED", Consumers: []Consumer{{Kind: "consumer_group", Name: "dataverse-source-analytics"}}},
				},
			}

			result := validator.Validate(policy, "", "")

			// Check if naming convention error exists
			hasNamingError := false
			for _, err := range result.Errors {
				if err.Field == "name" {
					hasNamingError = true
					break
				}
			}

			if tt.expectValid && hasNamingError {
				t.Errorf("expected valid name, got naming error")
			}
			if !tt.expectValid && !hasNamingError {
				t.Errorf("expected invalid name, but no naming error found")
			}
		})
	}
}

func TestValidator_ValidateDatatype(t *testing.T) {
	validator := NewValidator()

	tests := []struct {
		name        string
		datatype    string
		expectValid bool
	}{
		{"valid string", "string", true},
		{"valid float", "float", true},
		{"valid number", "number", true},
		{"invalid varchar", "varchar", false},
		{"invalid int", "int", false},
		{"invalid text", "text", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			policy := &MaskingPolicy{
				Kind:        "MaskingPolicy",
				Name:        "analytics_pii_" + tt.datatype + "_policy",
				DataProduct: "analytics",
				DataType:    tt.datatype,
				Mask:        "==MASKED==",
				Cases: []Case{
					{Strategy: "UNMASKED", Consumers: []Consumer{{Kind: "consumer_group", Name: "dataverse-source-analytics"}}},
				},
			}

			result := validator.Validate(policy, "", "")

			// Check for datatype error
			hasDatatypeError := false
			for _, err := range result.Errors {
				if err.Field == "datatype" && err.Message != "" {
					hasDatatypeError = true
					break
				}
			}

			if tt.expectValid && hasDatatypeError {
				t.Errorf("expected valid datatype, got error")
			}
			if !tt.expectValid && !hasDatatypeError {
				t.Errorf("expected invalid datatype, but no error found")
			}
		})
	}
}

func TestValidator_ValidateMaskFormat(t *testing.T) {
	validator := NewValidator()

	tests := []struct {
		name        string
		datatype    string
		mask        string
		expectValid bool
	}{
		// String masks
		{"string - valid masked", "string", "==MASKED==", true},
		{"string - valid custom", "string", "***REDACTED***", true},
		{"string - valid simple", "string", "***", true},

		// Float masks
		{"float - valid negative decimal", "float", "-9.0", true},
		{"float - valid zero", "float", "0.0", true},
		{"float - valid positive", "float", "123.45", true},
		{"float - invalid string", "float", "MASKED", false},
		{"float - invalid empty", "float", "", false},

		// Number masks
		{"number - valid integer", "number", "8888", true},
		{"number - valid negative", "number", "-9", true},
		{"number - valid zero", "number", "0", true},
		{"number - invalid decimal", "number", "88.8", false},
		{"number - invalid string", "number", "MASKED", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			policy := &MaskingPolicy{
				Kind:        "MaskingPolicy",
				Name:        "analytics_pii_" + tt.datatype + "_policy",
				DataProduct: "analytics",
				DataType:    tt.datatype,
				Mask:        tt.mask,
				Cases: []Case{
					{Strategy: "UNMASKED", Consumers: []Consumer{{Kind: "consumer_group", Name: "dataverse-source-analytics"}}},
				},
			}

			result := validator.Validate(policy, "", "")

			// Check for mask error
			hasMaskError := false
			for _, err := range result.Errors {
				if err.Field == "mask" {
					hasMaskError = true
					break
				}
			}

			if tt.expectValid && hasMaskError {
				t.Errorf("expected valid mask, got error: %v", result.GetErrorMessages())
			}
			if !tt.expectValid && !hasMaskError {
				t.Errorf("expected invalid mask, but no error found")
			}
		})
	}
}

func TestValidator_ValidateStrategies(t *testing.T) {
	validator := NewValidator()

	tests := []struct {
		name        string
		datatype    string
		strategy    string
		expectValid bool
	}{
		// UNMASKED works with all datatypes
		{"UNMASKED with string", "string", "UNMASKED", true},
		{"UNMASKED with float", "float", "UNMASKED", true},
		{"UNMASKED with number", "number", "UNMASKED", true},

		// HASH_SHA1 only works with string
		{"HASH_SHA1 with string", "string", "HASH_SHA1", true},
		{"HASH_SHA1 with float - invalid", "float", "HASH_SHA1", false},
		{"HASH_SHA1 with number - invalid", "number", "HASH_SHA1", false},

		// Invalid strategy
		{"invalid strategy MASK", "string", "MASK", false},
		// Note: lowercase "unmasked" is valid because validator does ToUpper comparison
		{"lowercase unmasked - valid", "string", "unmasked", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var mask string
			switch tt.datatype {
			case "float":
				mask = "-9.0"
			case "number":
				mask = "8888"
			default:
				mask = "==MASKED=="
			}

			policy := &MaskingPolicy{
				Kind:        "MaskingPolicy",
				Name:        "analytics_pii_" + tt.datatype + "_policy",
				DataProduct: "analytics",
				DataType:    tt.datatype,
				Mask:        mask,
				Cases: []Case{
					{Strategy: tt.strategy, Consumers: []Consumer{{Kind: "consumer_group", Name: "dataverse-source-analytics"}}},
				},
			}

			result := validator.Validate(policy, "", "")

			// Check for strategy error
			hasStrategyError := false
			for _, err := range result.Errors {
				if err.Field == "cases[0].strategy" {
					hasStrategyError = true
					break
				}
			}

			if tt.expectValid && hasStrategyError {
				t.Errorf("expected valid strategy, got error: %v", result.GetErrorMessages())
			}
			if !tt.expectValid && !hasStrategyError {
				t.Errorf("expected invalid strategy, but no error found")
			}
		})
	}
}

func TestValidator_ValidateStrategyOrdering(t *testing.T) {
	validator := NewValidator()

	tests := []struct {
		name        string
		cases       []Case
		expectValid bool
	}{
		{
			name: "correct order - UNMASKED first",
			cases: []Case{
				{Strategy: "UNMASKED", Consumers: []Consumer{{Kind: "consumer_group", Name: "dataverse-source-analytics"}}},
				{Strategy: "HASH_SHA1", Consumers: []Consumer{{Kind: "consumer_group", Name: "dataverse-aggregate-other"}}},
			},
			expectValid: true,
		},
		{
			name: "wrong order - HASH_SHA1 first",
			cases: []Case{
				{Strategy: "HASH_SHA1", Consumers: []Consumer{{Kind: "consumer_group", Name: "dataverse-aggregate-other"}}},
				{Strategy: "UNMASKED", Consumers: []Consumer{{Kind: "consumer_group", Name: "dataverse-source-analytics"}}},
			},
			expectValid: false,
		},
		{
			name: "only UNMASKED - valid",
			cases: []Case{
				{Strategy: "UNMASKED", Consumers: []Consumer{{Kind: "consumer_group", Name: "dataverse-source-analytics"}}},
			},
			expectValid: true,
		},
		{
			name: "only HASH_SHA1 - valid",
			cases: []Case{
				{Strategy: "HASH_SHA1", Consumers: []Consumer{{Kind: "consumer_group", Name: "dataverse-aggregate-other"}}},
			},
			expectValid: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			policy := &MaskingPolicy{
				Kind:        "MaskingPolicy",
				Name:        "analytics_pii_string_policy",
				DataProduct: "analytics",
				DataType:    "string",
				Mask:        "==MASKED==",
				Cases:       tt.cases,
			}

			result := validator.Validate(policy, "", "")

			// Check for ordering error
			hasOrderingError := false
			for _, err := range result.Errors {
				if err.Field == "cases" && err.Message != "" {
					hasOrderingError = true
					break
				}
			}

			if tt.expectValid && hasOrderingError {
				t.Errorf("expected valid ordering, got error: %v", result.GetErrorMessages())
			}
			if !tt.expectValid && !hasOrderingError {
				t.Errorf("expected invalid ordering, but no error found")
			}
		})
	}
}

func TestValidator_ValidateConsumerKind(t *testing.T) {
	validator := NewValidator()

	tests := []struct {
		name        string
		kind        string
		expectValid bool
	}{
		{"valid consumer_group", "consumer_group", true},
		{"valid service_account", "service_account", true},
		{"invalid user", "user", false},
		{"invalid role", "role", false},
		{"invalid empty", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			consumerName := "dataverse-source-analytics"
			if tt.kind == "service_account" {
				consumerName = "ciam_dbt_sandbox_appuser"
			}

			policy := &MaskingPolicy{
				Kind:        "MaskingPolicy",
				Name:        "analytics_pii_string_policy",
				DataProduct: "analytics",
				DataType:    "string",
				Mask:        "==MASKED==",
				Cases: []Case{
					{Strategy: "UNMASKED", Consumers: []Consumer{{Kind: tt.kind, Name: consumerName}}},
				},
			}

			result := validator.Validate(policy, "", "sandbox")

			// Check for consumer kind error
			hasKindError := false
			for _, err := range result.Errors {
				if err.Field == "cases[0].consumers[0].kind" {
					hasKindError = true
					break
				}
			}

			if tt.expectValid && hasKindError {
				t.Errorf("expected valid kind, got error: %v", result.GetErrorMessages())
			}
			if !tt.expectValid && tt.kind != "" && !hasKindError {
				t.Errorf("expected invalid kind, but no error found")
			}
		})
	}
}

func TestValidator_ValidateConsumerGroupName(t *testing.T) {
	validator := NewValidator()

	tests := []struct {
		name         string
		consumerName string
		expectValid  bool
	}{
		{"valid source", "dataverse-source-analytics", true},
		{"valid aggregate", "dataverse-aggregate-bookingsmaster", true},
		{"valid consumer", "dataverse-consumer-mydata", true},
		{"valid with suffix", "dataverse-source-analytics-team", true},
		{"invalid - no prefix", "analytics-team", false},
		{"invalid - wrong prefix", "mydataverse-source-analytics", false},
		{"invalid - underscores", "dataverse_source_analytics", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			policy := &MaskingPolicy{
				Kind:        "MaskingPolicy",
				Name:        "analytics_pii_string_policy",
				DataProduct: "analytics",
				DataType:    "string",
				Mask:        "==MASKED==",
				Cases: []Case{
					{Strategy: "UNMASKED", Consumers: []Consumer{{Kind: "consumer_group", Name: tt.consumerName}}},
				},
			}

			result := validator.Validate(policy, "", "")

			// Check for consumer name error
			hasNameError := false
			for _, err := range result.Errors {
				if err.Field == "cases[0].consumers[0].name" {
					hasNameError = true
					break
				}
			}

			if tt.expectValid && hasNameError {
				t.Errorf("expected valid name, got error: %v", result.GetErrorMessages())
			}
			if !tt.expectValid && !hasNameError {
				t.Errorf("expected invalid name, but no error found")
			}
		})
	}
}

func TestValidator_ValidateServiceAccountName(t *testing.T) {
	validator := NewValidator()

	tests := []struct {
		name        string
		saName      string
		environment string
		expectValid bool
	}{
		{"valid sandbox", "ciam_dbt_sandbox_appuser", "sandbox", true},
		{"valid preprod", "concur_tableau_preprod_appuser", "preprod", true},
		{"valid prod", "analytics_etl_prod_appuser", "prod", true},
		{"invalid - no _appuser suffix", "ciam_dbt_sandbox", "", false},
		{"invalid - wrong env (expects sandbox)", "ciam_dbt_prod_appuser", "sandbox", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			policy := &MaskingPolicy{
				Kind:        "MaskingPolicy",
				Name:        "analytics_pii_string_policy",
				DataProduct: "analytics",
				DataType:    "string",
				Mask:        "==MASKED==",
				Cases: []Case{
					{Strategy: "UNMASKED", Consumers: []Consumer{{Kind: "service_account", Name: tt.saName}}},
				},
			}

			result := validator.Validate(policy, "", tt.environment)

			// Check for consumer name error
			hasNameError := false
			for _, err := range result.Errors {
				if err.Field == "cases[0].consumers[0].name" {
					hasNameError = true
					break
				}
			}

			if tt.expectValid && hasNameError {
				t.Errorf("expected valid name, got error: %v", result.GetErrorMessages())
			}
			if !tt.expectValid && !hasNameError {
				t.Errorf("expected invalid name, but no error found")
			}
		})
	}
}

func TestValidator_ValidateDuplicateConsumers(t *testing.T) {
	validator := NewValidator()

	tests := []struct {
		name        string
		cases       []Case
		expectValid bool
	}{
		{
			name: "no duplicates",
			cases: []Case{
				{Strategy: "UNMASKED", Consumers: []Consumer{
					{Kind: "consumer_group", Name: "dataverse-source-analytics"},
				}},
				{Strategy: "HASH_SHA1", Consumers: []Consumer{
					{Kind: "consumer_group", Name: "dataverse-aggregate-other"},
				}},
			},
			expectValid: true,
		},
		{
			name: "duplicate in different cases",
			cases: []Case{
				{Strategy: "UNMASKED", Consumers: []Consumer{
					{Kind: "consumer_group", Name: "dataverse-source-analytics"},
				}},
				{Strategy: "HASH_SHA1", Consumers: []Consumer{
					{Kind: "consumer_group", Name: "dataverse-source-analytics"},
				}},
			},
			expectValid: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			policy := &MaskingPolicy{
				Kind:        "MaskingPolicy",
				Name:        "analytics_pii_string_policy",
				DataProduct: "analytics",
				DataType:    "string",
				Mask:        "==MASKED==",
				Cases:       tt.cases,
			}

			result := validator.Validate(policy, "", "")

			// Check for duplicate error
			hasDuplicateError := false
			for _, err := range result.Errors {
				if err.Field == "consumers" {
					hasDuplicateError = true
					break
				}
			}

			if tt.expectValid && hasDuplicateError {
				t.Errorf("expected valid (no duplicates), got error: %v", result.GetErrorMessages())
			}
			if !tt.expectValid && !hasDuplicateError {
				t.Errorf("expected duplicate error, but none found")
			}
		})
	}
}

func TestValidator_ValidateDataProductPath(t *testing.T) {
	validator := NewValidator()

	tests := []struct {
		name            string
		dataProduct     string
		dataProductPath string
		expectValid     bool
	}{
		{"matching path", "analytics", "analytics", true},
		{"matching case-insensitive", "Analytics", "analytics", true},
		{"mismatched path", "ciam", "analytics", false},
		{"empty path (skip check)", "analytics", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			policy := &MaskingPolicy{
				Kind:        "MaskingPolicy",
				Name:        "analytics_pii_string_policy",
				DataProduct: tt.dataProduct,
				DataType:    "string",
				Mask:        "==MASKED==",
				Cases: []Case{
					{Strategy: "UNMASKED", Consumers: []Consumer{{Kind: "consumer_group", Name: "dataverse-source-analytics"}}},
				},
			}

			result := validator.Validate(policy, tt.dataProductPath, "")

			// Check for path mismatch error
			hasPathError := false
			for _, err := range result.Errors {
				if err.Field == "data_product" {
					hasPathError = true
					break
				}
			}

			if tt.expectValid && hasPathError {
				t.Errorf("expected valid path, got error: %v", result.GetErrorMessages())
			}
			if !tt.expectValid && !hasPathError {
				t.Errorf("expected path error, but none found")
			}
		})
	}
}
