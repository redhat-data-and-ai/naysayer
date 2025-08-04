package rules

import (
	"context"
	"fmt"
	"time"
)

// ProjectRule implements project-specific approval logic
type ProjectRule struct {
	allowedProjects map[int]bool
	maxFileChanges  int
}

// NewProjectRule creates a new project rule with specific project IDs
func NewProjectRule(allowedProjects []int, maxFileChanges int) *ProjectRule {
	allowed := make(map[int]bool)
	for _, projectID := range allowedProjects {
		allowed[projectID] = true
	}
	
	return &ProjectRule{
		allowedProjects: allowed,
		maxFileChanges:  maxFileChanges,
	}
}

// Name returns the rule identifier
func (r *ProjectRule) Name() string {
	return "project_rule"
}

// Description returns human-readable description
func (r *ProjectRule) Description() string {
	return "Project-specific approval rules with file change limits"
}


// Applies checks if this rule should evaluate the MR
func (r *ProjectRule) Applies(ctx context.Context, mrCtx *MRContext) bool {
	// Only applies to configured projects
	return r.allowedProjects[mrCtx.ProjectID]
}

// Evaluate executes the project-specific logic
func (r *ProjectRule) Evaluate(ctx context.Context, mrCtx *MRContext) (*RuleResult, error) {
	start := time.Now()
	
	// Check file change limit
	if len(mrCtx.Changes) > r.maxFileChanges {
		return &RuleResult{
			Decision: Decision{
				AutoApprove: false,
				Reason:      fmt.Sprintf("too many file changes: %d > %d", len(mrCtx.Changes), r.maxFileChanges),
				Summary:     "🚫 File change limit exceeded",
				Details:     fmt.Sprintf("Project %d allows max %d file changes", mrCtx.ProjectID, r.maxFileChanges),
			},
			RuleName:      r.Name(),
			Confidence:    1.0,
			ExecutionTime: time.Since(start),
			Metadata: map[string]any{
				"file_changes":     len(mrCtx.Changes),
				"max_allowed":      r.maxFileChanges,
				"project_id":       mrCtx.ProjectID,
			},
		}, nil
	}
	
	// All checks passed
	return &RuleResult{
		Decision: Decision{
			AutoApprove: true,
			Reason:      fmt.Sprintf("project %d within limits", mrCtx.ProjectID),
			Summary:     "✅ Project rules satisfied",
		},
		RuleName:      r.Name(),
		Confidence:    1.0,
		ExecutionTime: time.Since(start),
		Metadata: map[string]any{
			"file_changes": len(mrCtx.Changes),
			"max_allowed":  r.maxFileChanges,
			"project_id":   mrCtx.ProjectID,
		},
	}, nil
}