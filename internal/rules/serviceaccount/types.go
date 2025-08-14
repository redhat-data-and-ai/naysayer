package serviceaccount

// ServiceAccount represents a service account configuration
type ServiceAccount struct {
	Name      string `yaml:"name"`
	Comment   string `yaml:"comment"`
	Email     string `yaml:"email"`
	Role      string `yaml:"role,omitempty"`
	Warehouse string `yaml:"warehouse,omitempty"`
}

// ValidationResult represents validation outcome
type ValidationResult struct {
	IsValid          bool
	Issues           []ValidationIssue
	Warnings         []ValidationIssue
	RequiresApproval bool
}

// ValidationIssue represents a specific validation problem
type ValidationIssue struct {
	Type       string // "email", "scoping", "naming", "environment"
	Severity   string // "error", "warning"
	Message    string
	Field      string
	Value      string
	Suggestion string
}

// ServiceAccountFile represents the location and environment context of a service account file
type ServiceAccountFile struct {
	Path         string
	Environment  string
	DataProduct  string
	Integration  string
	FileType     string // "appuser"
}