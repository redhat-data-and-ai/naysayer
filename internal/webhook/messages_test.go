package webhook

import (
	"strings"
	"testing"
	"time"

	"github.com/redhat-data-and-ai/naysayer/internal/config"
	"github.com/redhat-data-and-ai/naysayer/internal/gitlab"
	"github.com/redhat-data-and-ai/naysayer/internal/rules/shared"
	"github.com/stretchr/testify/assert"
)

func TestNewMessageBuilder(t *testing.T) {
	cfg := &config.Config{
		Comments: config.CommentsConfig{
			EnableMRComments: true,
			CommentVerbosity: "detailed",
		},
	}

	builder := NewMessageBuilder(cfg)
	assert.NotNil(t, builder)
	assert.Equal(t, cfg, builder.config)
}

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
		RuleResults: []shared.RuleResult{
			{
				RuleName: "warehouse_rule",
				Decision: shared.Decision{
					Type:   shared.Approve,
					Reason: "Warehouse decreases detected",
				},
			},
		},
		ExecutionTime: time.Millisecond * 150,
	}

	mrInfo := &gitlab.MRInfo{
		ProjectID: 123,
		MRIID:     456,
		Author:    "testuser",
		Title:     "Test MR",
	}

	comment := builder.BuildApprovalComment(result, mrInfo)

	// Verify basic comment structure
	assert.Contains(t, comment, "✅ **Auto-approved after CI pipeline success**")
	assert.Contains(t, comment, "📊 **Analysis Results:**")
	assert.Contains(t, comment, "1 rules evaluated, all passed")
	assert.Contains(t, comment, "🤖 *Automated by NAYSAYER v1.0.0*")
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
			Reason: "All warehouse changes are decreases",
		},
		RuleResults: []shared.RuleResult{
			{
				RuleName: "warehouse_rule",
				Decision: shared.Decision{
					Type:   shared.Approve,
					Reason: "Warehouse decreases detected",
				},
				Metadata: map[string]any{
					"analyzed_files": []string{"dataproducts/agg/bookings/prod/product.yaml"},
					"warehouse_changes": []interface{}{"LARGE -> MEDIUM"},
				},
			},
		},
		ExecutionTime: time.Millisecond * 150,
	}

	mrInfo := &gitlab.MRInfo{
		ProjectID: 123,
		MRIID:     456,
		Author:    "testuser",
		Title:     "Test MR",
	}

	comment := builder.BuildApprovalComment(result, mrInfo)

	// Verify detailed comment structure
	assert.Contains(t, comment, "✅ **Auto-approved after CI pipeline success**")
	assert.Contains(t, comment, "📊 **Analysis Results:**")
	assert.Contains(t, comment, "✅ warehouse_rule: Warehouse decreases detected")
	assert.Contains(t, comment, "📄 **Files Analyzed:**")
	assert.Contains(t, comment, "dataproducts/agg/bookings/prod/product.yaml")
	assert.Contains(t, comment, "⏱️ **Processing Details:**")
	assert.Contains(t, comment, "Rule evaluation time: 150ms")
	assert.Contains(t, comment, "🤖 *Automated by NAYSAYER v1.0.0*")
}

