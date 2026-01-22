package warehouse

import (
	"fmt"
	"sort"
	"strings"

	"github.com/redhat-data-and-ai/naysayer/internal/gitlab"
	"github.com/redhat-data-and-ai/naysayer/internal/rules/shared"
)

// Rule implements warehouse file validation for product.yaml files
type Rule struct {
	client   gitlab.GitLabClient
	analyzer AnalyzerInterface
	mrCtx    *shared.MRContext // Store MR context for warehouse analysis
}

// NewRule creates a new warehouse validation rule
func NewRule(client gitlab.GitLabClient) *Rule {
	var analyzer AnalyzerInterface
	if client != nil {
		analyzer = NewAnalyzer(client)
	}

	return &Rule{
		client:   client,
		analyzer: analyzer,
	}
}

// Name returns the rule identifier
func (r *Rule) Name() string {
	return "warehouse_rule"
}

// Description returns human-readable description
func (r *Rule) Description() string {
	return "Validates warehouse size changes in product.yaml files - all warehouse changes require manual review for cost control and governance."
}

// SetMRContext implements ContextAwareRule interface
func (r *Rule) SetMRContext(mrCtx *shared.MRContext) {
	r.mrCtx = mrCtx
}

// GetCoveredLines returns which line ranges this rule validates in a file
func (r *Rule) GetCoveredLines(filePath string, fileContent string) []shared.LineRange {
	if !r.isWarehouseFile(filePath) {
		return nil // This rule doesn't apply to non-warehouse files
	}

	// Check if file has content
	if len(strings.TrimSpace(fileContent)) == 0 {
		return nil // No content to validate
	}

	// For section-based validation, we return a placeholder range to indicate
	// this rule wants to participate in validation. The actual section content
	// will be provided by the section manager.
	return []shared.LineRange{
		{
			StartLine: 1,
			EndLine:   1, // Placeholder - actual lines handled by section manager
			FilePath:  filePath,
		},
	}
}

