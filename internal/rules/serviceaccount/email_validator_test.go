package serviceaccount

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestEmailValidator_ValidateEmail(t *testing.T) {
	validator := NewEmailValidator([]string{"redhat.com"})

	tests := []struct {
		name    string
		email   string
		isValid bool
	}{
		{"valid individual email", "john.doe@redhat.com", true},
		{"group email with team suffix", "platform-team@redhat.com", false},
		{"group email with bot suffix", "deployment-bot@redhat.com", false},
		{"invalid domain", "john.doe@external.com", false},
		{"noreply email", "noreply@redhat.com", false},
		{"empty email", "", false},
		{"malformed email", "notanemail", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			issue := validator.ValidateEmail(tt.email)
			if tt.isValid {
				assert.Empty(t, issue.Type, "Expected valid email")
			} else {
				assert.Equal(t, "email", issue.Type)
				assert.Equal(t, "error", issue.Severity)
			}
		})
	}
}

func TestEmailValidator_DomainRestrictions(t *testing.T) {
	// Test with domain restrictions
	validator := NewEmailValidator([]string{"redhat.com"})
	issue := validator.ValidateEmail("user@external.com")
	assert.Equal(t, "email", issue.Type, "Should reject external domains")

	// Test without domain restrictions
	validatorNoDomain := NewEmailValidator([]string{})
	issue = validatorNoDomain.ValidateEmail("user@any-domain.org")
	assert.Empty(t, issue.Type, "Should accept any domain when no restrictions")

	// Group emails still blocked regardless of domain restrictions
	issue = validatorNoDomain.ValidateEmail("team-group@example.com")
	assert.Equal(t, "email", issue.Type, "Group emails should always be blocked")
}