package serviceaccount

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNamingValidator_ValidateNaming(t *testing.T) {
	validator := NewNamingValidator()

	tests := []struct {
		name      string
		sa        ServiceAccount
		saFile    ServiceAccountFile
		expectValid bool
	}{
		{
			name: "perfect naming convention",
			sa: ServiceAccount{Name: "marketo_astro_prod_appuser"},
			saFile: ServiceAccountFile{
				Path: "serviceaccounts/prod/marketo_astro_prod_appuser.yaml",
				DataProduct: "marketo", Environment: "prod", Integration: "astro",
			},
			expectValid: true,
		},
		{
			name: "invalid characters - uppercase",
			sa: ServiceAccount{Name: "Marketo_Astro_Prod_Appuser"},
			saFile: ServiceAccountFile{Path: "serviceaccounts/prod/Marketo_Astro_Prod_Appuser.yaml"},
			expectValid: false,
		},
		{
			name: "invalid characters - hyphens",
			sa: ServiceAccount{Name: "marketo-astro-prod-appuser"},
			saFile: ServiceAccountFile{Path: "serviceaccounts/prod/marketo-astro-prod-appuser.yaml"},
			expectValid: false,
		},
		{
			name: "filename doesn't match name",
			sa: ServiceAccount{Name: "marketo_astro_prod_appuser"},
			saFile: ServiceAccountFile{Path: "serviceaccounts/prod/wrong_filename.yaml"},
			expectValid: false,
		},
		{
			name: "starts with underscore - invalid",
			sa: ServiceAccount{Name: "_marketo_astro_prod_appuser"},
			saFile: ServiceAccountFile{Path: "serviceaccounts/prod/_marketo_astro_prod_appuser.yaml"},
			expectValid: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			issues := validator.ValidateNaming(tt.sa, tt.saFile)
			hasErrors := false
			for _, issue := range issues {
				if issue.Severity == "error" {
					hasErrors = true
					break
				}
			}

			if tt.expectValid {
				assert.False(t, hasErrors, "Expected valid naming")
			} else {
				assert.True(t, hasErrors, "Expected naming errors")
			}
		})
	}
}

func TestNamingValidator_RegexPatterns(t *testing.T) {
	validator := NewNamingValidator()

	// Test valid patterns
	validNames := []string{"marketo_astro_prod_appuser", "dataverse_operator_dev_appuser", "a_b_c_d"}
	for _, name := range validNames {
		sa := ServiceAccount{Name: name}
		saFile := ServiceAccountFile{Path: "serviceaccounts/test/" + name + ".yaml", DataProduct: "unknown"}
		issues := validator.ValidateNaming(sa, saFile)
		
		hasFormatError := false
		for _, issue := range issues {
			if issue.Severity == "error" && strings.Contains(issue.Message, "naming convention") {
				hasFormatError = true
				break
			}
		}
		assert.False(t, hasFormatError, "Valid name '%s' should not have format errors", name)
	}

	// Test invalid patterns
	invalidNames := []string{"HAS_UPPERCASE", "has-hyphens", "has spaces", "_starts_with_underscore"}
	for _, name := range invalidNames {
		sa := ServiceAccount{Name: name}
		saFile := ServiceAccountFile{Path: "serviceaccounts/test/" + name + ".yaml", DataProduct: "unknown"}
		issues := validator.ValidateNaming(sa, saFile)
		
		hasFormatError := false
		for _, issue := range issues {
			if issue.Severity == "error" && strings.Contains(issue.Message, "naming convention") {
				hasFormatError = true
				break
				}
		}
		assert.True(t, hasFormatError, "Invalid name '%s' should have format errors", name)
	}
}