package warehouse

import (
	"fmt"
	"strings"

	"github.com/redhat-data-and-ai/naysayer/internal/gitlab"
	"github.com/redhat-data-and-ai/naysayer/internal/rules/shared"
)

// Rule implements warehouse size change approval logic
type Rule struct {
	analyzer AnalyzerInterface
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
	return "Auto-approves MRs with only dataverse-safe files (warehouse/sourcebinding), requires manual review for warehouse increases"
}

// Applies checks if this rule should evaluate the MR
func (r *Rule) Applies(mrCtx *shared.MRContext) bool {
	// Apply if ANY dataverse files are changed (warehouse OR sourcebinding)
	for _, change := range mrCtx.Changes {
		// Check new path
		if isDataverse, _ := shared.IsDataverseFile(change.NewPath); isDataverse {
			return true
		}
		// Check old path
		if isDataverse, _ := shared.IsDataverseFile(change.OldPath); isDataverse {
			return true
		}
	}
	return false
}

// ShouldApprove executes the warehouse size logic and returns a binary decision
func (r *Rule) ShouldApprove(mrCtx *shared.MRContext) (shared.DecisionType, string) {
	if r.client == nil {
		return shared.ManualReview, "GitLab token not configured - cannot analyze dataproduct files"
	}

	// First check if all changes are dataverse-safe files
	fileTypes := shared.AnalyzeDataverseChanges(mrCtx.Changes)

	// If no dataverse files, approve (this rule doesn't apply)
	if len(fileTypes) == 0 {
		return shared.Approve, "No dataverse file changes detected"
	}

	// If all changes are dataverse-safe, check for breaking warehouse changes
	if shared.AreAllDataverseSafe(mrCtx.Changes) {
		// Only analyze warehouse changes if there are any warehouse files
		if fileTypes[shared.WarehouseFile] > 0 {
			warehouseChanges, err := r.analyzer.AnalyzeChanges(mrCtx.ProjectID, mrCtx.MRIID, mrCtx.Changes)
			if err != nil {
				return shared.ManualReview, fmt.Sprintf("Warehouse analysis failed: %v", err)
			}

			// Check for warehouse increases (breaking changes)
			if decision, reason := r.evaluateWarehouseChanges(warehouseChanges); decision == shared.ManualReview {
				return decision, reason
			}
		}

		// All changes are safe dataverse files - auto-approve with dynamic message
		return shared.Approve, shared.BuildDataverseApprovalMessage(fileTypes)
	}

	// Mixed changes - require manual review
	return shared.ManualReview, "MR contains changes outside the allowed scope of the warehouse rule"
}

// evaluateWarehouseChanges applies the warehouse decision logic
func (r *Rule) evaluateWarehouseChanges(changes []WarehouseChange) (shared.DecisionType, string) {
	if len(changes) == 0 {
		return shared.Approve, "No warehouse changes detected in dataproduct files"
	}

	// Check if all changes are decreases
	for _, change := range changes {
		if !change.IsDecrease {
			if strings.Contains(change.FilePath, "(non-warehouse changes)") {
				// Non-warehouse changes detected
				return shared.ManualReview, fmt.Sprintf("Non-warehouse changes detected in %s",
					strings.Replace(change.FilePath, " (non-warehouse changes)", "", 1))
			} else if change.FromSize == "" {
				// New warehouse creation
				return shared.ManualReview, fmt.Sprintf("New warehouse creation detected: %s in %s",
					change.ToSize, change.FilePath)
			} else {
				// Warehouse size increase
				return shared.ManualReview, fmt.Sprintf("Warehouse increase detected: %s → %s in %s",
					change.FromSize, change.ToSize, change.FilePath)
			}
		}
	}

	// All changes are decreases - auto-approve
	return shared.Approve, fmt.Sprintf("All %d warehouse changes are decreases", len(changes))
}
