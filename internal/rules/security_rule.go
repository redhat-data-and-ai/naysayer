package rules

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/redhat-data-and-ai/naysayer/internal/decision"
)

// SecurityRule blocks changes to sensitive files and configurations
type SecurityRule struct {
	sensitivePatterns []*regexp.Regexp
	sensitiveFiles    []string
}

// NewSecurityRule creates a new security rule with default patterns
func NewSecurityRule() *SecurityRule {
	patterns := []*regexp.Regexp{
		regexp.MustCompile(`(?i)\.env`),                    // Environment files
		regexp.MustCompile(`(?i)secret`),                  // Secret files
		regexp.MustCompile(`(?i)password`),                // Password files
		regexp.MustCompile(`(?i)token`),                   // Token files
		regexp.MustCompile(`(?i)key\.pem`),                // Private keys
		regexp.MustCompile(`(?i)\.key$`),                  // Key files
		regexp.MustCompile(`(?i)config/deployment\.yaml`), // Deployment configs
		regexp.MustCompile(`(?i)\.crt$`),                  // Certificates
	}
	
	sensitiveFiles := []string{
		"Dockerfile",
		"docker-compose.yml",
		"k8s/secrets.yaml",
		"config/secrets.yaml",
		".gitlab-ci.yml",
		"Makefile",
	}
	
	return &SecurityRule{
		sensitivePatterns: patterns,
		sensitiveFiles:    sensitiveFiles,
	}
}

// Name returns the rule identifier
func (r *SecurityRule) Name() string {
	return "security_rule"
}

// Description returns human-readable description
func (r *SecurityRule) Description() string {
	return "Blocks changes to sensitive files and security configurations"
}

// Priority returns rule priority
func (r *SecurityRule) Priority() int {
	return 200 // Higher priority than warehouse rule
}

// Version returns rule version
func (r *SecurityRule) Version() string {
	return "1.0.0"
}

// Applies checks if this rule should evaluate the MR
func (r *SecurityRule) Applies(ctx context.Context, mrCtx *MRContext) bool {
	// Check if any changed files are sensitive
	for _, change := range mrCtx.Changes {
		if r.isSensitiveFile(change.NewPath) || r.isSensitiveFile(change.OldPath) {
			return true
		}
	}
	return false
}

// Evaluate executes the security logic
func (r *SecurityRule) Evaluate(ctx context.Context, mrCtx *MRContext) (*RuleResult, error) {
	start := time.Now()
	
	var sensitiveFiles []string
	
	// Check all changed files
	for _, change := range mrCtx.Changes {
		if r.isSensitiveFile(change.NewPath) {
			sensitiveFiles = append(sensitiveFiles, change.NewPath)
		}
		if change.OldPath != "" && r.isSensitiveFile(change.OldPath) {
			sensitiveFiles = append(sensitiveFiles, change.OldPath)
		}
	}
	
	if len(sensitiveFiles) > 0 {
		return &RuleResult{
			Decision: decision.Decision{
				AutoApprove: false,
				Reason:      "sensitive file changes detected",
				Summary:     "🚫 Security review required",
				Details:     fmt.Sprintf("Sensitive files: %v", sensitiveFiles),
			},
			RuleName:      r.Name(),
			Confidence:    1.0,
			ExecutionTime: time.Since(start),
			Metadata: map[string]any{
				"sensitive_files": sensitiveFiles,
				"files_count":     len(sensitiveFiles),
			},
		}, nil
	}
	
	// No sensitive files found - approve
	return &RuleResult{
		Decision: decision.Decision{
			AutoApprove: true,
			Reason:      "no sensitive file changes detected",
			Summary:     "✅ Security check passed",
		},
		RuleName:      r.Name(),
		Confidence:    1.0,
		ExecutionTime: time.Since(start),
		Metadata: map[string]any{
			"sensitive_files": []string{},
			"files_count":     0,
		},
	}, nil
}

// isSensitiveFile checks if a file path is considered sensitive
func (r *SecurityRule) isSensitiveFile(filePath string) bool {
	if filePath == "" {
		return false
	}
	
	// Check exact matches
	for _, sensitiveFile := range r.sensitiveFiles {
		if strings.HasSuffix(filePath, sensitiveFile) {
			return true
		}
	}
	
	// Check regex patterns
	for _, pattern := range r.sensitivePatterns {
		if pattern.MatchString(filePath) {
			return true
		}
	}
	
	return false
}