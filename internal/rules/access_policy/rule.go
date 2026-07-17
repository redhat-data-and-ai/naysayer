package access_policy

import (
	"strings"

	"github.com/redhat-data-and-ai/naysayer/internal/rules/common"
	"github.com/redhat-data-and-ai/naysayer/internal/rules/shared"
)

// AccessPolicyRule requires manual review for any changes to the access_policy
// field within data_product_db.presentation_schemas in product.yaml files.
type AccessPolicyRule struct {
	*common.BaseRule
	*common.FileTypeMatcher
}

// NewAccessPolicyRule creates a new access policy rule instance
func NewAccessPolicyRule() *AccessPolicyRule {
	return &AccessPolicyRule{
		BaseRule:        common.NewBaseRule("access_policy_rule", "Requires manual review for "+accessPolicyKey+" changes"),
		FileTypeMatcher: common.NewFileTypeMatcher(),
	}
}
const accessPolicyKey = "access_policy"

// GetCoveredLines returns the actual lines where access_policy appears.
// Always returns at least a placeholder for product files so the rule participates
// in evaluation and satisfies the defense-in-depth expected-rule check.
func (r *AccessPolicyRule) GetCoveredLines(filePath string, fileContent string) []shared.LineRange {
	if !r.IsProductFile(filePath) {
		return []shared.LineRange{}
	}

	var ranges []shared.LineRange
	lines := strings.Split(fileContent, "\n")
	for i, line := range lines {
		if strings.Contains(line, accessPolicyKey) {
			ranges = append(ranges, shared.LineRange{
				StartLine: i + 1,
				EndLine:   i + 1,
				FilePath:  filePath,
			})
		}
	}

	if len(ranges) > 0 {
		return ranges
	}
	return []shared.LineRange{{StartLine: 1, EndLine: 1, FilePath: filePath}}
}

// ValidateLines returns ManualReview only when modified line ranges contain access_policy.
func (r *AccessPolicyRule) ValidateLines(filePath string, fileContent string, lineRanges []shared.LineRange) (shared.DecisionType, string) {
	if !r.IsProductFile(filePath) {
		return shared.Approve, "Not a product file - access_policy rule does not apply"
	}
	if len(lineRanges) == 0 {
		return shared.Approve, "No access_policy changes detected"
	}

	lines := strings.Split(fileContent, "\n")
	for _, lr := range lineRanges {
		for l := lr.StartLine; l <= lr.EndLine; l++ {
			if l > 0 && l <= len(lines) {
				if strings.Contains(lines[l-1], accessPolicyKey) {
					return shared.ManualReview, accessPolicyKey + " changes require manual review"
				}
			}
		}
	}
	return shared.Approve, "No access_policy changes detected"
}
