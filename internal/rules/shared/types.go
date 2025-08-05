package shared

import (
	"strings"
	"time"

	"github.com/redhat-data-and-ai/naysayer/internal/gitlab"
)

// Decision represents a binary approval decision
type DecisionType string

const (
	Approve      DecisionType = "approve"       // Auto-approve the MR
	ManualReview DecisionType = "manual_review" // Require manual approval
)

// Decision represents a simplified approval decision for a merge request
type Decision struct {
	Type    DecisionType `json:"type"`
	Reason  string       `json:"reason"`
	Summary string       `json:"summary"`
	Details string       `json:"details,omitempty"`
}

// RuleResult represents the result of a rule evaluation
type RuleResult struct {
	Decision      Decision `json:"decision"`
	RuleName      string           `json:"rule_name"`
	Confidence    float64          `json:"confidence"`  // 0.0-1.0
	Metadata      map[string]any   `json:"metadata,omitempty"`
	ExecutionTime time.Duration    `json:"execution_time"`
}

// MRContext contains all information needed for rule evaluation
type MRContext struct {
	ProjectID   int                    `json:"project_id"`
	MRIID       int                    `json:"mr_iid"`
	Changes     []gitlab.FileChange    `json:"changes"`
	MRInfo      *gitlab.MRInfo        `json:"mr_info"`
	Environment string                 `json:"environment,omitempty"`
	Labels      []string              `json:"labels,omitempty"`
	Metadata    map[string]any        `json:"metadata,omitempty"`
}

// Rule defines a simplified interface for all rules
type Rule interface {
	// Name returns the unique identifier for this rule
	Name() string
	
	// Description returns a human-readable description
	Description() string
	
	// Applies checks if this rule should be evaluated for the given MR
	Applies(mrCtx *MRContext) bool
	
	// ShouldApprove executes the rule logic and returns a binary decision
	ShouldApprove(mrCtx *MRContext) (DecisionType, string)
}

// RuleManager manages and executes rules with simple logic
type RuleManager interface {
	// AddRule registers a rule
	AddRule(rule Rule)
	
	// EvaluateAll runs all applicable rules and returns a final decision
	EvaluateAll(mrCtx *MRContext) *RuleEvaluation
}

// RuleEvaluation contains the results of evaluating all rules
type RuleEvaluation struct {
	FinalDecision Decision        `json:"final_decision"`
	RuleResults   []RuleResult    `json:"rule_results"`
	ExecutionTime time.Duration   `json:"execution_time"`
}

// Common helper functions for rule evaluation

// IsDraftMR returns true if the MR is a draft/WIP
func IsDraftMR(mrCtx *MRContext) bool {
	if mrCtx.MRInfo == nil {
		return false
	}
	
	title := strings.ToLower(mrCtx.MRInfo.Title)
	return strings.Contains(title, "draft") || 
		   strings.Contains(title, "wip") ||
		   strings.HasPrefix(title, "draft:") ||
		   strings.HasPrefix(title, "wip:")
}

// IsAutomatedUser returns true if the MR author is a bot or automated user
func IsAutomatedUser(mrCtx *MRContext) bool {
	if mrCtx.MRInfo == nil {
		return false
	}
	
	author := strings.ToLower(mrCtx.MRInfo.Author)
	automatedUsers := []string{"dependabot", "renovate", "greenkeeper", "snyk-bot"}
	
	for _, botUser := range automatedUsers {
		if strings.Contains(author, botUser) {
			return true
		}
	}
	
	return false
}