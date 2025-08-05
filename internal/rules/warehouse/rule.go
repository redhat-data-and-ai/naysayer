package warehouse

import (
	"fmt"
	"strings"

	"github.com/redhat-data-and-ai/naysayer/internal/gitlab"
	"github.com/redhat-data-and-ai/naysayer/internal/rules/shared"
)

// Rule implements warehouse size change approval logic
type Rule struct {
	analyzer *Analyzer
	client   *gitlab.Client
}

// NewRule creates a new warehouse rule
func NewRule(client *gitlab.Client) *Rule {
	// Create the analyzer internally - no external dependency injection needed
	analyzer := NewAnalyzer(client)
	
	return &Rule{
		analyzer: analyzer,
		client:   client,
	}
}

// Name returns the rule identifier
func (r *Rule) Name() string {
	return "warehouse_rule"
}

// Description returns human-readable description
func (r *Rule) Description() string {
	return "Evaluates warehouse size changes - Approves when warehouse size decreases, skips approval when warehouse size increases"
}

// Applies checks if this rule should evaluate the MR
func (r *Rule) Applies(mrCtx *shared.MRContext) bool {
	// Only apply if dataproduct files are changed
	for _, change := range mrCtx.Changes {
		if r.isDataProductFile(change.NewPath) || r.isDataProductFile(change.OldPath) {
			return true
		}
	}
	return false
}

// isDataProductFile checks if a file is a dataproduct configuration file
func (r *Rule) isDataProductFile(path string) bool {
	if path == "" {
		return false
	}
	
	lowerPath := strings.ToLower(path)
	return strings.HasSuffix(lowerPath, "product.yaml") || strings.HasSuffix(lowerPath, "product.yml")
}

// ShouldApprove executes the warehouse size logic and returns a binary decision
func (r *Rule) ShouldApprove(mrCtx *shared.MRContext) (shared.DecisionType, string) {
	if r.client == nil {
		return shared.ManualReview, "GitLab token not configured - cannot analyze dataproduct files"
	}
	
	// Analyze warehouse changes using internal analyzer
	warehouseChanges, err := r.analyzer.AnalyzeChanges(mrCtx.ProjectID, mrCtx.MRIID, mrCtx.Changes)
	if err != nil {
		return shared.ManualReview, fmt.Sprintf("Analysis failed: %v", err)
	}
	
	// Apply warehouse logic
	return r.evaluateWarehouseChanges(warehouseChanges)
}

// evaluateWarehouseChanges applies the warehouse decision logic
func (r *Rule) evaluateWarehouseChanges(changes []WarehouseChange) (shared.DecisionType, string) {
	if len(changes) == 0 {
		return shared.Approve, "No warehouse changes detected in dataproduct files"
	}

	// Check if all changes are decreases
	for _, change := range changes {
		if !change.IsDecrease {
			return shared.ManualReview, fmt.Sprintf("Warehouse increase detected: %s → %s in %s", 
				change.FromSize, change.ToSize, change.FilePath)
		}
	}

	// All changes are decreases - auto-approve
	return shared.Approve, fmt.Sprintf("All %d warehouse changes are decreases", len(changes))
}