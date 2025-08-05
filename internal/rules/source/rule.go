package source

import (
	"strings"

	"github.com/redhat-data-and-ai/naysayer/internal/gitlab"
	"github.com/redhat-data-and-ai/naysayer/internal/rules/shared"
)

// Rule implements source binding configuration approval logic
// This is a template/example rule for future development based on ADR 0037
type Rule struct {
	client *gitlab.Client
}

// NewRule creates a new source binding rule
func NewRule(client *gitlab.Client) *Rule {
	return &Rule{
		client: client,
	}
}

// Name returns the rule identifier
func (r *Rule) Name() string {
	return "source_binding_rule"
}

// Description returns human-readable description
func (r *Rule) Description() string {
	return "Evaluates source binding configuration changes (sourceBinding.yaml) - Template rule for future implementation"
}

// Applies checks if this rule should evaluate the MR
func (r *Rule) Applies(mrCtx *shared.MRContext) bool {
	// Check if sourceBinding.yaml files are changed
	for _, change := range mrCtx.Changes {
		if r.isSourceBindingFile(change.NewPath) || r.isSourceBindingFile(change.OldPath) {
			return true
		}
	}
	return false
}

// isSourceBindingFile checks if a file is a sourceBinding configuration file
func (r *Rule) isSourceBindingFile(path string) bool {
	if path == "" {
		return false
	}
	
	lowerPath := strings.ToLower(path)
	return strings.HasSuffix(lowerPath, "sourcebinding.yaml") || 
		   strings.HasSuffix(lowerPath, "sourcebinding.yml") ||
		   strings.Contains(lowerPath, "sourcebinding")
}

// ShouldApprove executes the source binding logic and returns a binary decision
// This is a template implementation - actual logic should be implemented based on requirements
func (r *Rule) ShouldApprove(mrCtx *shared.MRContext) (shared.DecisionType, string) {
	// Template implementation - always requires manual review for now
	// TODO: Implement actual source binding validation logic based on ADR 0037
	
	if r.client == nil {
		return shared.ManualReview, "GitLab token not configured - cannot analyze sourceBinding files"
	}
	
	// Count sourceBinding file changes
	sourceBindingChanges := 0
	for _, change := range mrCtx.Changes {
		if r.isSourceBindingFile(change.NewPath) || r.isSourceBindingFile(change.OldPath) {
			sourceBindingChanges++
		}
	}
	
	if sourceBindingChanges == 0 {
		return shared.Approve, "No sourceBinding changes detected"
	}
	
	// For now, all sourceBinding changes require manual review
	// Future implementation should analyze the actual changes and determine approval criteria
	return shared.ManualReview, "sourceBinding configuration changes require manual review (template rule)"
}

// Future implementation ideas based on ADR 0037:
// 1. Parse sourceBinding.yaml structure
// 2. Validate source configurations
// 3. Check for breaking changes
// 4. Validate against allowed source types
// 5. Check for security implications
// 6. Validate data schema compatibility