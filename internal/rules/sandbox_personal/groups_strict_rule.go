package sandbox_personal

import (
	"fmt"

	"github.com/redhat-data-and-ai/naysayer/internal/gitlab"
	"github.com/redhat-data-and-ai/naysayer/internal/logging"
	"github.com/redhat-data-and-ai/naysayer/internal/rules/common"
	"github.com/redhat-data-and-ai/naysayer/internal/rules/shared"
)

// GroupsStrictRule always requires manual review for changes to product-level groups/ folder
// for Personal UnstructuredDataProducts in sandbox environments.
// This prevents automatic approval of consumer group changes.
type GroupsStrictRule struct {
	*common.BaseRule
	*common.ValidationHelper
	client    gitlab.GitLabClient
	mrContext *shared.MRContext
}

// NewGroupsStrictRule creates a new groups strict rule
func NewGroupsStrictRule(client gitlab.GitLabClient) *GroupsStrictRule {
	return &GroupsStrictRule{
		BaseRule:         common.NewBaseRule("sandbox_groups_strict_rule", "Requires manual review for all groups/ folder changes in sandbox Personal products"),
		ValidationHelper: common.NewValidationHelper(),
		client:           client,
	}
}

// SetMRContext implements the ContextAwareRule interface
func (r *GroupsStrictRule) SetMRContext(mrCtx *shared.MRContext) {
	r.mrContext = mrCtx
}

// ValidateLines always requires manual review for groups files
func (r *GroupsStrictRule) ValidateLines(filePath string, fileContent string, lineRanges []shared.LineRange) (shared.DecisionType, string) {
	if r.mrContext == nil {
		return r.CreateApprovalResult("Auto-approved: No MR context")
	}

	// Only apply when the associated product is a sandbox Personal UnstructuredDataProduct
	if !IsSandboxPersonalProductForFile(r.mrContext, r.client, filePath) {
		return r.CreateApprovalResult("Auto-approved: Not a sandbox Personal UnstructuredDataProduct")
	}

	// Always require manual review for groups folder changes
	reason := fmt.Sprintf("Manual review required: Changes to product-level groups/ folder require manual review: %s", filePath)
	logging.Info("[%s] %s", r.Name(), reason)
	return r.CreateManualReviewResult(reason)
}

// GetCoveredLines returns the full file coverage
func (r *GroupsStrictRule) GetCoveredLines(filePath string, fileContent string) []shared.LineRange {
	return r.GetFullFileCoverage(filePath, fileContent)
}
