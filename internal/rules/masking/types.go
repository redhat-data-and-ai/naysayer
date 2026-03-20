package masking

// MaskingPolicy represents the structure of a masking policy YAML
type MaskingPolicy struct {
	Kind        string `yaml:"kind"`
	Name        string `yaml:"name"`
	DataProduct string `yaml:"data_product"`
	DataType    string `yaml:"datatype"`
	Mask        string `yaml:"mask"`
	Cases       []Case `yaml:"cases"`
}

// Case represents a masking strategy case
type Case struct {
	Strategy  string     `yaml:"strategy"`
	Consumers []Consumer `yaml:"consumers"`
}

// Consumer represents a consumer in a masking policy case
type Consumer struct {
	Kind string `yaml:"kind"`
	Name string `yaml:"name"`
}

// ValidationError represents a validation error with details
type ValidationError struct {
	Field   string
	Message string
}

// ValidationResult contains all validation errors for a masking policy
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
