package access_request

import (
	"path/filepath"
	"strings"

	"github.com/redhat-data-and-ai/naysayer/internal/rules/common"
	"github.com/redhat-data-and-ai/naysayer/internal/rules/shared"
	"gopkg.in/yaml.v3"
)

const (
	ruleName = "hello_access_request"

	helloaggregatePrefix = "dataproducts/aggregate/helloaggregate/access-requests/groups/dataverse-aggregate-helloaggregate/"
	hellosourcePrefix      = "dataproducts/source/hellosource/access-requests/groups/dataverse-source-hellosource/"

	dataProductHelloaggregate = "helloaggregate"
	dataProductHellosource    = "hellosource"
)

// Rule auto-approves access-request YAML files for helloaggregate and hellosource data products.
type Rule struct {
	*common.BaseRule
	*common.ValidationHelper
}

// NewRule creates a new access request rule instance.
func NewRule() *Rule {
	return &Rule{
		BaseRule: common.NewBaseRule(
			ruleName,
			"Auto-approves access-request files under helloaggregate and hellosource when the MR contains only those files and YAML fields match the path",
		),
		ValidationHelper: common.NewValidationHelper(),
	}
}

// GetCoveredLines returns line ranges this rule validates.
func (r *Rule) GetCoveredLines(filePath string, fileContent string) []shared.LineRange {
	if !r.isAccessRequestFile(filePath) {
		return nil
	}

	if strings.TrimSpace(fileContent) == "" {
		return []shared.LineRange{{StartLine: 1, EndLine: 1, FilePath: filePath}}
	}

	return r.GetFullFileCoverage(filePath, fileContent)
}

// ValidateLines validates access-request file content and MR scope.
func (r *Rule) ValidateLines(filePath string, fileContent string, lineRanges []shared.LineRange) (shared.DecisionType, string) {
	if !r.isAccessRequestFile(filePath) {
		return r.CreateManualReviewResult("Not an allowed access-request file path")
	}

	if reason := r.validateMRContainsOnlyAccessRequests(); reason != "" {
		return r.CreateManualReviewResult(reason)
	}

	if strings.TrimSpace(fileContent) == "" {
		return r.CreateManualReviewResult("Access-request file deletion or empty content requires manual review")
	}

	expectedProduct, ok := r.expectedDataProduct(filePath)
	if !ok {
		return r.CreateManualReviewResult("Access-request file path is not under an allowed data product directory")
	}

	var doc struct {
		Name        string `yaml:"name"`
		DataProduct string `yaml:"data_product"`
	}
	if err := yaml.Unmarshal([]byte(fileContent), &doc); err != nil {
		return r.CreateManualReviewResult("Failed to parse access-request YAML content")
	}

	if doc.Name == "" || doc.DataProduct == "" {
		return r.CreateManualReviewResult("Access-request YAML must contain name and data_product fields")
	}

	expectedName := r.nameFromFilename(filePath)
	if expectedName == "" {
		return r.CreateManualReviewResult("Could not derive expected name from filename")
	}

	if doc.Name != expectedName {
		return r.CreateManualReviewResult(
			"name field '" + doc.Name + "' does not match filename '" + expectedName + "'",
		)
	}

	if doc.DataProduct != expectedProduct {
		return r.CreateManualReviewResult(
			"data_product '" + doc.DataProduct + "' does not match expected '" + expectedProduct + "' for this path",
		)
	}

	return r.CreateApprovalResult("Auto-approved: valid access-request for " + expectedProduct)
}

func (r *Rule) isAccessRequestFile(filePath string) bool {
	if filePath == "" {
		return false
	}

	normalized := filepath.ToSlash(filePath)
	lower := strings.ToLower(normalized)

	if !strings.HasSuffix(lower, ".yaml") && !strings.HasSuffix(lower, ".yml") {
		return false
	}

	return strings.HasPrefix(lower, helloaggregatePrefix) || strings.HasPrefix(lower, hellosourcePrefix)
}

func (r *Rule) expectedDataProduct(filePath string) (string, bool) {
	lower := strings.ToLower(filepath.ToSlash(filePath))
	switch {
	case strings.HasPrefix(lower, helloaggregatePrefix):
		return dataProductHelloaggregate, true
	case strings.HasPrefix(lower, hellosourcePrefix):
		return dataProductHellosource, true
	default:
		return "", false
	}
}

func (r *Rule) nameFromFilename(filePath string) string {
	base := filepath.Base(filePath)
	lower := strings.ToLower(base)

	switch {
	case strings.HasSuffix(lower, ".yaml"):
		return base[:len(base)-5]
	case strings.HasSuffix(lower, ".yml"):
		return base[:len(base)-4]
	default:
		return ""
	}
}

func (r *Rule) validateMRContainsOnlyAccessRequests() string {
	mrCtx := r.GetMRContext()
	if mrCtx == nil {
		return "MR context not available"
	}

	for _, change := range mrCtx.Changes {
		path := change.NewPath
		if path == "" {
			path = change.OldPath
		}
		if path == "" {
			continue
		}
		if !r.isAccessRequestFile(path) {
			return "MR contains files outside allowed access-request paths: " + path
		}
	}

	return ""
}
