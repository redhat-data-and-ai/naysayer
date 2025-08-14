package serviceaccount

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/redhat-data-and-ai/naysayer/internal/config"
	"github.com/redhat-data-and-ai/naysayer/internal/gitlab"
	"github.com/redhat-data-and-ai/naysayer/internal/rules/shared"
	"gopkg.in/yaml.v3"
)

// GitLabClientInterface defines the interface for GitLab API operations needed by the service account rule
type GitLabClientInterface interface {
	FetchFileContent(projectID int, filePath, ref string) (*gitlab.FileContent, error)
}

// Rule implements service account validation logic
type Rule struct {
	client               GitLabClientInterface
	emailValidator       *EmailValidator
	environmentValidator *EnvironmentValidator
	scopingValidator     *ScopingValidator
	namingValidator      *NamingValidator
	config               *Config
}

// Config holds service account rule configuration
type Config struct {
	ValidateEmailFormat      bool
	RequireIndividualEmail   bool
	AllowedDomains          []string
	AstroEnvironmentsOnly   []string
	EnforceNamingConventions bool
}

// NewRule creates a new service account rule
func NewRule(client GitLabClientInterface) *Rule {
	cfg := config.Load()
	saConfig := &Config{
		ValidateEmailFormat:      cfg.Rules.ServiceAccountRule.ValidateEmailFormat,
		RequireIndividualEmail:   cfg.Rules.ServiceAccountRule.RequireIndividualEmail,
		AllowedDomains:          cfg.Rules.ServiceAccountRule.AllowedDomains,
		AstroEnvironmentsOnly:   cfg.Rules.ServiceAccountRule.AstroEnvironmentsOnly,
		EnforceNamingConventions: cfg.Rules.ServiceAccountRule.EnforceNamingConventions,
	}

	return &Rule{
		client:               client,
		emailValidator:       NewEmailValidator(saConfig.AllowedDomains),
		environmentValidator: NewEnvironmentValidator(),
		scopingValidator:     NewScopingValidator(),
		namingValidator:      NewNamingValidator(),
		config:              saConfig,
	}
}

// Name returns the rule identifier
func (r *Rule) Name() string {
	return "service_account_rule"
}

// Description returns human-readable description
func (r *Rule) Description() string {
	return "Validates service account configurations for email format, scoping, environment restrictions, and naming conventions"
}

// Applies checks if this rule should evaluate the MR
func (r *Rule) Applies(mrCtx *shared.MRContext) bool {
	for _, change := range mrCtx.Changes {
		if r.isServiceAccountFile(change.NewPath) || r.isServiceAccountFile(change.OldPath) {
			return true
		}
	}
	return false
}

// ShouldApprove executes the service account validation logic
func (r *Rule) ShouldApprove(mrCtx *shared.MRContext) (shared.DecisionType, string) {
	var allIssues []ValidationIssue
	var filesProcessed []string

	for _, change := range mrCtx.Changes {
		if r.isServiceAccountFile(change.NewPath) && !change.DeletedFile {
			issues, err := r.validateServiceAccountFile(mrCtx.ProjectID, mrCtx.MRIID, change.NewPath)
			if err != nil {
				return shared.ManualReview, fmt.Sprintf("Failed to validate service account file %s: %v", change.NewPath, err)
			}
			allIssues = append(allIssues, issues...)
			filesProcessed = append(filesProcessed, change.NewPath)
		}
	}

	// Check for errors
	errorCount := 0
	warningCount := 0
	for _, issue := range allIssues {
		if issue.Severity == "error" {
			errorCount++
		} else if issue.Severity == "warning" {
			warningCount++
		}
	}

	if errorCount > 0 {
		return shared.ManualReview, r.formatValidationMessage(allIssues, filesProcessed, errorCount, warningCount)
	}

	// Auto-approve if no errors (warnings are acceptable)
	if warningCount > 0 {
		return shared.Approve, fmt.Sprintf("Service account validation passed with %d warnings across %d files", warningCount, len(filesProcessed))
	}

	return shared.Approve, fmt.Sprintf("Service account validation passed for %d files", len(filesProcessed))
}

