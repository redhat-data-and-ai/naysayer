package serviceaccount

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestEnvironmentValidator_ValidateServiceAccountForEnvironment(t *testing.T) {
	validator := NewEnvironmentValidator()

	tests := []struct {
		name        string
		integration string
		environment string
		expectValid bool
	}{
		{"astro in prod - valid", "astro", "prod", true},
		{"astro in preprod - valid", "astro", "preprod", true},
		{"astro in dev - invalid", "astro", "dev", false},
		{"tableau in dev - valid", "tableau", "dev", true},
		{"operator in any environment - valid", "operator", "dev", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sa := ServiceAccount{Name: "test_" + tt.integration + "_" + tt.environment + "_appuser"}
			saFile := ServiceAccountFile{Environment: tt.environment, Integration: tt.integration}
			issues := validator.ValidateServiceAccountForEnvironment(sa, saFile)

			if tt.expectValid {
				assert.Empty(t, issues, "Expected no issues")
			} else {
				assert.Equal(t, "environment", issues[0].Type)
				assert.Equal(t, "error", issues[0].Severity)
			}
		})
	}
}

func TestEnvironmentValidator_AstroDetection(t *testing.T) {
	validator := NewEnvironmentValidator()

	// Test case-insensitive astro detection
	assert.True(t, validator.isAstroServiceAccount(ServiceAccountFile{Integration: "astro"}))
	assert.True(t, validator.isAstroServiceAccount(ServiceAccountFile{Integration: "ASTRO"}))
	assert.False(t, validator.isAstroServiceAccount(ServiceAccountFile{Integration: "tableau"}))
	assert.False(t, validator.isAstroServiceAccount(ServiceAccountFile{Integration: "operator"}))

	// Test environment restrictions
	assert.True(t, validator.isRestrictedEnvironment("prod"))
	assert.True(t, validator.isRestrictedEnvironment("preprod"))
	assert.False(t, validator.isRestrictedEnvironment("dev"))
	assert.False(t, validator.isRestrictedEnvironment("sandbox"))
}