package errors

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// ValidationError represents a field-specific validation error
type ValidationError struct {
	Field   string `json:"field"`
	Value   string `json:"value,omitempty"`
	Rule    string `json:"rule"`
	Message string `json:"message"`
}

// Validator provides common validation functions that return AppErrors
type Validator struct {
	errors []ValidationError
}

// NewValidator creates a new validator instance
func NewValidator() *Validator {
	return &Validator{
		errors: make([]ValidationError, 0),
	}
}

// AddError adds a validation error
func (v *Validator) AddError(field, rule, message string, value ...interface{}) {
	var valueStr string
	if len(value) > 0 {
		valueStr = fmt.Sprintf("%v", value[0])
	}

	v.errors = append(v.errors, ValidationError{
		Field:   field,
		Value:   valueStr,
		Rule:    rule,
		Message: message,
	})
}

// HasErrors returns true if there are validation errors
func (v *Validator) HasErrors() bool {
	return len(v.errors) > 0
}

// GetErrors returns all validation errors
func (v *Validator) GetErrors() []ValidationError {
	return v.errors
}

// ToAppError converts validation errors to an AppError
func (v *Validator) ToAppError() *AppError {
	if !v.HasErrors() {
		return nil
	}

	var messages []string
	for _, err := range v.errors {
		messages = append(messages, fmt.Sprintf("%s: %s", err.Field, err.Message))
	}

	appErr := NewError(ErrValidationFailed, "Validation failed")
	appErr.Details = strings.Join(messages, "; ")
	_ = appErr.WithContext("validation_errors", v.errors)

	return appErr
}

// RequiredField validates that a field is not empty
func (v *Validator) RequiredField(field, value string) *Validator {
	if strings.TrimSpace(value) == "" {
		v.AddError(field, "required", "Field is required", value)
	}
	return v
}

// MinLength validates minimum string length
func (v *Validator) MinLength(field, value string, min int) *Validator {
	if len(value) < min {
		v.AddError(field, "min_length",
			fmt.Sprintf("Must be at least %d characters long", min), value)
	}
	return v
}

// MaxLength validates maximum string length
func (v *Validator) MaxLength(field, value string, max int) *Validator {
	if len(value) > max {
		v.AddError(field, "max_length",
			fmt.Sprintf("Must be at most %d characters long", max), value)
	}
	return v
}

// ValidateEmail validates email format
func (v *Validator) ValidateEmail(field, email string) *Validator {
	if email == "" {
		return v
	}

	emailRegex := regexp.MustCompile(`^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`)
	if !emailRegex.MatchString(email) {
		v.AddError(field, "email_format", "Invalid email format", email)
	}
	return v
}

// ValidateURL validates URL format
func (v *Validator) ValidateURL(field, url string) *Validator {
	if url == "" {
		return v
	}

	if !strings.HasPrefix(url, "http://") && !strings.HasPrefix(url, "https://") {
		v.AddError(field, "url_format", "URL must start with http:// or https://", url)
	}
	return v
}

// ValidatePositiveInt validates that a value is a positive integer
func (v *Validator) ValidatePositiveInt(field string, value interface{}) *Validator {
	var intVal int
	var err error

	switch val := value.(type) {
	case int:
		intVal = val
	case string:
		intVal, err = strconv.Atoi(val)
		if err != nil {
			v.AddError(field, "integer_format", "Must be a valid integer", value)
			return v
		}
	case float64:
		intVal = int(val)
	default:
		v.AddError(field, "integer_format", "Must be a valid integer", value)
		return v
	}

	if intVal <= 0 {
		v.AddError(field, "positive_integer", "Must be a positive integer", value)
	}
	return v
}

// ValidateEnum validates that a value is in a list of allowed values
func (v *Validator) ValidateEnum(field, value string, allowedValues []string) *Validator {
	if value == "" {
		return v
	}

	for _, allowed := range allowedValues {
		if value == allowed {
			return v
		}
	}

	v.AddError(field, "enum",
		fmt.Sprintf("Must be one of: %s", strings.Join(allowedValues, ", ")), value)
	return v
}

// ValidateRegex validates that a value matches a regex pattern
func (v *Validator) ValidateRegex(field, value, pattern, errorMessage string) *Validator {
	if value == "" {
		return v
	}

	regex, err := regexp.Compile(pattern)
	if err != nil {
		v.AddError(field, "regex_compile", "Invalid regex pattern", pattern)
		return v
	}

	if !regex.MatchString(value) {
		v.AddError(field, "regex_match", errorMessage, value)
	}
	return v
}

// ValidateGitLabUsername validates GitLab username format
func (v *Validator) ValidateGitLabUsername(field, username string) *Validator {
	if username == "" {
		return v
	}

	// GitLab username rules
	if len(username) > 100 {
		v.AddError(field, "username_length", "Username too long (max 100 characters)", username)
	}

	// Check for valid characters (alphanumeric, underscore, hyphen, dot)
	validUsernameRegex := regexp.MustCompile(`^[a-zA-Z0-9._-]+$`)
	if !validUsernameRegex.MatchString(username) {
		v.AddError(field, "username_format",
			"Username can only contain letters, numbers, underscores, hyphens, and dots", username)
	}

	return v
}