func TestBuildApprovalComment_DebugVerbosity(t *testing.T) {
	cfg := &config.Config{
		Comments: config.CommentsConfig{
			CommentVerbosity: "debug",
		},
	}

	builder := NewMessageBuilder(cfg)
	
	result := &shared.RuleEvaluation{
		FinalDecision: shared.Decision{
			Type:   shared.Approve,
			Reason: "All warehouse changes are decreases",
		},
		RuleResults: []shared.RuleResult{
			{
				RuleName: "warehouse_rule",
				Decision: shared.Decision{
					Type:   shared.Approve,
					Reason: "Warehouse decreases detected",
				},
				ExecutionTime: time.Millisecond * 50,
				Metadata: map[string]any{
					"analyzed_files": []string{"dataproducts/agg/bookings/prod/product.yaml"},
				},
			},
		},
		ExecutionTime: time.Millisecond * 150,
	}

	mrInfo := &gitlab.MRInfo{
		ProjectID: 123,
		MRIID:     456,
		Author:    "testuser",
		Title:     "Test MR",
	}

	comment := builder.BuildApprovalComment(result, mrInfo)

	// Verify debug comment structure includes MR information
	assert.Contains(t, comment, "🔍 **MR Information:**")
	assert.Contains(t, comment, "Project ID: 123")
	assert.Contains(t, comment, "MR IID: 456")
	assert.Contains(t, comment, "Author: testuser")
	assert.Contains(t, comment, "Title: Test MR")
	assert.Contains(t, comment, "📊 **Detailed Analysis Results:**")
	assert.Contains(t, comment, "⚙️ **System Details:**")
}

func TestBuildApprovalMessage_WarehouseChanges(t *testing.T) {
	cfg := &config.Config{}
	builder := NewMessageBuilder(cfg)

	result := &shared.RuleEvaluation{
		RuleResults: []shared.RuleResult{
			{
				RuleName: "warehouse_rule",
				Decision: shared.Decision{Type: shared.Approve},
				Metadata: map[string]any{
					"warehouse_changes": []interface{}{"LARGE -> MEDIUM"},
				},
			},
		},
	}

	message := builder.BuildApprovalMessage(result)
	assert.Equal(t, "Auto-approved: Warehouse changes are safe (decreases only)", message)
}

func TestBuildApprovalMessage_DraftMR(t *testing.T) {
	cfg := &config.Config{}
	builder := NewMessageBuilder(cfg)

	result := &shared.RuleEvaluation{
		RuleResults: []shared.RuleResult{
			{
				RuleName: "draft_rule",
				Decision: shared.Decision{Type: shared.Approve},
				Metadata: map[string]any{
					"is_draft": true,
				},
			},
		},
	}

	message := builder.BuildApprovalMessage(result)
	assert.Equal(t, "Auto-approved: Draft MR with dataverse-safe changes", message)
}

func TestBuildApprovalMessage_AutomatedUser(t *testing.T) {
	cfg := &config.Config{}
	builder := NewMessageBuilder(cfg)

	result := &shared.RuleEvaluation{
		RuleResults: []shared.RuleResult{
			{
				RuleName: "automated_user_rule",
				Decision: shared.Decision{Type: shared.Approve},
				Metadata: map[string]any{
					"is_automated": true,
				},
			},
		},
	}

	message := builder.BuildApprovalMessage(result)
	assert.Equal(t, "Auto-approved: Automated user with passing CI", message)
}

func TestBuildApprovalMessage_DataverseFiles(t *testing.T) {
	cfg := &config.Config{}
	builder := NewMessageBuilder(cfg)

	result := &shared.RuleEvaluation{
		RuleResults: []shared.RuleResult{
			{
				RuleName: "warehouse_rule",
				Decision: shared.Decision{Type: shared.Approve},
			},
		},
	}

	message := builder.BuildApprovalMessage(result)
	assert.Equal(t, "Auto-approved: Only dataverse-safe files modified", message)
}

func TestBuildApprovalMessage_Default(t *testing.T) {
	cfg := &config.Config{}
	builder := NewMessageBuilder(cfg)

	result := &shared.RuleEvaluation{
		RuleResults: []shared.RuleResult{
			{
				RuleName: "generic_rule",
				Decision: shared.Decision{Type: shared.Approve},
			},
		},
	}

	message := builder.BuildApprovalMessage(result)
	assert.Equal(t, "Auto-approved: All rules passed after CI success", message)
}

