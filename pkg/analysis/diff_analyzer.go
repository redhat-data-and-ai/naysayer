package analysis

import (
	"fmt"
	"regexp"
	"strings"
)

// DiffAnalyzer handles analysis of GitLab MR diffs
type DiffAnalyzer struct {
	warehouseSizes map[string]int
}

// NewDiffAnalyzer creates a new diff analyzer instance
func NewDiffAnalyzer() *DiffAnalyzer {
	return &DiffAnalyzer{
		// Updated to match actual repository format
		warehouseSizes: map[string]int{
			"XSMALL":  1,
			"SMALL":   2,
			"MEDIUM":  3,
			"LARGE":   4,
			"XXLARGE": 5,
		},
	}
}

// ChangeType represents the type of change detected
type ChangeType string

const (
	ChangeTypeWarehouseIncrease ChangeType = "warehouse_increase"
	ChangeTypeWarehouseDecrease ChangeType = "warehouse_decrease"
)

// Change represents a detected change in the MR
type Change struct {
	Type        ChangeType `json:"type"`
	FilePath    string     `json:"file_path"`
	Description string     `json:"description"`
	From        string     `json:"from,omitempty"`
	To          string     `json:"to,omitempty"`
	Severity    string     `json:"severity"` // "low", "medium", "high"
}

// ApprovalDecision represents the final approval decision
type ApprovalDecision struct {
	RequiresApproval bool      `json:"requires_approval"`
	ApprovalType     string    `json:"approval_type"` // "platform", "toc", "pipeline", "none"
	Reason           string    `json:"reason"`
	Changes          []Change  `json:"changes"`
	AutoApprove      bool      `json:"auto_approve"`
	Summary          string    `json:"summary"`
	BlockingIssues   []string  `json:"blocking_issues"` // Issues that prevent approval
}

// AnalyzeMRTitle analyzes MR title for warehouse changes (mock implementation for Phase 1)
func (d *DiffAnalyzer) AnalyzeMRTitle(title, description string) *ApprovalDecision {
	decision := &ApprovalDecision{
		RequiresApproval: false,
		ApprovalType:     "none",
		AutoApprove:      true,
		Changes:          []Change{},
		BlockingIssues:   []string{},
	}

	// Mock analysis based on title and description keywords
	changes := d.detectChangesFromText(title, description)
	decision.Changes = changes

		// WAREHOUSE CHANGES ONLY - Filter to only warehouse changes
	warehouseChanges := []Change{}
	hasWarehouseDecrease := false
	hasWarehouseIncrease := false
	
	for _, change := range changes {
		if change.Type == ChangeTypeWarehouseDecrease {
			hasWarehouseDecrease = true
			warehouseChanges = append(warehouseChanges, change)
		}
		if change.Type == ChangeTypeWarehouseIncrease {
			hasWarehouseIncrease = true
			warehouseChanges = append(warehouseChanges, change)
		}
		// Ignore all other change types
	}

	// Update decision to only include warehouse changes
	decision.Changes = warehouseChanges

	// If no warehouse changes, ignore this MR (auto-approve - not our concern)
	if !hasWarehouseDecrease && !hasWarehouseIncrease {
		decision.RequiresApproval = false
		decision.ApprovalType = "none"
		decision.AutoApprove = true
		decision.Reason = "No warehouse changes detected - auto-approved"
		decision.Summary = d.generateSummary(decision)
		return decision
	}

	// Warehouse decrease policy: Auto-approve
	if hasWarehouseDecrease && !hasWarehouseIncrease {
		decision.RequiresApproval = false
		decision.ApprovalType = "none"
		decision.AutoApprove = true
		decision.Reason = "Warehouse decrease - auto-approved"
		decision.Summary = d.generateSummary(decision)
		return decision
	}

	// Warehouse increase policy: Require platform approval
	if hasWarehouseIncrease {
		decision.RequiresApproval = true
		decision.ApprovalType = "platform"
		decision.AutoApprove = false
		if hasWarehouseDecrease {
			decision.Reason = "Mixed warehouse changes (both increase and decrease) - platform approval required"
		} else {
			decision.Reason = "Warehouse increase - platform approval required"
		}
		decision.Summary = d.generateSummary(decision)
		return decision
	}

	decision.Summary = d.generateSummary(decision)
	return decision
}

// detectChangesFromText performs pattern matching on MR text (WAREHOUSE CHANGES ONLY)
func (d *DiffAnalyzer) detectChangesFromText(title, description string) []Change {
	var changes []Change
	text := strings.ToLower(title + " " + description)

	// ONLY detect warehouse change patterns - ignore everything else
	warehousePattern := regexp.MustCompile(`warehouse.*(?:from\s+(\w+)\s+to\s+(\w+)|(\w+)\s*→\s*(\w+)|increase.*to\s+(\w+))`)
	if matches := warehousePattern.FindStringSubmatch(text); len(matches) > 0 {
		var from, to string
		
		// Extract from/to values from different match groups
		if matches[1] != "" && matches[2] != "" {
			from, to = strings.ToUpper(matches[1]), strings.ToUpper(matches[2])
		} else if matches[3] != "" && matches[4] != "" {
			from, to = strings.ToUpper(matches[3]), strings.ToUpper(matches[4])
		} else if matches[5] != "" {
			from, to = "SMALL", strings.ToUpper(matches[5]) // Default assumption for increases
		}

		if from != "" && to != "" {
			changeType := ChangeTypeWarehouseDecrease
			severity := "low"
			
			if d.isWarehouseIncrease(from, to) {
				changeType = ChangeTypeWarehouseIncrease
				severity = "high"
			}

			changes = append(changes, Change{
				Type:        changeType,
				FilePath:    "product.yaml", // Would be actual file path in real implementation
				Description: fmt.Sprintf("Warehouse size change from %s to %s", from, to),
				From:        from,
				To:          to,
				Severity:    severity,
			})
		}
	}

	// REMOVED: All non-warehouse change detection
	// - Production deployment patterns
	// - Migration patterns  
	// - Pipeline-related patterns
	// NAYSAYER now focuses exclusively on warehouse size changes

	return changes
}

// isWarehouseIncrease determines if a warehouse change is an increase
func (d *DiffAnalyzer) isWarehouseIncrease(from, to string) bool {
	fromSize, fromExists := d.warehouseSizes[from]
	toSize, toExists := d.warehouseSizes[to]
	
	if !fromExists || !toExists {
		return false
	}
	
	return toSize > fromSize
}

// generateSummary creates a human-readable summary for warehouse changes only
func (d *DiffAnalyzer) generateSummary(decision *ApprovalDecision) string {
	if len(decision.Changes) == 0 {
		return "✅ No warehouse changes detected - auto-approved"
	}

	// Single warehouse change
	if len(decision.Changes) == 1 {
		change := decision.Changes[0]
		if change.Type == ChangeTypeWarehouseDecrease {
			return fmt.Sprintf("✅ Warehouse decrease (%s → %s) - auto-approved", 
				change.From, change.To)
		} else if change.Type == ChangeTypeWarehouseIncrease {
			return fmt.Sprintf("🚫 Warehouse increase (%s → %s) - platform approval required", 
				change.From, change.To)
		}
	}

	// Multiple warehouse changes (mixed increase/decrease)
	return "🚫 Mixed warehouse changes - platform approval required"
} 