// ValidateGitBranchName validates Git branch name format
func (v *Validator) ValidateGitBranchName(field, branchName string) *Validator {
	if branchName == "" {
		return v
	}

	if len(branchName) > 255 {
		v.AddError(field, "branch_length", "Branch name too long (max 255 characters)", branchName)
	}

	// Git branch name rules
	if strings.HasPrefix(branchName, "-") || strings.HasPrefix(branchName, ".") {
		v.AddError(field, "branch_prefix", "Branch name cannot start with - or .", branchName)
	}

	if strings.Contains(branchName, "..") {
		v.AddError(field, "branch_dots", "Branch name cannot contain consecutive dots", branchName)
	}

	// Check for invalid characters
	invalidChars := []string{" ", "~", "^", ":", "?", "*", "[", "]", "\\"}
	for _, char := range invalidChars {
		if strings.Contains(branchName, char) {
			v.AddError(field, "branch_chars",
				fmt.Sprintf("Branch name cannot contain '%s'", char), branchName)
			break
		}
	}

	return v
}

// ValidateFilePath validates file path format for security
func (v *Validator) ValidateFilePath(field, filePath string) *Validator {
	if filePath == "" {
		return v
	}

	if len(filePath) > 4096 {
		v.AddError(field, "path_length", "File path too long (max 4096 characters)", filePath)
	}

	// Check for directory traversal
	if strings.Contains(filePath, "..") {
		v.AddError(field, "path_traversal", "File path cannot contain directory traversal", filePath)
	}

	if strings.HasPrefix(filePath, "/") {
		v.AddError(field, "path_absolute", "File path cannot be absolute", filePath)
	}

	// Check for control characters
	for _, r := range filePath {
		if r < 32 || r == 127 {
			v.AddError(field, "path_control_chars", "File path cannot contain control characters", filePath)
			break
		}
	}

	return v
}

// ValidateYAMLStructure validates basic YAML structure requirements
func (v *Validator) ValidateYAMLStructure(field string, content map[string]interface{}, requiredFields []string) *Validator {
	for _, required := range requiredFields {
		if _, exists := content[required]; !exists {
			v.AddError(field, "yaml_required_field",
				fmt.Sprintf("Required YAML field '%s' is missing", required), "")
		}
	}
	return v
}

// Validate webhook payload structure for security
func ValidateWebhookPayload(payload map[string]interface{}) *AppError {
	validator := NewValidator()

	// Check for required top-level fields
	if payload == nil {
		return NewValidationError("payload", "Payload is null")
	}

	// Validate object_attributes
	objectAttrs, ok := payload["object_attributes"]
	if !ok {
		validator.AddError("object_attributes", "required", "Missing object_attributes")
	} else {
		objectAttrsMap, ok := objectAttrs.(map[string]interface{})
		if !ok {
			validator.AddError("object_attributes", "type", "object_attributes must be an object")
		} else {
			// Validate MR IID
			if mrIID, exists := objectAttrsMap["iid"]; exists {
				validator.ValidatePositiveInt("object_attributes.iid", mrIID)
			} else {
				validator.AddError("object_attributes.iid", "required", "MR IID is required")
			}

			// Validate title if present
			if title, exists := objectAttrsMap["title"]; exists {
				if titleStr, ok := title.(string); ok {
					validator.MaxLength("object_attributes.title", titleStr, 255)

					// Basic XSS prevention
					if strings.Contains(strings.ToLower(titleStr), "<script") ||
						strings.Contains(strings.ToLower(titleStr), "javascript:") {
						validator.AddError("object_attributes.title", "security",
							"Title contains potentially malicious content")
					}
				}
			}

			// Validate branch names
			if sourceBranch, exists := objectAttrsMap["source_branch"]; exists {
				if branchStr, ok := sourceBranch.(string); ok {
					validator.ValidateGitBranchName("object_attributes.source_branch", branchStr)
				}
			}

			if targetBranch, exists := objectAttrsMap["target_branch"]; exists {
				if branchStr, ok := targetBranch.(string); ok {
					validator.ValidateGitBranchName("object_attributes.target_branch", branchStr)
				}
			}
		}
	}

	// Validate project
	project, ok := payload["project"]
	if !ok {
		validator.AddError("project", "required", "Missing project")
	} else {
		projectMap, ok := project.(map[string]interface{})
		if !ok {
			validator.AddError("project", "type", "project must be an object")
		} else {
			// Validate project ID
			if projectID, exists := projectMap["id"]; exists {
				validator.ValidatePositiveInt("project.id", projectID)
			} else {
				validator.AddError("project.id", "required", "Project ID is required")
			}
		}
	}

	// Validate user if present
	if user, exists := payload["user"]; exists {
		if userMap, ok := user.(map[string]interface{}); ok {
			if username, exists := userMap["username"]; exists {
				if usernameStr, ok := username.(string); ok {
					validator.ValidateGitLabUsername("user.username", usernameStr)
				}
			}
		}
	}

	return validator.ToAppError()
}
