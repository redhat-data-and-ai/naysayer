package rules

import (
	"context"
	"time"

	"github.com/redhat-data-and-ai/naysayer/internal/gitlab"
)

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

// Rule defines the interface that all pluggable rules must implement
type Rule interface {
	// Name returns the unique identifier for this rule
	Name() string
	
	// Description returns a human-readable description
	Description() string
	
	// Applies checks if this rule should be evaluated for the given context
	Applies(ctx context.Context, mrCtx *MRContext) bool
	
	// Evaluate executes the rule logic and returns a decision
	Evaluate(ctx context.Context, mrCtx *MRContext) (*RuleResult, error)
}

// RuleEngine manages and executes pluggable rules with unanimous strategy
type RuleEngine interface {
	// RegisterRule adds a rule to the engine
	RegisterRule(rule Rule) error
	
	// UnregisterRule removes a rule by name
	UnregisterRule(name string) error
	
	// ListRules returns all registered rules
	ListRules() []Rule
	
	// EvaluateAll runs all applicable rules and returns unanimous decision
	EvaluateAll(ctx context.Context, mrCtx *MRContext) (*UnanimousResult, error)
	
	// EvaluateRule runs a specific rule by name
	EvaluateRule(ctx context.Context, ruleName string, mrCtx *MRContext) (*RuleResult, error)
}

// UnanimousResult combines results from all rules with unanimous strategy
type UnanimousResult struct {
	FinalDecision   Decision `json:"final_decision"`
	RuleResults     []*RuleResult     `json:"rule_results"`
	TotalRules      int              `json:"total_rules"`
	ApplicableRules int              `json:"applicable_rules"`
	ExecutionTime   time.Duration    `json:"execution_time"`
	AllApproved     bool             `json:"all_approved"`
}

// RuleConfig represents configuration for rule loading
type RuleConfig struct {
	Name        string         `yaml:"name" json:"name"`
	Enabled     bool          `yaml:"enabled" json:"enabled"`
	Config      map[string]any `yaml:"config" json:"config"`
	Conditions  []Condition   `yaml:"conditions" json:"conditions"`
}

// Condition defines when a rule should apply
type Condition struct {
	Field    string `yaml:"field" json:"field"`       // "project_id", "labels", "environment"
	Operator string `yaml:"operator" json:"operator"` // "eq", "in", "regex", "exists"
	Value    any    `yaml:"value" json:"value"`
}