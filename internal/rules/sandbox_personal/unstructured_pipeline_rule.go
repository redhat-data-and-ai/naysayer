package sandbox_personal

import (
	"github.com/redhat-data-and-ai/naysayer/internal/gitlab"
	"github.com/redhat-data-and-ai/naysayer/internal/rules/common"
	"github.com/redhat-data-and-ai/naysayer/internal/rules/shared"
)

// UnstructuredPipelineRule always auto-approves unstructured-data-pipeline.yaml files
// in sandbox environments for Personal UnstructuredDataProducts.
// This file is optional and always safe to approve whether it exists or not.
type UnstructuredPipelineRule struct {
	*common.BaseRule
	*common.ValidationHelper
	client    gitlab.GitLabClient
	mrContext *shared.MRContext
}

// NewUnstructuredPipelineRule creates a new unstructured pipeline rule instance
func NewUnstructuredPipelineRule(client gitlab.GitLabClient) *UnstructuredPipelineRule {
	return &UnstructuredPipelineRule{
		BaseRule:         common.NewBaseRule("sandbox_unstructured_pipeline_rule", "Always auto-approves sandbox unstructured-data-pipeline.yaml for Personal products"),
		ValidationHelper: common.NewValidationHelper(),
		client:           client,
	}
}

// SetMRContext implements the ContextAwareRule interface
func (r *UnstructuredPipelineRule) SetMRContext(mrCtx *shared.MRContext) {
	r.mrContext = mrCtx
}

// ValidateLines validates the unstructured pipeline file
func (r *UnstructuredPipelineRule) ValidateLines(filePath string, fileContent string, lineRanges []shared.LineRange) (shared.DecisionType, string) {
	if r.mrContext == nil {
		return r.CreateApprovalResult("Auto-approved: No MR context")
	}

	// Only apply when the associated product is a sandbox Personal UnstructuredDataProduct
	if !IsSandboxPersonalProductForFile(r.mrContext, r.client, filePath) {
		return r.CreateApprovalResult("Auto-approved: Not a sandbox Personal UnstructuredDataProduct")
	}

	// Always auto-approve - this file is optional and always safe
	return r.CreateApprovalResult("Auto-approved: Unstructured data pipeline configuration in sandbox personal environment")
}

// GetCoveredLines returns the full file coverage
func (r *UnstructuredPipelineRule) GetCoveredLines(filePath string, fileContent string) []shared.LineRange {
	// Cover the entire file
	return r.GetFullFileCoverage(filePath, fileContent)
}
