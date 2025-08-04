package rules

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// UnanimousRuleEngine implements unanimous-only rule evaluation
type UnanimousRuleEngine struct {
	rules map[string]Rule
	mu    sync.RWMutex
}

// NewUnanimousRuleEngine creates a new rule engine with unanimous strategy
func NewUnanimousRuleEngine() *UnanimousRuleEngine {
	return &UnanimousRuleEngine{
		rules: make(map[string]Rule),
	}
}

// RegisterRule adds a rule to the engine
func (e *UnanimousRuleEngine) RegisterRule(rule Rule) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	
	if _, exists := e.rules[rule.Name()]; exists {
		return fmt.Errorf("rule %s already registered", rule.Name())
	}
	
	e.rules[rule.Name()] = rule
	return nil
}

// UnregisterRule removes a rule by name
func (e *UnanimousRuleEngine) UnregisterRule(name string) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	
	if _, exists := e.rules[name]; !exists {
		return fmt.Errorf("rule %s not found", name)
	}
	
	delete(e.rules, name)
	return nil
}

// ListRules returns all registered rules in registration order
func (e *UnanimousRuleEngine) ListRules() []Rule {
	e.mu.RLock()
	defer e.mu.RUnlock()
	
	rules := make([]Rule, 0, len(e.rules))
	for _, rule := range e.rules {
		rules = append(rules, rule)
	}
	
	return rules
}

// EvaluateAll runs all applicable rules and returns unanimous decision
func (e *UnanimousRuleEngine) EvaluateAll(ctx context.Context, mrCtx *MRContext) (*UnanimousResult, error) {
	start := time.Now()
	
	rules := e.ListRules()
	var results []*RuleResult
	applicableCount := 0
	allApproved := true
	
	for _, rule := range rules {
		if !rule.Applies(ctx, mrCtx) {
			continue
		}
		
		applicableCount++
		result, err := rule.Evaluate(ctx, mrCtx)
		if err != nil {
			result = &RuleResult{
				Decision: Decision{
					AutoApprove: false,
					Reason:      fmt.Sprintf("Rule %s failed: %v", rule.Name(), err),
					Summary:     "🚫 Rule evaluation error",
				},
				RuleName:      rule.Name(),
				Confidence:    0.0,
				Metadata:      map[string]any{"error": err.Error()},
				ExecutionTime: time.Since(start),
			}
			allApproved = false
		} else if !result.Decision.AutoApprove {
			allApproved = false
		}
		
		results = append(results, result)
	}
	
	finalDecision := e.createUnanimousDecision(results, allApproved, applicableCount)
	
	return &UnanimousResult{
		FinalDecision:   finalDecision,
		RuleResults:     results,
		TotalRules:      len(rules),
		ApplicableRules: applicableCount,
		ExecutionTime:   time.Since(start),
		AllApproved:     allApproved,
	}, nil
}

// EvaluateRule runs a specific rule by name
func (e *UnanimousRuleEngine) EvaluateRule(ctx context.Context, ruleName string, mrCtx *MRContext) (*RuleResult, error) {
	e.mu.RLock()
	rule, exists := e.rules[ruleName]
	e.mu.RUnlock()
	
	if !exists {
		return nil, fmt.Errorf("rule %s not found", ruleName)
	}
	
	if !rule.Applies(ctx, mrCtx) {
		return &RuleResult{
			Decision: Decision{
				AutoApprove: false,
				Reason:      "Rule not applicable",
				Summary:     "Rule conditions not met",
			},
			RuleName:   ruleName,
			Confidence: 0.0,
		}, nil
	}
	
	return rule.Evaluate(ctx, mrCtx)
}

// createUnanimousDecision builds the final decision based on unanimous approval
func (e *UnanimousRuleEngine) createUnanimousDecision(results []*RuleResult, allApproved bool, applicableCount int) Decision {
	if applicableCount == 0 {
		return Decision{
			AutoApprove: false,
			Reason:      "no applicable rules found",
			Summary:     "🚫 No rules matched - requires manual review",
		}
	}
	
	if allApproved {
		reasons := make([]string, len(results))
		for i, result := range results {
			reasons[i] = fmt.Sprintf("%s: %s", result.RuleName, result.Decision.Reason)
		}
		
		return Decision{
			AutoApprove: true,
			Reason:      fmt.Sprintf("all %d rules approve", len(results)),
			Summary:     "✅ Unanimous approval",
			Details:     fmt.Sprintf("Rules: %v", reasons),
		}
	}
	
	rejectionReasons := []string{}
	for _, result := range results {
		if !result.Decision.AutoApprove {
			rejectionReasons = append(rejectionReasons, fmt.Sprintf("%s: %s", result.RuleName, result.Decision.Reason))
		}
	}
	
	return Decision{
		AutoApprove: false,
		Reason:      fmt.Sprintf("%d rule(s) rejected", len(rejectionReasons)),
		Summary:     "🚫 Unanimous approval required",
		Details:     fmt.Sprintf("Rejections: %v", rejectionReasons),
	}
}