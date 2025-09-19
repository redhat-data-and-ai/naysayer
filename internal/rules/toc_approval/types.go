package toc_approval

// TOCEnvironmentConfig holds configuration for TOC approval environments
type TOCEnvironmentConfig struct {
	// RequiredEnvironments are environments that require TOC approval for new products
	RequiredEnvironments []string `json:"required_environments"`

	// CaseSensitive determines if environment matching is case sensitive
	CaseSensitive bool `json:"case_sensitive"`
}

// DefaultTOCEnvironmentConfig returns the default configuration
func DefaultTOCEnvironmentConfig() *TOCEnvironmentConfig {
	return &TOCEnvironmentConfig{
		RequiredEnvironments: []string{"preprod", "prod"},
		CaseSensitive:        false,
	}
}

// TOCApprovalContext contains context information for TOC approval decisions
type TOCApprovalContext struct {
	FilePath         string `json:"file_path"`
	Environment      string `json:"environment"`
	IsNewFile        bool   `json:"is_new_file"`
	RequiresApproval bool   `json:"requires_approval"`
	ApprovalReason   string `json:"approval_reason"`
}
