package access_request

import (
	"testing"

	"github.com/redhat-data-and-ai/naysayer/internal/gitlab"
	"github.com/redhat-data-and-ai/naysayer/internal/rules/shared"
	"github.com/stretchr/testify/assert"
)

const (
	testUserHellosource    = "mbramle"
	testUserHelloaggregate = "tvaldez"
)

func TestRule_Name(t *testing.T) {
	assert.Equal(t, "hello_access_request", NewRule().Name())
}

func TestRule_isAccessRequestFile(t *testing.T) {
	rule := NewRule()

	tests := []struct {
		path     string
		expected bool
	}{
		{
			"dataproducts/aggregate/helloaggregate/access-requests/groups/dataverse-aggregate-helloaggregate/" + testUserHelloaggregate + ".yaml",
			true,
		},
		{
			"dataproducts/source/hellosource/access-requests/groups/dataverse-source-hellosource/" + testUserHellosource + ".yaml",
			true,
		},
		{
			"dataproducts/aggregate/helloaggregate/groups/dataverse-aggregate-helloaggregate/user.yaml",
			false,
		},
		{
			"dataproducts/source/hellosource/access-requests/groups/other-group/user.yaml",
			false,
		},
		{"dataproducts/analytics/prod/product.yaml", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			assert.Equal(t, tt.expected, rule.isAccessRequestFile(tt.path))
		})
	}
}

func TestRule_ValidateLines(t *testing.T) {
	hellosourcePath := "dataproducts/source/hellosource/access-requests/groups/dataverse-source-hellosource/" + testUserHellosource + ".yaml"
	helloaggregatePath := "dataproducts/aggregate/helloaggregate/access-requests/groups/dataverse-aggregate-helloaggregate/" + testUserHelloaggregate + ".yaml"

	validHellosource := "name: " + testUserHellosource + "\ndata_product: hellosource\n"
	validHelloaggregate := "name: " + testUserHelloaggregate + "\ndata_product: helloaggregate\n"

	tests := []struct {
		name         string
		filePath     string
		content      string
		mrChanges    []gitlab.FileChange
		wantDecision shared.DecisionType
		wantSubstr   string
	}{
		{
			name:     "valid hellosource access request",
			filePath: hellosourcePath,
			content:  validHellosource,
			mrChanges: []gitlab.FileChange{
				{NewPath: hellosourcePath},
			},
			wantDecision: shared.Approve,
			wantSubstr:   "Auto-approved",
		},
		{
			name:     "valid helloaggregate access request",
			filePath: helloaggregatePath,
			content:  validHelloaggregate,
			mrChanges: []gitlab.FileChange{
				{NewPath: helloaggregatePath},
			},
			wantDecision: shared.Approve,
			wantSubstr:   "helloaggregate",
		},
		{
			name:     "name mismatch",
			filePath: hellosourcePath,
			content:  "name: wrong\ndata_product: hellosource\n",
			mrChanges: []gitlab.FileChange{
				{NewPath: hellosourcePath},
			},
			wantDecision: shared.ManualReview,
			wantSubstr:   "does not match filename",
		},
		{
			name:     "data_product mismatch",
			filePath: hellosourcePath,
			content:  "name: " + testUserHellosource + "\ndata_product: helloaggregate\n",
			mrChanges: []gitlab.FileChange{
				{NewPath: hellosourcePath},
			},
			wantDecision: shared.ManualReview,
			wantSubstr:   "data_product",
		},
		{
			name:     "MR with extra file",
			filePath: hellosourcePath,
			content:  validHellosource,
			mrChanges: []gitlab.FileChange{
				{NewPath: hellosourcePath},
				{NewPath: "dataproducts/source/hellosource/dev/product.yaml"},
			},
			wantDecision: shared.ManualReview,
			wantSubstr:   "outside allowed access-request paths",
		},
		{
			name:     "empty file",
			filePath: hellosourcePath,
			content:  "",
			mrChanges: []gitlab.FileChange{
				{NewPath: hellosourcePath},
			},
			wantDecision: shared.ManualReview,
			wantSubstr:   "deletion or empty",
		},
		{
			name:         "missing MR context",
			filePath:     hellosourcePath,
			content:      validHellosource,
			mrChanges:    nil,
			wantDecision: shared.ManualReview,
			wantSubstr:   "MR context not available",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rule := NewRule()
			if tt.mrChanges != nil {
				rule.SetMRContext(&shared.MRContext{Changes: tt.mrChanges})
			}

			decision, reason := rule.ValidateLines(
				tt.filePath,
				tt.content,
				[]shared.LineRange{{StartLine: 1, EndLine: 2, FilePath: tt.filePath}},
			)

			assert.Equal(t, tt.wantDecision, decision)
			assert.Contains(t, reason, tt.wantSubstr)
		})
	}
}

func TestRule_GetCoveredLines(t *testing.T) {
	rule := NewRule()
	path := "dataproducts/source/hellosource/access-requests/groups/dataverse-source-hellosource/" + testUserHellosource + ".yaml"
	content := "name: " + testUserHellosource + "\ndata_product: hellosource\n"

	ranges := rule.GetCoveredLines(path, content)
	assert.Len(t, ranges, 1)
	assert.Equal(t, 1, ranges[0].StartLine)
	assert.Equal(t, shared.CountLines(content), ranges[0].EndLine)

	assert.Nil(t, rule.GetCoveredLines("dataproducts/analytics/prod/product.yaml", content))
}
