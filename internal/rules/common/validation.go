package common

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/redhat-data-and-ai/naysayer/internal/rules/shared"
)

// ValidationHelper provides common validation utilities
type ValidationHelper struct{}

// NewValidationHelper creates a new validation helper
func NewValidationHelper() *ValidationHelper {
	return &ValidationHelper{}
}

// ValidateEmail checks if an email address is valid and from approved domains
func (v *ValidationHelper) ValidateEmail(email string, approvedDomains []string) error {
	if email == "" {
		return fmt.Errorf("email cannot be empty")
	}

	// Basic email format validation
	emailRegex := regexp.MustCompile(`^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`)
	if !emailRegex.MatchString(email) {
		return fmt.Errorf("invalid email format: %s", email)
	}

	// Check approved domains
	if len(approvedDomains) > 0 {
		domain := strings.Split(email, "@")[1]
		for _, approvedDomain := range approvedDomains {
			if strings.EqualFold(domain, approvedDomain) {
				return nil
			}
		}
		return fmt.Errorf("email domain %s not in approved domains: %v", domain, approvedDomains)
	}

	return nil
}

// ValidateRole checks if a role is in the list of valid roles
func (v *ValidationHelper) ValidateRole(role string, validRoles []string) error {
	if role == "" {
		return fmt.Errorf("role cannot be empty")
	}

	for _, validRole := range validRoles {
		if strings.EqualFold(role, validRole) {
			return nil
		}
	}

	return fmt.Errorf("invalid role %s, valid roles are: %v", role, validRoles)
}

// ValidateRequiredFields checks that all required fields are present in content
func (v *ValidationHelper) ValidateRequiredFields(content string, requiredFields []string) []string {
	var missingFields []string

	for _, field := range requiredFields {
		if !strings.Contains(content, field+":") {
			missingFields = append(missingFields, field)
		}
	}

	return missingFields
}

// CreateApprovalResult creates a standard approval result
func (v *ValidationHelper) CreateApprovalResult(reason string) (shared.DecisionType, string) {
	return shared.Approve, reason
}

// CreateManualReviewResult creates a standard manual review result
func (v *ValidationHelper) CreateManualReviewResult(reason string) (shared.DecisionType, string) {
	return shared.ManualReview, reason
}

// Note: Global validation helper removed - use NewValidationHelper() directly
