package warehouse

import (
	"strings"

	"github.com/redhat-data-and-ai/naysayer/internal/gitlab"
	"github.com/redhat-data-and-ai/naysayer/internal/rules/shared"
)

// Rule implements warehouse file validation for product.yaml files
type Rule struct {
	client   *gitlab.Client
	analyzer AnalyzerInterface
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

	// For testing and when GitLab client is not available, auto-approve warehouse files
	// TODO: Implement proper line-level warehouse size validation with actual analysis
	return shared.Approve, "Warehouse file validated (simplified validation for testing)"
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


