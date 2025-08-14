package serviceaccount

import (
	"fmt"
	"regexp"
	"strings"
)

// EmailValidator handles email validation logic
type EmailValidator struct {
	allowedDomains       []string
	groupEmailPatterns   []*regexp.Regexp
	individualEmailRegex *regexp.Regexp
}

// NewEmailValidator creates a new email validator
func NewEmailValidator(allowedDomains []string) *EmailValidator {
	// Common group email patterns found in the existing files
	groupPatterns := []*regexp.Regexp{
		regexp.MustCompile(`.*-team@.*`),
		regexp.MustCompile(`.*-group@.*`),
		regexp.MustCompile(`.*-list@.*`),
		regexp.MustCompile(`.*-notifications@.*`),
		regexp.MustCompile(`noreply@.*`),
		regexp.MustCompile(`.*-bot@.*`),
		regexp.MustCompile(`.*-service@.*`),
		regexp.MustCompile(`.*-platform@.*`), // Found: dataverse-platform@redhat.com
		regexp.MustCompile(`.*-platform-team@.*`), // Found: dataverse-platform-team@redhat.com
	}

	// Individual email pattern (basic validation)
	individualRegex := regexp.MustCompile(`^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`)

	return &EmailValidator{
		allowedDomains:       allowedDomains,
		groupEmailPatterns:   groupPatterns,
		individualEmailRegex: individualRegex,
	}
}

// ValidateEmail validates that an email is individual and from allowed domain
func (v *EmailValidator) ValidateEmail(email string) ValidationIssue {
	email = strings.TrimSpace(email)
	
	// Basic format validation
	if !v.individualEmailRegex.MatchString(email) {
		return ValidationIssue{
			Type:       "email",
			Severity:   "error",
			Message:    "Invalid email format",
			Field:      "email",
			Value:      email,
			Suggestion: "Use a valid email format like 'user@domain.com'",
		}
	}

	// Check for group email patterns
	for _, pattern := range v.groupEmailPatterns {
		if pattern.MatchString(strings.ToLower(email)) {
			return ValidationIssue{
				Type:       "email",
				Severity:   "error",
				Message:    "Group email addresses are not allowed for service accounts",
				Field:      "email",
				Value:      email,
				Suggestion: "Use an individual's email address for compliance and ownership tracking",
			}
		}
	}

	// Check domain restrictions
	if len(v.allowedDomains) > 0 {
		emailParts := strings.Split(email, "@")
		if len(emailParts) != 2 {
			return ValidationIssue{
				Type:       "email",
				Severity:   "error",
				Message:    "Invalid email format",
				Field:      "email",
				Value:      email,
				Suggestion: "Use a valid email format like 'user@domain.com'",
			}
		}
		
		emailDomain := strings.ToLower(emailParts[1])
		allowed := false
		for _, domain := range v.allowedDomains {
			if emailDomain == strings.ToLower(domain) {
				allowed = true
				break
			}
		}
		if !allowed {
			return ValidationIssue{
				Type:       "email",
				Severity:   "error",
				Message:    "Email domain not allowed",
				Field:      "email",
				Value:      email,
				Suggestion: fmt.Sprintf("Use an email from allowed domains: %v", v.allowedDomains),
			}
		}
	}

	// Valid email
	return ValidationIssue{}
}