package common

import (
	"strings"

	"github.com/redhat-data-and-ai/naysayer/internal/rules/shared"
)

// BaseRule provides common functionality for all rules
type BaseRule struct {
	name        string
	description string
	mrContext   *shared.MRContext
}

// NewBaseRule creates a new base rule with common functionality
func NewBaseRule(name, description string) *BaseRule {
	return &BaseRule{
		name:        name,
		description: description,
	}
}

// Name returns the rule identifier
func (b *BaseRule) Name() string {
	return b.name
}

// Description returns a human-readable description
func (b *BaseRule) Description() string {
	return b.description
}

// SetMRContext implements ContextAwareRule interface
func (b *BaseRule) SetMRContext(mrCtx *shared.MRContext) {
	b.mrContext = mrCtx
}

// GetMRContext returns the stored MR context
func (b *BaseRule) GetMRContext() *shared.MRContext {
	return b.mrContext
}

// GetFullFileCoverage returns line ranges covering the entire file
func (b *BaseRule) GetFullFileCoverage(filePath, fileContent string) []shared.LineRange {
	totalLines := shared.CountLines(fileContent)
	if totalLines == 0 {
		return []shared.LineRange{}
	}

	return []shared.LineRange{
		{
			StartLine: 1,
			EndLine:   totalLines,
			FilePath:  filePath,
		},
	}
}

// ContainsYAMLField checks if file content contains a specific YAML field
func (b *BaseRule) ContainsYAMLField(fileContent, field string) bool {
	// Simple field detection - can be enhanced with proper YAML parsing
	return strings.Contains(fileContent, field+":") ||
		strings.Contains(fileContent, "spec:\n  "+field+":")
}
