package webhook

import (
	"testing"
	"time"

	"github.com/redhat-data-and-ai/naysayer/internal/config"
	"github.com/redhat-data-and-ai/naysayer/internal/gitlab"
	"github.com/redhat-data-and-ai/naysayer/internal/rules/shared"
	"github.com/stretchr/testify/assert"
)

func TestBuildApprovalComment_BasicVerbosity(t *testing.T) {
	cfg := &config.Config{
		Comments: config.CommentsConfig{
			CommentVerbosity: "basic",
		},
	}

	builder := NewMessageBuilder(cfg)

	result := &shared.RuleEvaluation{
		FinalDecision: shared.Decision{
			Type:   shared.Approve,
			Reason: "All warehouse changes are decreases",
		},
		FileValidations: map[string]*shared.FileValidationSummary{
			"test/product.yaml": {
				FilePath:     "test/product.yaml",
				TotalLines:   30,
				CoveredLines: []shared.LineRange{{StartLine: 1, EndLine: 30}},
				RuleResults: []shared.LineValidationResult{
					{
						RuleName:     "warehouse_rule",
						Decision:     shared.Approve,
						Reason:       "Warehouse decreases detected",
						LineRanges:   []shared.LineRange{{StartLine: 1, EndLine: 30}},
						WasEvaluated: true,
					},
				},
				FileDecision: shared.Approve,
			},
		},
		TotalFiles:     1,
		ApprovedFiles:  1,
		ReviewFiles:    0,
		UncoveredFiles: 0,
		ExecutionTime:  time.Millisecond * 150,
	}

	mrInfo := &gitlab.MRInfo{
		ProjectID: 123,
		MRIID:     456,
		Author:    "testuser",
		Title:     "Test warehouse decrease",
	}

	comment := builder.BuildApprovalComment(result, mrInfo)

	assert.Contains(t, comment, "✅ **Auto-approved**")
	assert.Contains(t, comment, "**What was checked:**")
	assert.Contains(t, comment, "Warehouse decreases detected")
}

func TestBuildApprovalComment_ContainsIdentifier(t *testing.T) {
	cfg := &config.Config{
		Comments: config.CommentsConfig{
			CommentVerbosity: "basic",
		},
	}

	builder := NewMessageBuilder(cfg)

	result := &shared.RuleEvaluation{
		FinalDecision: shared.Decision{
			Type:   shared.Approve,
			Reason: "All changes approved",
		},
		FileValidations: map[string]*shared.FileValidationSummary{},
		TotalFiles:      1,
		ApprovedFiles:   1,
		ExecutionTime:   time.Millisecond * 100,
	}

	mrInfo := &gitlab.MRInfo{
		ProjectID: 123,
		MRIID:     456,
		Author:    "test-user",
		Title:     "Test MR",
	}

	comment := builder.BuildApprovalComment(result, mrInfo)

	// Should contain the hidden identifier for approval comments
	assert.Contains(t, comment, "<!-- naysayer-comment-id: approval -->")
	assert.Contains(t, comment, "✅ **Auto-approved**")
}

func TestBuildManualReviewComment_ContainsIdentifier(t *testing.T) {
	cfg := &config.Config{
		Comments: config.CommentsConfig{
			CommentVerbosity: "detailed",
		},
	}

	builder := NewMessageBuilder(cfg)

	result := &shared.RuleEvaluation{
		FinalDecision: shared.Decision{
			Type:   shared.ManualReview,
			Reason: "High-risk changes detected",
		},
		FileValidations: map[string]*shared.FileValidationSummary{
			"test/product.yaml": {
				FilePath:     "test/product.yaml",
				TotalLines:   20,
				CoveredLines: []shared.LineRange{{StartLine: 1, EndLine: 20}},
				RuleResults: []shared.LineValidationResult{
					{
						RuleName:   "warehouse_rule",
						Decision:   shared.ManualReview,
						Reason:     "High-risk warehouse changes",
						LineRanges: []shared.LineRange{{StartLine: 10, EndLine: 15}},
					},
				},
				FileDecision: shared.ManualReview,
			},
		},
		TotalFiles:     1,
		ApprovedFiles:  0,
		ReviewFiles:    1,
		UncoveredFiles: 0,
		ExecutionTime:  time.Millisecond * 200,
	}

	mrInfo := &gitlab.MRInfo{
		ProjectID: 123,
		MRIID:     456,
		Author:    "test-user",
		Title:     "Test MR with high-risk changes",
	}

	comment := builder.BuildManualReviewComment(result, mrInfo)

	// Should contain the hidden identifier for manual review comments
	assert.Contains(t, comment, "<!-- naysayer-comment-id: manual-review -->")
	assert.Contains(t, comment, "Manual review required")
	assert.Contains(t, comment, "High-risk changes detected")
}

func TestBuildApprovalComment_DetailedVerbosity(t *testing.T) {
	cfg := &config.Config{
		Comments: config.CommentsConfig{
			CommentVerbosity: "detailed",
		},
	}

	builder := NewMessageBuilder(cfg)

	result := &shared.RuleEvaluation{
		FinalDecision: shared.Decision{
			Type:   shared.Approve,
			Reason: "Warehouse changes are safe",
		},
		FileValidations: map[string]*shared.FileValidationSummary{},
		TotalFiles:      1,
		ApprovedFiles:   1,
		ReviewFiles:     0,
		UncoveredFiles:  0,
		ExecutionTime:   time.Millisecond * 100,
	}

	mrInfo := &gitlab.MRInfo{
		ProjectID: 456,
		MRIID:     789,
		Author:    "developer",
		Title:     "Safe warehouse changes",
	}

	comment := builder.BuildApprovalComment(result, mrInfo)

	assert.Contains(t, comment, "✅ **Auto-approved**")
	assert.Contains(t, comment, "**What was checked:**")
	// Note: Files section only appears when there are 3+ files
}

func TestBuildManualReviewComment(t *testing.T) {
	cfg := &config.Config{
		Comments: config.CommentsConfig{
			CommentVerbosity: "detailed",
		},
	}

	builder := NewMessageBuilder(cfg)

	result := &shared.RuleEvaluation{
		FinalDecision: shared.Decision{
			Type:   shared.ManualReview,
			Reason: "Manual review required: warehouse size increase detected",
		},
		FileValidations: map[string]*shared.FileValidationSummary{
			"test/product.yaml": {
				FilePath:     "test/product.yaml",
				TotalLines:   30,
				CoveredLines: []shared.LineRange{{StartLine: 1, EndLine: 30}},
				RuleResults: []shared.LineValidationResult{
					{
						RuleName:   "warehouse_rule",
						Decision:   shared.ManualReview,
						Reason:     "Warehouse size increase detected",
						LineRanges: []shared.LineRange{{StartLine: 1, EndLine: 30}},
					},
				},
				FileDecision: shared.ManualReview,
			},
		},
		TotalFiles:     1,
		ApprovedFiles:  0,
		ReviewFiles:    1,
		UncoveredFiles: 0,
		ExecutionTime:  time.Millisecond * 200,
	}

	mrInfo := &gitlab.MRInfo{
		ProjectID: 123,
		MRIID:     456,
		Author:    "testuser",
		Title:     "Test warehouse increase",
	}

	comment := builder.BuildManualReviewComment(result, mrInfo)

	assert.Contains(t, comment, "⚠️ **Manual review required**")
	assert.Contains(t, comment, "**Why manual review is needed:**")
	assert.Contains(t, comment, "warehouse size increase detected")
	assert.Contains(t, comment, "**What was checked:**")
}
