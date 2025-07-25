package rules

import (
	"time"

	"github.com/redhat-data-and-ai/naysayer/internal/rules/shared"
)

// SimpleRuleManager is a concrete implementation of RuleManager
type SimpleRuleManager struct {
	rules []shared.Rule
}

// NewSimpleRuleManager creates a new simple rule manager
func NewSimpleRuleManager() *SimpleRuleManager {
	return &SimpleRuleManager{
		rules: make([]shared.Rule, 0),
	}
}

// AddRule registers a rule with the manager
func (rm *SimpleRuleManager) AddRule(rule shared.Rule) {
	rm.rules = append(rm.rules, rule)
}

// EvaluateAll runs all applicable rules and returns a final decision
func (rm *SimpleRuleManager) EvaluateAll(mrCtx *shared.MRContext) *shared.RuleEvaluation {
	start := time.Now()

	// Early filtering for common skip conditions
	if shared.IsDraftMR(mrCtx) {
		return &shared.RuleEvaluation{
			FinalDecision: shared.Decision{
				Type:    shared.Approve,
				Reason:  "Draft MR - auto-approved",
				Summary: "✅ Draft MR skipped",
				Details: "Draft MRs are automatically approved without rule evaluation",
			},
			RuleResults:   []shared.RuleResult{},
			ExecutionTime: time.Since(start),
		}
	}

	if shared.IsAutomatedUser(mrCtx) {
		return &shared.RuleEvaluation{
			FinalDecision: shared.Decision{
				Type:    shared.Approve,
				Reason:  "Automated user MR - auto-approved",
				Summary: "🤖 Bot MR skipped",
				Details: "MRs from automated users (bots) are automatically approved",
			},
			RuleResults:   []shared.RuleResult{},
			ExecutionTime: time.Since(start),
		}
	}

	var results []shared.RuleResult
	var finalDecision shared.Decision

	// Check if any rules apply
	applicableRules := 0
	for _, rule := range rm.rules {
		ruleStart := time.Now()
		applies := rule.Applies(mrCtx)

		if applies {
			applicableRules++
			decisionType, reason := rule.ShouldApprove(mrCtx)
			ruleExecutionTime := time.Since(ruleStart)

			result := shared.RuleResult{
				Decision: shared.Decision{
					Type:    decisionType,
					Reason:  reason,
					Summary: rm.createSummary(rule.Name(), decisionType),
				},
				RuleName:      rule.Name(),
				Confidence:    1.0,
				ExecutionTime: ruleExecutionTime,
			}
			results = append(results, result)

			// If any rule requires manual review, that's the final decision
			if decisionType == shared.ManualReview {
				finalDecision = shared.Decision{
					Type:    shared.ManualReview,
					Reason:  "One or more rules require manual approval",
					Summary: "🚫 Manual review required",
					Details: rm.createDetailsFromResults(results),
				}
				break
			}
		}
	}

	// If we reach here and haven't set a manual review decision, check what to do
	if finalDecision.Type == "" {
		if applicableRules == 0 {
			// No rules applied - auto approve
			finalDecision = shared.Decision{
				Type:    shared.Approve,
				Reason:  "No applicable rules found",
				Summary: "✅ No rules apply - auto-approved",
				Details: "No rules matched the changes in this MR",
			}
		} else {
			// All applicable rules approved
			finalDecision = shared.Decision{
				Type:    shared.Approve,
				Reason:  "All applicable rules approved",
				Summary: "✅ All rules approved",
				Details: rm.createDetailsFromResults(results),
			}
		}
	}

	return &shared.RuleEvaluation{
		FinalDecision: finalDecision,
		RuleResults:   results,
		ExecutionTime: time.Since(start),
	}
}

// createSummary creates a summary message for a rule result
func (rm *SimpleRuleManager) createSummary(ruleName string, decision shared.DecisionType) string {
	switch decision {
	case shared.Approve:
		return "✅ " + ruleName + " approved"
	case shared.ManualReview:
		return "🚫 " + ruleName + " requires manual review"
	default:
		return "❓ " + ruleName + " unknown decision"
	}
}

// createDetailsFromResults creates a details string from rule results
func (rm *SimpleRuleManager) createDetailsFromResults(results []shared.RuleResult) string {
	if len(results) == 0 {
		return ""
	}

	details := "Rule evaluations:\n"
	for _, result := range results {
		details += "- " + result.RuleName + ": " + result.Decision.Reason + "\n"
	}

	return details
}
