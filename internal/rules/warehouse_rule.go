package rules

import (
	"context"
	"fmt"
	"time"

	"github.com/redhat-data-and-ai/naysayer/internal/yaml"
	"github.com/redhat-data-and-ai/naysayer/internal/gitlab"
)

// WarehouseRule implements warehouse size change approval logic
type WarehouseRule struct {
	analyzer *yaml.YAMLAnalyzer
	client   *gitlab.Client
}

// NewWarehouseRule creates a new warehouse rule
func NewWarehouseRule(client *gitlab.Client) *WarehouseRule {
	return &WarehouseRule{
		analyzer: yaml.NewYAMLAnalyzer(client),
		client:   client,
	}
}

// Name returns the rule identifier
func (r *WarehouseRule) Name() string {
	return "warehouse_rule"
}

// Description returns human-readable description
func (r *WarehouseRule) Description() string {
	return "Approves warehouse size decreases, rejects increases"
}


//rename
// Applies checks if this rule should evaluate the MR
func (r *WarehouseRule) Applies(ctx context.Context, mrCtx *MRContext) bool {
	// Always applies - we need to check for warehouse changes
	return true
}

// Evaluate executes the warehouse size logic
func (r *WarehouseRule) Evaluate(ctx context.Context, mrCtx *MRContext) (*RuleResult, error) {
	start := time.Now()
	
	if r.client == nil {
		return &RuleResult{
			Decision: Decision{
				AutoApprove: false,
				Reason:      "GitLab token not configured",
				Summary:     "🚫 Cannot analyze YAML files - missing GitLab token",
				Details:     "Set GITLAB_TOKEN environment variable to enable YAML analysis",
			},
			RuleName:      r.Name(),
			Confidence:    1.0,
			ExecutionTime: time.Since(start),
			Metadata:      map[string]any{"token_configured": false},
		}, nil
	}
	
	// Analyze warehouse changes using existing logic
	warehouseChanges, err := r.analyzer.AnalyzeChanges(mrCtx.ProjectID, mrCtx.MRIID, mrCtx.Changes)
	if err != nil {
		return &RuleResult{
			Decision: Decision{
				AutoApprove: false,
				Reason:      "YAML analysis failed",
				Summary:     "🚫 Analysis error - requires manual approval",
				Details:     fmt.Sprintf("Could not analyze warehouse changes: %v", err),
			},
			RuleName:      r.Name(),
			Confidence:    0.0,
			ExecutionTime: time.Since(start),
			Metadata:      map[string]any{"error": err.Error()},
		}, err
	}
	
	// Apply warehouse logic
	warehouseDecision := r.evaluateWarehouseChanges(warehouseChanges)
	
	return &RuleResult{
		Decision:      warehouseDecision,
		RuleName:      r.Name(),
		Confidence:    1.0,
		ExecutionTime: time.Since(start),
		Metadata: map[string]any{
			"warehouse_changes": len(warehouseChanges),
			"token_configured":  true,
		},
	}, nil
}

// evaluateWarehouseChanges applies the existing warehouse decision logic
func (r *WarehouseRule) evaluateWarehouseChanges(changes []WarehouseChange) Decision {
	if len(changes) == 0 {
		return Decision{
			AutoApprove: false,
			Reason:      "no warehouse changes detected in YAML files",
			Summary:     "🚫 No warehouse changes in YAML - requires approval",
		}
	}

	// Check if all changes are decreases
	for _, change := range changes {
		if !change.IsDecrease {
			return Decision{
				AutoApprove: false,
				Reason:      fmt.Sprintf("warehouse increase detected: %s → %s", change.FromSize, change.ToSize),
				Summary:     "🚫 Warehouse increase - platform approval required",
				Details:     fmt.Sprintf("File: %s", change.FilePath),
			}
		}
	}

	// All changes are decreases - auto-approve
	details := fmt.Sprintf("Found %d warehouse decrease(s)", len(changes))
	return Decision{
		AutoApprove: true,
		Reason:      "all warehouse changes are decreases",
		Summary:     "✅ Warehouse decrease(s) - auto-approved",
		Details:     details,
	}
}