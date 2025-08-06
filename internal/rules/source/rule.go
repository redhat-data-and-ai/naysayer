package source

import (
	"strings"

	"github.com/redhat-data-and-ai/naysayer/internal/gitlab"
	"github.com/redhat-data-and-ai/naysayer/internal/rules/shared"
)

// Rule implements source binding configuration approval logic
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
	return "Auto-approves MRs with only dataverse-safe files (warehouse/sourcebinding)"
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
func (r *Rule) ShouldApprove(mrCtx *shared.MRContext) (shared.DecisionType, string) {
	if r.client == nil {
		return shared.ManualReview, "GitLab token not configured - cannot analyze sourceBinding files"
	}

	// Use shared dataverse file analysis
	fileTypes := shared.AnalyzeDataverseChanges(mrCtx.Changes)

	// If no dataverse files, approve (this rule doesn't apply)
	if len(fileTypes) == 0 {
		return shared.Approve, "No dataverse file changes detected"
	}

	// If all changes are dataverse-safe files, auto-approve with dynamic message
	if shared.AreAllDataverseSafe(mrCtx.Changes) {
		return shared.Approve, shared.BuildDataverseApprovalMessage(fileTypes)
	}

	// Mixed changes - require manual review
	return shared.ManualReview, "MR contains non-dataverse file changes"
}
