package sandbox_personal

import (
	"fmt"
	"strings"

	"github.com/redhat-data-and-ai/naysayer/internal/gitlab"
	"github.com/redhat-data-and-ai/naysayer/internal/logging"
	"github.com/redhat-data-and-ai/naysayer/internal/rules/common"
	"github.com/redhat-data-and-ai/naysayer/internal/rules/shared"
	"gopkg.in/yaml.v3"
)

// ProductWarehouseRule validates that NEW sandbox/product.yaml files
// for Personal UnstructuredDataProducts have warehouses=[user:XSMALL, sa:XSMALL]
type ProductWarehouseRule struct {
	*common.BaseRule
	*common.ValidationHelper
	client    gitlab.GitLabClient
	mrContext *shared.MRContext
}

// ProductYAML represents the structure of product.yaml for warehouse validation
type ProductYAML struct {
	Name           string           `yaml:"name"`
	Kind           string           `yaml:"kind"`
	Type           string           `yaml:"type,omitempty"`
	RoverGroup     string           `yaml:"rover_group"`
	Warehouses     []WarehouseEntry `yaml:"warehouses"`
	ServiceAccount map[string]any   `yaml:"service_account,omitempty"`
	Tags           map[string]any   `yaml:"tags,omitempty"`
}

// WarehouseEntry represents a warehouse configuration entry
type WarehouseEntry struct {
	Type string `yaml:"type"`
	Size string `yaml:"size"`
}

// NewProductWarehouseRule creates a new sandbox product warehouse rule
func NewProductWarehouseRule(client gitlab.GitLabClient) *ProductWarehouseRule {
	return &ProductWarehouseRule{
		BaseRule:         common.NewBaseRule("sandbox_product_warehouse_rule", "Validates XSMALL warehouses for NEW sandbox Personal UnstructuredDataProduct"),
		ValidationHelper: common.NewValidationHelper(),
		client:           client,
	}
}

// SetMRContext implements the ContextAwareRule interface
func (r *ProductWarehouseRule) SetMRContext(mrCtx *shared.MRContext) {
	r.mrContext = mrCtx
}

// ValidateLines validates the warehouse configuration for NEW files only
func (r *ProductWarehouseRule) ValidateLines(filePath string, fileContent string, lineRanges []shared.LineRange) (shared.DecisionType, string) {
	if r.mrContext == nil {
		logging.Warn("[%s] No MR context available for validation", r.Name())
		return r.CreateApprovalResult("Auto-approved: No MR context")
	}

	// Only apply when this file belongs to a sandbox Personal UnstructuredDataProduct
	if !IsSandboxPersonalProductForFile(r.mrContext, r.client, filePath) {
		return r.CreateApprovalResult("Auto-approved: Not a sandbox Personal UnstructuredDataProduct")
	}

	// This MR IS a sandbox Personal context - now check if THIS specific file is the sandbox product.yaml
	if !strings.Contains(filePath, "/sandbox/product.yaml") && !strings.Contains(filePath, "/sandbox/product.yml") {
		return r.CreateApprovalResult("Auto-approved: Not the sandbox product.yaml file")
	}

	// Check if this is a NEW file
	isNewFile := isNewFileInMR(r.mrContext, r.client, filePath)

	// If this is an EXISTING file, skip to warehouse_rule
	if !isNewFile {
		logging.Info("[%s] EXISTING file detected, skipping sandbox warehouse validation", r.Name())
		return r.CreateApprovalResult("Auto-approved: Existing sandbox product.yaml - handled by warehouse_rule")
	}

	// NEW file - validate warehouses
	logging.Info("[%s] NEW file detected, validating warehouses must be XSMALL", r.Name())

	// Fetch the full file content to parse the complete product.yaml
	fullFileContent, err := r.getFullFileContent(filePath)
	if err != nil {
		logging.Error("[%s] Failed to fetch full file content: %v", r.Name(), err)
		return r.CreateManualReviewResult(fmt.Sprintf("Manual review required: Failed to fetch file content: %v", err))
	}

	// Parse the full YAML content
	var product ProductYAML
	err = yaml.Unmarshal([]byte(fullFileContent), &product)
	if err != nil {
		logging.Error("[%s] Failed to parse product.yaml: %v", r.Name(), err)
		return r.CreateManualReviewResult(fmt.Sprintf("Manual review required: Failed to parse product.yaml: %v", err))
	}

	// Validate all warehouses are XSMALL
	valid, invalidWarehouses := r.validateWarehousesAreXSMALL(product.Warehouses)
	if !valid {
		reason := fmt.Sprintf("Manual review required: New sandbox Personal product must have all warehouses set to XSMALL. Found: %s", strings.Join(invalidWarehouses, ", "))
		logging.Warn("[%s] %s", r.Name(), reason)
		return r.CreateManualReviewResult(reason)
	}

	return r.CreateApprovalResult("Auto-approved: New sandbox Personal product with all XSMALL warehouses")
}

// GetCoveredLines returns the warehouse section coverage
func (r *ProductWarehouseRule) GetCoveredLines(filePath string, fileContent string) []shared.LineRange {
	// This rule covers the warehouses section
	// The section-based manager will extract the appropriate line ranges
	return r.GetFullFileCoverage(filePath, fileContent)
}

// validateWarehousesAreXSMALL checks if all warehouses have size=XSMALL
func (r *ProductWarehouseRule) validateWarehousesAreXSMALL(warehouses []WarehouseEntry) (bool, []string) {
	var invalidWarehouses []string

	for _, wh := range warehouses {
		if strings.ToUpper(wh.Size) != "XSMALL" {
			invalidWarehouses = append(invalidWarehouses, fmt.Sprintf("%s:%s", wh.Type, wh.Size))
		}
	}

	return len(invalidWarehouses) == 0, invalidWarehouses
}

// getFullFileContent fetches the complete file content from GitLab
func (r *ProductWarehouseRule) getFullFileContent(filePath string) (string, error) {
	if r.client == nil {
		return "", fmt.Errorf("GitLab client not available")
	}

	if r.mrContext == nil {
		return "", fmt.Errorf("MR context not available")
	}

	sourceBranch := r.mrContext.MRInfo.SourceBranch
	if sourceBranch == "" {
		return "", fmt.Errorf("source branch not available")
	}

	// Fetch the file from the source branch (current version in MR)
	fileContent, err := r.client.FetchFileContent(r.mrContext.ProjectID, filePath, sourceBranch)
	if err != nil {
		return "", fmt.Errorf("failed to fetch file from %s: %w", sourceBranch, err)
	}

	if fileContent == nil {
		return "", fmt.Errorf("file content is nil")
	}

	content := strings.TrimSpace(fileContent.Content)
	if content == "" {
		return "", fmt.Errorf("file content is empty")
	}

	return content, nil
}
