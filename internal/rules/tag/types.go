package tag

// Tag represents the structure of a Tag CR YAML
type Tag struct {
	Kind            string   `yaml:"kind"`
	Name            string   `yaml:"name"`
	Description     string   `yaml:"description"`
	DataProduct     string   `yaml:"data_product"`
	MaskingPolicies []Policy `yaml:"masking_policies"`
	AllowedValues   []string `yaml:"allowed_values"`
}

// Policy represents a masking policy reference in a tag
type Policy struct {
	Name string `yaml:"name"`
}

// ValidationError represents a validation error with details
type ValidationError struct {
	Field   string
	Message string
}

// ValidationResult contains all validation errors for a tag
type ValidationResult struct {
	IsValid bool
	Errors  []ValidationError
}

// NewValidationResult creates a new validation result
func NewValidationResult() *ValidationResult {
	return &ValidationResult{
		IsValid: true,
		Errors:  []ValidationError{},
	}
}

// AddError adds a validation error
func (v *ValidationResult) AddError(field, message string) {
	v.IsValid = false
	v.Errors = append(v.Errors, ValidationError{
		Field:   field,
		Message: message,
	})
}

// GetErrorMessages returns all error messages as a slice of strings
func (v *ValidationResult) GetErrorMessages() []string {
	messages := make([]string, len(v.Errors))
	for i, err := range v.Errors {
		if err.Field != "" {
			messages[i] = err.Field + ": " + err.Message
		} else {
			messages[i] = err.Message
		}
	}
	return messages
}