// validateServiceAccountFile validates a single service account file
func (r *Rule) validateServiceAccountFile(projectID, mrIID int, filePath string) ([]ValidationIssue, error) {
	var issues []ValidationIssue

	// Parse file path to extract context
	saFile := r.parseServiceAccountFile(filePath)

	// Fetch file content
	fileContent, err := r.client.FetchFileContent(projectID, filePath, "")
	if err != nil {
		return nil, fmt.Errorf("failed to fetch file content: %w", err)
	}

	// Parse YAML
	var sa ServiceAccount
	if err := yaml.Unmarshal([]byte(fileContent.Content), &sa); err != nil {
		issues = append(issues, ValidationIssue{
			Type:       "format",
			Severity:   "error",
			Message:    "Invalid YAML format",
			Field:      "file",
			Value:      filePath,
			Suggestion: "Fix YAML syntax errors",
		})
		return issues, nil
	}

	// Email validation
	if r.config.ValidateEmailFormat && sa.Email != "" {
		if emailIssue := r.emailValidator.ValidateEmail(sa.Email); emailIssue.Type != "" {
			issues = append(issues, emailIssue)
		}
	}

	// Environment validation
	envIssues := r.environmentValidator.ValidateServiceAccountForEnvironment(sa, saFile)
	issues = append(issues, envIssues...)

	// Scoping validation
	scopingIssues := r.scopingValidator.ValidateScoping(sa, saFile)
	issues = append(issues, scopingIssues...)

	// Naming convention validation
	if r.config.EnforceNamingConventions {
		namingIssues := r.namingValidator.ValidateNaming(sa, saFile)
		issues = append(issues, namingIssues...)
	}

	return issues, nil
}

// Helper methods
func (r *Rule) isServiceAccountFile(path string) bool {
	return strings.Contains(path, "serviceaccounts/") && 
		   (strings.HasSuffix(path, "_appuser.yaml") || strings.HasSuffix(path, "_appuser.yml"))
}

func (r *Rule) parseServiceAccountFile(path string) ServiceAccountFile {
	saFile := ServiceAccountFile{
		Path:        path,
		Environment: "unknown",
		DataProduct: "unknown",
		Integration: "unknown",
		FileType:    "appuser",
	}

	// Parse path: serviceaccounts/{environment}/{dataproduct}_{integration}_{environment}_appuser.yaml
	parts := strings.Split(path, "/")
	if len(parts) >= 2 {
		// Extract environment from directory
		for i, part := range parts {
			if part == "serviceaccounts" && i+1 < len(parts) {
				saFile.Environment = parts[i+1]
				break
			}
		}

		// Extract filename without extension
		filename := filepath.Base(path)
		filename = strings.TrimSuffix(filename, ".yaml")
		filename = strings.TrimSuffix(filename, ".yml")

		// Parse filename pattern: {dataproduct}_{integration}_{environment}_appuser
		fileParts := strings.Split(filename, "_")
		if len(fileParts) >= 3 {
			saFile.DataProduct = fileParts[0]
			if len(fileParts) >= 4 {
				// For patterns like: marketo_astro_preprod_appuser
				saFile.Integration = fileParts[1]
			}
		}
	}

	return saFile
}

func (r *Rule) formatValidationMessage(issues []ValidationIssue, files []string, errorCount, warningCount int) string {
	summary := fmt.Sprintf("Service account validation failed: %d errors", errorCount)
	if warningCount > 0 {
		summary += fmt.Sprintf(", %d warnings", warningCount)
	}
	summary += fmt.Sprintf(" across %d files", len(files))

	// Add top 3 most critical issues
	errorIssues := make([]ValidationIssue, 0)
	for _, issue := range issues {
		if issue.Severity == "error" && len(errorIssues) < 3 {
			errorIssues = append(errorIssues, issue)
		}
	}

	details := ""
	for i, issue := range errorIssues {
		details += fmt.Sprintf("%d. %s (%s)", i+1, issue.Message, issue.Field)
		if i < len(errorIssues)-1 {
			details += "; "
		}
	}

	if details != "" {
		return summary + ". Issues: " + details
	}
	return summary
}