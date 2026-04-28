package developer_access

import (
	"bytes"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/redhat-data-and-ai/naysayer/internal/rules/common"
	"github.com/redhat-data-and-ai/naysayer/internal/rules/shared"
	"gopkg.in/yaml.v3"
)

var accessRequestPathPattern = regexp.MustCompile(`^dataproducts/(aggregate|source)/([^/]+)/access-requests/groups/([^/]+)/([^/]+)\.ya?ml$`)

type accessRequestFile struct {
	DataProduct *string `yaml:"data_product"`
	Name        *string `yaml:"name"`
}

// Rule validates developer access-request files for data products
// It auto-approves only when path and YAML content are consistent
type Rule struct {
	*common.BaseRule
	*common.ValidationHelper
}

// NewRule creates a developer access-request validation rule
func NewRule() *Rule {
	return &Rule{
		BaseRule: common.NewBaseRule(
			"developer_access_rule",
			"Auto-approves developer access-request files when path and YAML content are consistent.",
		),
		ValidationHelper: common.NewValidationHelper(),
	}
}

// GetCoveredLines returns full-file coverage for matching access-request files
func (r *Rule) GetCoveredLines(filePath, fileContent string) []shared.LineRange {
	if !isAccessRequestFile(filePath) || strings.TrimSpace(fileContent) == "" {
		return []shared.LineRange{}
	}
	return r.GetFullFileCoverage(filePath, fileContent)
}

// ValidateLines validates developer access-request files
func (r *Rule) ValidateLines(filePath, fileContent string, lineRanges []shared.LineRange) (shared.DecisionType, string) {
	if !isAccessRequestFile(filePath) {
		return shared.ManualReview, "Not a developer access-request file"
	}

	// Decode key path segments once and use them as the source of truth
	// for all semantic checks (group convention, username, data_product)
	productTypeFromPath, dataproductFromPath, groupFromPath, usernameFromPath, err := extractPathValues(filePath)
	if err != nil {
		return r.CreateManualReviewResult(fmt.Sprintf("Invalid access-request path: %v", err))
	}
	// Convention guardrail: group must always align with product type and name
	expectedGroup := fmt.Sprintf("dataverse-%s-%s", productTypeFromPath, dataproductFromPath)
	if groupFromPath != expectedGroup {
		return r.CreateManualReviewResult(fmt.Sprintf("group '%s' does not match expected group '%s' from file path", groupFromPath, expectedGroup))
	}

	var payload accessRequestFile
	decoder := yaml.NewDecoder(bytes.NewReader([]byte(fileContent)))
	decoder.KnownFields(true)
	if err := decoder.Decode(&payload); err != nil {
		return r.CreateManualReviewResult("Failed to parse access-request YAML")
	}

	if payload.Name == nil || strings.TrimSpace(*payload.Name) == "" {
		return r.CreateManualReviewResult("Missing required field: name")
	}
	if payload.DataProduct == nil || strings.TrimSpace(*payload.DataProduct) == "" {
		return r.CreateManualReviewResult("Missing required field: data_product")
	}

	trimmedName := strings.TrimSpace(*payload.Name)
	if trimmedName != usernameFromPath {
		return r.CreateManualReviewResult(fmt.Sprintf("name '%s' does not match username '%s' in file path", trimmedName, usernameFromPath))
	}

	trimmedDataProduct := strings.TrimSpace(*payload.DataProduct)
	if trimmedDataProduct != dataproductFromPath {
		return r.CreateManualReviewResult(fmt.Sprintf("data_product '%s' does not match dataproduct '%s' in file path", trimmedDataProduct, dataproductFromPath))
	}

	return r.CreateApprovalResult("Developer access-request validated: path and YAML content match")
}

func isAccessRequestFile(filePath string) bool {
	if filePath == "" {
		return false
	}
	return accessRequestPathPattern.MatchString(filepath.ToSlash(filePath))
}

func extractPathValues(filePath string) (productType string, dataproduct string, group string, username string, err error) {
	matches := accessRequestPathPattern.FindStringSubmatch(filepath.ToSlash(filePath))
	if len(matches) != 5 {
		return "", "", "", "", fmt.Errorf("path must match dataproducts/(aggregate|source)/<dataproduct>/access-requests/groups/<group-name>/<username>.yaml")
	}
	return matches[1], matches[2], matches[3], matches[4], nil
}