// ValidateLines validates warehouse configuration changes
// When called by section-based validation, fileContent contains the warehouses section content
// ALL warehouse changes require manual review - no auto-approval
func (r *Rule) ValidateLines(filePath string, fileContent string, lineRanges []shared.LineRange) (shared.DecisionType, string) {
	if !r.isWarehouseFile(filePath) {
		return shared.Approve, "Not a warehouse file"
	}

	// If we don't have analyzer or MR context, require manual review for safety
	// Never auto-approve warehouse changes without proper analysis
	if r.analyzer == nil || r.mrCtx == nil {
		return shared.ManualReview, "Warehouse changes require manual review"
	}

	// Use the analyzer to detect warehouse changes
	changes, err := r.analyzer.AnalyzeChanges(r.mrCtx.ProjectID, r.mrCtx.MRIID, r.mrCtx.Changes)
	if err != nil {
		// If analysis fails, require manual review for safety
		return shared.ManualReview, fmt.Sprintf("Warehouse analysis failed: %v", err)
	}

	// Check if this specific file has ANY warehouse changes
	// Categories: additions, removals, increases, decreases
	var warehouseAdditions []WarehouseChange
	var warehouseRemovals []WarehouseChange
	var warehouseIncreases []WarehouseChange
	var warehouseDecreases []WarehouseChange

	for _, change := range changes {
		// Check if this change affects the current file
		if strings.Contains(change.FilePath, filePath) {
			// Categorize ALL warehouse changes (not just size changes to existing)
			// Note: FromSize can be "N/A" or empty string "" for new warehouses
			isNewWarehouse := (change.FromSize == "N/A" || change.FromSize == "") && change.ToSize != "N/A" && change.ToSize != ""
			isRemovedWarehouse := change.FromSize != "N/A" && change.FromSize != "" && (change.ToSize == "N/A" || change.ToSize == "")

			if isNewWarehouse {
				// New warehouse added
				warehouseAdditions = append(warehouseAdditions, change)
			} else if isRemovedWarehouse {
				// Warehouse removed
				warehouseRemovals = append(warehouseRemovals, change)
			} else if change.FromSize != "N/A" && change.FromSize != "" && change.ToSize != "N/A" && change.ToSize != "" {
				// Size change to existing warehouse
				if change.IsDecrease {
					warehouseDecreases = append(warehouseDecreases, change)
				} else {
					warehouseIncreases = append(warehouseIncreases, change)
				}
			}
		}
	}

	// ALL warehouse changes require manual review - no auto-approval
	allChanges := len(warehouseAdditions) + len(warehouseRemovals) + len(warehouseIncreases) + len(warehouseDecreases)
	if allChanges > 0 {
		var details []string

		// Report additions (using old format: "New X warehouse: SIZE")
		for _, change := range warehouseAdditions {
			warehouseType := r.extractWarehouseType(change.FilePath)
			details = append(details, fmt.Sprintf("New %s warehouse: %s", warehouseType, change.ToSize))
		}

		// Report removals
		for _, change := range warehouseRemovals {
			warehouseType := r.extractWarehouseType(change.FilePath)
			details = append(details, fmt.Sprintf("%s warehouse removed: was %s", warehouseType, change.FromSize))
		}

		// Count the number of different change types present
		changeTypesPresent := 0
		for _, changes := range [][]WarehouseChange{warehouseAdditions, warehouseRemovals, warehouseIncreases, warehouseDecreases} {
			if len(changes) > 0 {
				changeTypesPresent++
			}
		}

		// Determine if we have truly mixed changes (more than one type of change)
		hasMixedChanges := changeTypesPresent > 1

		// Report size increases
		for _, change := range warehouseIncreases {
			warehouseType := r.extractWarehouseType(change.FilePath)
			details = append(details, formatSizeChangeDetail(warehouseType, change.FromSize, change.ToSize, hasMixedChanges, "increased"))
		}

		// Report size decreases
		for _, change := range warehouseDecreases {
			warehouseType := r.extractWarehouseType(change.FilePath)
			details = append(details, formatSizeChangeDetail(warehouseType, change.FromSize, change.ToSize, hasMixedChanges, "decreased"))
		}

		// Sort details for consistent ordering in comments
		sort.Strings(details)

		// Use appropriate message format based on change type
		if hasMixedChanges {
			// Multiple types of changes - use generic message
			return shared.ManualReview, fmt.Sprintf("Warehouse changes detected - manual review required: %s", strings.Join(details, ", "))
		} else if len(warehouseRemovals) > 0 {
			// Only removals
			return shared.ManualReview, fmt.Sprintf("Warehouse removal detected: %s", strings.Join(details, ", "))
		} else if len(warehouseDecreases) > 0 {
			// Only decreases
			return shared.ManualReview, fmt.Sprintf("Warehouse size decrease detected: %s", strings.Join(details, ", "))
		}
		// Only additions OR only increases - use "increase" message
		return shared.ManualReview, fmt.Sprintf("Warehouse size increase detected: %s", strings.Join(details, ", "))
	}

	// No warehouse changes detected in this file - approve (using old format)
	return shared.Approve, "No warehouse size changes detected - approved"
}

// isWarehouseFile checks if a file is a warehouse configuration file
func (r *Rule) isWarehouseFile(path string) bool {
	if path == "" {
		return false
	}

	lowerPath := strings.ToLower(path)

	// Check for warehouse files (product.yaml)
	if strings.HasSuffix(lowerPath, "product.yaml") || strings.HasSuffix(lowerPath, "product.yml") {
		return true
	}

	return false
}

// extractWarehouseType extracts warehouse type from a change FilePath
// FilePath format: "dataproducts/source/fivetranplatform/sandbox/product.yaml (type: user)"
func (r *Rule) extractWarehouseType(filePath string) string {
	// Look for " (type: " pattern
	if idx := strings.Index(filePath, " (type: "); idx != -1 {
		// Extract everything after " (type: " and before the closing ")"
		typeStart := idx + len(" (type: ")
		if endIdx := strings.Index(filePath[typeStart:], ")"); endIdx != -1 {
			return filePath[typeStart : typeStart+endIdx]
		}
	}
	return "unknown"
}

// formatSizeChangeDetail formats the detail string for warehouse size changes
func formatSizeChangeDetail(warehouseType, from, to string, hasMixedChanges bool, changeVerb string) string {
	if hasMixedChanges {
		return fmt.Sprintf("%s warehouse %s: %s → %s", warehouseType, changeVerb, from, to)
	}
	return fmt.Sprintf("%s warehouse: %s → %s", warehouseType, from, to)
}
