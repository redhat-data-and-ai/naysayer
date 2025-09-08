package common

import (
	"testing"

	"github.com/redhat-data-and-ai/naysayer/internal/rules/shared"
	"github.com/stretchr/testify/assert"
)

func TestNewValidationHelper(t *testing.T) {
	helper := NewValidationHelper()
	assert.NotNil(t, helper)
}

func TestValidationHelper_ValidateEmail(t *testing.T) {
	helper := NewValidationHelper()

	tests := []struct {
		name            string
		email           string
		approvedDomains []string
		expectError     bool
		errorContains   string
	}{
		// Valid emails
		{"valid email no domain check", "user@example.com", nil, false, ""},
		{"valid email with approved domain", "user@redhat.com", []string{"redhat.com"}, false, ""},
		{"valid email multiple approved domains", "test@gmail.com", []string{"redhat.com", "gmail.com", "yahoo.com"}, false, ""},
		{"complex valid email", "john.doe+test@company.co.uk", []string{"company.co.uk"}, false, ""},

		// Invalid format
		{"empty email", "", nil, true, "email cannot be empty"},
		{"missing @", "userexample.com", nil, true, "invalid email format"},
		{"missing domain", "user@", nil, true, "invalid email format"},
		{"missing username", "@example.com", nil, true, "invalid email format"},
		{"multiple @", "user@@example.com", nil, true, "invalid email format"},
		{"invalid characters", "user name@example.com", nil, true, "invalid email format"},
		{"missing TLD", "user@example", nil, true, "invalid email format"},

		// Domain validation
		{"unapproved domain", "user@badexample.com", []string{"redhat.com", "gmail.com"}, true, "not in approved domains"},
		{"case insensitive domain check", "user@REDHAT.COM", []string{"redhat.com"}, false, ""},
		{"mixed case domain", "user@RedHat.Com", []string{"redhat.com"}, false, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := helper.ValidateEmail(tt.email, tt.approvedDomains)

			if tt.expectError {
				assert.Error(t, err)
				if tt.errorContains != "" {
					assert.Contains(t, err.Error(), tt.errorContains)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidationHelper_ValidateRole(t *testing.T) {
	helper := NewValidationHelper()

	tests := []struct {
		name          string
		role          string
		validRoles    []string
		expectError   bool
		errorContains string
	}{
		// Valid roles
		{"valid role exact match", "admin", []string{"admin", "user", "viewer"}, false, ""},
		{"valid role case insensitive", "ADMIN", []string{"admin", "user", "viewer"}, false, ""},
		{"valid role mixed case", "Admin", []string{"admin", "user", "viewer"}, false, ""},
		{"valid role from multiple options", "viewer", []string{"admin", "user", "viewer"}, false, ""},

		// Invalid roles
		{"empty role", "", []string{"admin", "user"}, true, "role cannot be empty"},
		{"invalid role", "superuser", []string{"admin", "user", "viewer"}, true, "invalid role superuser"},
		{"role not in list", "guest", []string{"admin", "user"}, true, "invalid role guest"},

		// Edge cases
		{"empty valid roles list", "admin", []string{}, true, "invalid role admin"},
		{"nil valid roles list", "admin", nil, true, "invalid role admin"},
		{"role with spaces", "admin user", []string{"admin", "user"}, true, "invalid role admin user"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := helper.ValidateRole(tt.role, tt.validRoles)

			if tt.expectError {
				assert.Error(t, err)
				if tt.errorContains != "" {
					assert.Contains(t, err.Error(), tt.errorContains)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidationHelper_ValidateRequiredFields(t *testing.T) {
	helper := NewValidationHelper()

	tests := []struct {
		name            string
		content         string
		requiredFields  []string
		expectedMissing []string
	}{
		{
			name:            "all fields present",
			content:         "name: test\nversion: 1.0\ndescription: Test service",
			requiredFields:  []string{"name", "version", "description"},
			expectedMissing: nil,
		},
		{
			name:            "some fields missing",
			content:         "name: test\ndescription: Test service",
			requiredFields:  []string{"name", "version", "description"},
			expectedMissing: []string{"version"},
		},
		{
			name:            "all fields missing",
			content:         "other: value\nconfig: settings",
			requiredFields:  []string{"name", "version"},
			expectedMissing: []string{"name", "version"},
		},
		{
			name:            "no required fields",
			content:         "name: test\nversion: 1.0",
			requiredFields:  []string{},
			expectedMissing: nil,
		},
		{
			name:            "empty content",
			content:         "",
			requiredFields:  []string{"name", "version"},
			expectedMissing: []string{"name", "version"},
		},
		{
			name:            "fields with complex values",
			content:         "database:\n  host: localhost\n  port: 5432\nauth:\n  enabled: true",
			requiredFields:  []string{"database", "auth"},
			expectedMissing: nil,
		},
		{
			name:            "partial field name should not match",
			content:         "username: test\npassword_hash: abc123",
			requiredFields:  []string{"user", "password"},
			expectedMissing: []string{"user", "password"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			missing := helper.ValidateRequiredFields(tt.content, tt.requiredFields)
			assert.Equal(t, tt.expectedMissing, missing)
		})
	}
}

func TestValidationHelper_CreateApprovalResult(t *testing.T) {
	helper := NewValidationHelper()

	tests := []struct {
		name   string
		reason string
	}{
		{"simple approval", "All checks passed"},
		{"detailed approval", "Auto-approved: All warehouse configurations are valid and within limits"},
		{"empty reason", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			decision, reason := helper.CreateApprovalResult(tt.reason)
			assert.Equal(t, shared.Approve, decision)
			assert.Equal(t, tt.reason, reason)
		})
	}
}

func TestValidationHelper_CreateManualReviewResult(t *testing.T) {
	helper := NewValidationHelper()

	tests := []struct {
		name   string
		reason string
	}{
		{"simple manual review", "Manual review required"},
		{"detailed manual review", "Manual review required: Warehouse size increase detected from SMALL to LARGE"},
		{"empty reason", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			decision, reason := helper.CreateManualReviewResult(tt.reason)
			assert.Equal(t, shared.ManualReview, decision)
			assert.Equal(t, tt.reason, reason)
		})
	}
}

func TestValidationHelper_Integration(t *testing.T) {
	helper := NewValidationHelper()

	// Test a complete validation workflow
	t.Run("complete_validation_workflow", func(t *testing.T) {
		// Test email validation
		err := helper.ValidateEmail("admin@redhat.com", []string{"redhat.com", "ibm.com"})
		assert.NoError(t, err)

		// Test role validation
		err = helper.ValidateRole("admin", []string{"admin", "user", "viewer"})
		assert.NoError(t, err)

		// Test required fields
		content := `
name: test-service
version: 1.0.0
description: Test service for validation
admin_email: admin@redhat.com
role: admin
`
		missing := helper.ValidateRequiredFields(content, []string{"name", "version", "admin_email"})
		assert.Empty(t, missing)

		// Test result creation
		approveDecision, approveReason := helper.CreateApprovalResult("All validations passed")
		assert.Equal(t, shared.Approve, approveDecision)
		assert.Equal(t, "All validations passed", approveReason)

		reviewDecision, reviewReason := helper.CreateManualReviewResult("Some validation failed")
		assert.Equal(t, shared.ManualReview, reviewDecision)
		assert.Equal(t, "Some validation failed", reviewReason)
	})
}

func TestValidationHelper_RealWorldScenarios(t *testing.T) {
	helper := NewValidationHelper()

	t.Run("dataproduct_validation", func(t *testing.T) {
		// Simulate validating a data product configuration
		productContent := `
name: analytics-platform
version: 2.1.0
description: Analytics data product for business intelligence
maintainer_email: analytics-team@redhat.com
owner_role: admin
warehouses:
  - type: user
    size: SMALL
  - type: loader  
    size: MEDIUM
`

		// Validate maintainer email
		err := helper.ValidateEmail("analytics-team@redhat.com", []string{"redhat.com", "ibm.com"})
		assert.NoError(t, err)

		// Validate owner role
		err = helper.ValidateRole("admin", []string{"admin", "maintainer", "viewer"})
		assert.NoError(t, err)

		// Check required fields
		requiredFields := []string{"name", "version", "description", "maintainer_email", "warehouses"}
		missing := helper.ValidateRequiredFields(productContent, requiredFields)
		assert.Empty(t, missing)

		// Create approval
		decision, reason := helper.CreateApprovalResult("Data product configuration is valid")
		assert.Equal(t, shared.Approve, decision)
		assert.Contains(t, reason, "valid")
	})

	t.Run("invalid_configuration", func(t *testing.T) {
		// Simulate an invalid configuration
		invalidContent := `
name: test-product
# missing version and other required fields
`

		// Check for missing fields
		requiredFields := []string{"name", "version", "description", "maintainer_email"}
		missing := helper.ValidateRequiredFields(invalidContent, requiredFields)
		assert.Contains(t, missing, "version")
		assert.Contains(t, missing, "description")
		assert.Contains(t, missing, "maintainer_email")

		// Create manual review result
		decision, reason := helper.CreateManualReviewResult("Missing required fields: " +
			"version, description, maintainer_email")
		assert.Equal(t, shared.ManualReview, decision)
		assert.Contains(t, reason, "Missing required fields")
	})
}
