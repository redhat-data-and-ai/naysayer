package warehouse

import (
	"fmt"
	"strings"

	"github.com/redhat-data-and-ai/naysayer/internal/gitlab"
	"github.com/redhat-data-and-ai/naysayer/internal/rules/shared"
)

// Rule implements warehouse file validation for product.yaml files
type Rule struct {
	client   *gitlab.Client
	analyzer AnalyzerInterface
	mrCtx    *shared.MRContext // Store MR context for warehouse analysis
}

// NewRule creates a new warehouse validation rule
func NewRule(client *gitlab.Client) *Rule {
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
	return "Validates warehouse size changes in product.yaml files - auto-approves size decreases, requires review for increases. Other sections handled by respective rules."
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
func (r *Rule) ValidateLines(filePath string, fileContent string, lineRanges []shared.LineRange) (shared.DecisionType, string) {
	if !r.isWarehouseFile(filePath) {
		return shared.Approve, "Not a warehouse file"
	}

	// If we don't have analyzer or MR context, fall back to simplified validation
	if r.analyzer == nil || r.mrCtx == nil {
		return shared.Approve, "Warehouse file validated (simplified validation - no context)"
	}

	// Use the analyzer to detect warehouse changes - but focus only on warehouse changes
	changes, err := r.analyzer.AnalyzeChanges(r.mrCtx.ProjectID, r.mrCtx.MRIID, r.mrCtx.Changes)
	if err != nil {
		// If analysis fails, require manual review for safety
		return shared.ManualReview, fmt.Sprintf("Warehouse analysis failed: %v", err)
	}

	// Check if this specific file has warehouse size changes (ignore non-warehouse changes)
	var warehouseIncreases []WarehouseChange
	var warehouseDecreases []WarehouseChange

	for _, change := range changes {
		// Check if this change affects the current file
		if strings.Contains(change.FilePath, filePath) {
			// ONLY process actual warehouse size changes - ignore non-warehouse changes
			if change.FromSize != "N/A" && change.ToSize != "N/A" {
				if change.IsDecrease {
					warehouseDecreases = append(warehouseDecreases, change)
				} else {
					warehouseIncreases = append(warehouseIncreases, change)
				}
			}
			// Skip non-warehouse changes (FromSize == "N/A" && ToSize == "N/A")
			// These will be handled by other rules (metadata_rule, etc.)
		}
	}

	// If there are warehouse size increases, require manual review
	if len(warehouseIncreases) > 0 {
		details := []string{}
		for _, change := range warehouseIncreases {
			// Extract warehouse type from FilePath (format: "path (type: user)")
			warehouseType := r.extractWarehouseType(change.FilePath)
			if change.FromSize == "" {
				details = append(details, fmt.Sprintf("New %s warehouse: %s", warehouseType, change.ToSize))
			} else {
				details = append(details, fmt.Sprintf("%s warehouse: %s → %s", warehouseType, change.FromSize, change.ToSize))
			}
		}
		return shared.ManualReview, fmt.Sprintf("Warehouse size increase detected: %s", strings.Join(details, ", "))
	}

	// If there are warehouse size decreases, approve them
	if len(warehouseDecreases) > 0 {
		details := []string{}
		for _, change := range warehouseDecreases {
			warehouseType := r.extractWarehouseType(change.FilePath)
			details = append(details, fmt.Sprintf("%s warehouse: %s → %s", warehouseType, change.FromSize, change.ToSize))
		}
		return shared.Approve, fmt.Sprintf("Warehouse size decrease approved: %s", strings.Join(details, ", "))
	}

	// No warehouse size changes detected - approve (other rules will handle non-warehouse sections)
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