func TestBuildRulesSummary(t *testing.T) {
	cfg := &config.Config{}
	builder := NewMessageBuilder(cfg)

	ruleResults := []shared.RuleResult{
		{
			RuleName: "warehouse_rule",
			Decision: shared.Decision{
				Type:   shared.Approve,
				Reason: "Warehouse decreases detected",
			},
		},
		{
			RuleName: "source_rule",
			Decision: shared.Decision{
				Type:   shared.ManualReview,
				Reason: "Manual review required",
			},
		},
	}

	summary := builder.buildRulesSummary(ruleResults)

	assert.Contains(t, summary, "✅ warehouse_rule: Warehouse decreases detected")
	assert.Contains(t, summary, "🚫 source_rule: Manual review required")
}

func TestBuildFilesSummary(t *testing.T) {
	cfg := &config.Config{}
	builder := NewMessageBuilder(cfg)

	result := &shared.RuleEvaluation{
		RuleResults: []shared.RuleResult{
			{
				RuleName: "warehouse_rule",
				Metadata: map[string]any{
					"analyzed_files": []string{
						"dataproducts/agg/bookings/prod/product.yaml",
						"dataproducts/agg/costs/dev/product.yaml",
					},
					"warehouse_changes": []interface{}{"LARGE->MEDIUM", "SMALL->XSMALL"},
				},
			},
		},
	}

	summary := builder.buildFilesSummary(result)

	assert.Contains(t, summary, "dataproducts/agg/bookings/prod/product.yaml")
	assert.Contains(t, summary, "dataproducts/agg/costs/dev/product.yaml")
	assert.Contains(t, summary, "2 warehouse changes detected")
}

func TestHasWarehouseChanges(t *testing.T) {
	cfg := &config.Config{}
	builder := NewMessageBuilder(cfg)

	// Test with warehouse changes
	resultWithChanges := &shared.RuleEvaluation{
		RuleResults: []shared.RuleResult{
			{
				RuleName: "warehouse_rule",
				Decision: shared.Decision{Type: shared.Approve},
				Metadata: map[string]any{
					"warehouse_changes": []interface{}{"LARGE->MEDIUM"},
				},
			},
		},
	}

	assert.True(t, builder.hasWarehouseChanges(resultWithChanges))

	// Test without warehouse changes
	resultWithoutChanges := &shared.RuleEvaluation{
		RuleResults: []shared.RuleResult{
			{
				RuleName: "source_rule",
				Decision: shared.Decision{Type: shared.Approve},
			},
		},
	}

	assert.False(t, builder.hasWarehouseChanges(resultWithoutChanges))
}

func TestComment_ContainsExpectedSections(t *testing.T) {
	cfg := &config.Config{
		Comments: config.CommentsConfig{
			CommentVerbosity: "detailed",
		},
	}

	builder := NewMessageBuilder(cfg)
	
	result := &shared.RuleEvaluation{
		FinalDecision: shared.Decision{
			Type:   shared.Approve,
			Reason: "All rules passed",
		},
		RuleResults: []shared.RuleResult{
			{
				RuleName: "warehouse_rule",
				Decision: shared.Decision{
					Type:   shared.Approve,
					Reason: "Safe warehouse changes",
				},
			},
		},
		ExecutionTime: time.Millisecond * 200,
	}

	mrInfo := &gitlab.MRInfo{
		ProjectID: 123,
		MRIID:     456,
		Author:    "testuser",
		Title:     "Test MR",
	}

	comment := builder.BuildApprovalComment(result, mrInfo)

	// Verify all expected sections are present
	expectedSections := []string{
		"✅ **Auto-approved after CI pipeline success**",
		"📊 **Analysis Results:**",
		"⏱️ **Processing Details:**",
		"🤖 *Automated by NAYSAYER v1.0.0*",
	}

	for _, section := range expectedSections {
		assert.Contains(t, comment, section, "Comment should contain section: %s", section)
	}

	// Verify comment is properly formatted (has multiple lines)
	lines := strings.Split(comment, "\n")
	assert.Greater(t, len(lines), 5, "Comment should have multiple lines")
}