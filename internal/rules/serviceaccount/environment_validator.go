package serviceaccount

import (
	"fmt"
	"strings"
)

// EnvironmentValidator handles environment-specific validation
type EnvironmentValidator struct {
	astroRestrictedEnvs []string
}

// NewEnvironmentValidator creates a new environment validator
func NewEnvironmentValidator() *EnvironmentValidator {
	return &EnvironmentValidator{
		astroRestrictedEnvs: []string{"preprod", "prod"},
	}
}

// ValidateServiceAccountForEnvironment validates service account against environment rules
func (v *EnvironmentValidator) ValidateServiceAccountForEnvironment(sa ServiceAccount, saFile ServiceAccountFile) []ValidationIssue {
	var issues []ValidationIssue

	// Rule: Astro service accounts only for PreProd and Prod
	if v.isAstroServiceAccount(saFile) {
		if !v.isRestrictedEnvironment(saFile.Environment) {
			issues = append(issues, ValidationIssue{
				Type:       "environment",
				Severity:   "error",
				Message:    "Astro service accounts are only allowed in preprod and prod environments",
				Field:      "name",
				Value:      sa.Name,
				Suggestion: fmt.Sprintf("Use a different integration type for %s environment, or deploy to preprod/prod", saFile.Environment),
			})
		}
	}

	return issues
}

// isAstroServiceAccount checks if this is an Astro service account based on filename
func (v *EnvironmentValidator) isAstroServiceAccount(saFile ServiceAccountFile) bool {
	return strings.Contains(strings.ToLower(saFile.Integration), "astro")
}

// isRestrictedEnvironment checks if environment allows Astro service accounts
func (v *EnvironmentValidator) isRestrictedEnvironment(env string) bool {
	envLower := strings.ToLower(env)
	for _, restrictedEnv := range v.astroRestrictedEnvs {
		if envLower == restrictedEnv {
			return true
		}
	}
	return false
}