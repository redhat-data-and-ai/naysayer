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
	return "Validates warehouse configuration files (product.yaml) - auto-approves size decreases, requires review for increases"
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

	// Warehouse rule covers the entire warehouse file
	totalLines := shared.CountLines(fileContent)
	if totalLines == 0 {
		return nil
	}

	return []shared.LineRange{
		{
			StartLine: 1,
			EndLine:   totalLines,
			FilePath:  filePath,
		},
	}
}

// ValidateLines validates the specified line ranges in a warehouse file
func (r *Rule) ValidateLines(filePath string, fileContent string, lineRanges []shared.LineRange) (shared.DecisionType, string) {
	if !r.isWarehouseFile(filePath) {
		return shared.Approve, "Not a warehouse file"
	}

	// If we don't have analyzer or MR context, fall back to simplified validation
	if r.analyzer == nil || r.mrCtx == nil {

		return shared.Approve, "Warehouse file validated (simplified validation - no context)"
	}

	// Use the analyzer to detect warehouse changes
	changes, err := r.analyzer.AnalyzeChanges(r.mrCtx.ProjectID, r.mrCtx.MRIID, r.mrCtx.Changes)
	if err != nil {
		// If analysis fails, require manual review for safety
		return shared.ManualReview, fmt.Sprintf("Warehouse analysis failed: %v", err)
	}

	// Check if this specific file has warehouse changes
	fileHasChanges := false
	var warehouseIncreases []WarehouseChange
	var warehouseDecreases []WarehouseChange
	var nonWarehouseChanges []WarehouseChange

	for _, change := range changes {
		// Check if this change affects the current file
		if strings.Contains(change.FilePath, filePath) {
			fileHasChanges = true
			
			// Categorize the change
			if change.FromSize == "N/A" && change.ToSize == "N/A" {
				// Non-warehouse changes
				nonWarehouseChanges = append(nonWarehouseChanges, change)
			} else if change.IsDecrease {
				warehouseDecreases = append(warehouseDecreases, change)
			} else {
				warehouseIncreases = append(warehouseIncreases, change)
			}
		}
	}

	// If this file has no warehouse-related changes, approve it
	if !fileHasChanges {
		return shared.Approve, "No warehouse changes detected in this file"
	}

	// If there are non-warehouse changes, require manual review
	if len(nonWarehouseChanges) > 0 {
		return shared.ManualReview, "File contains changes beyond warehouse sizes - requires manual review"
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

	// If only decreases or no changes, approve
	if len(warehouseDecreases) > 0 {
		details := []string{}
		for _, change := range warehouseDecreases {
			warehouseType := r.extractWarehouseType(change.FilePath)
			details = append(details, fmt.Sprintf("%s warehouse: %s → %s", warehouseType, change.FromSize, change.ToSize))
		}
		return shared.Approve, fmt.Sprintf("Warehouse size decrease approved: %s", strings.Join(details, ", "))
	}

	return shared.Approve, "Warehouse file validated - no size changes detected"
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


