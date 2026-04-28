package developer_access

import (
	"testing"

	"github.com/redhat-data-and-ai/naysayer/internal/rules/shared"
	"github.com/stretchr/testify/assert"
)

func TestRule_Name(t *testing.T) {
	rule := NewRule()
	assert.Equal(t, "developer_access_rule", rule.Name())
}

func TestRule_GetCoveredLines(t *testing.T) {
	rule := NewRule()

	tests := []struct {
		name        string
		filePath    string
		fileContent string
		expected    int
	}{
		{
			name:        "valid access request file",
			filePath:    "dataproducts/aggregate/payments/access-requests/groups/dataverse-aggregate-payments/alice.yaml",
			fileContent: "name: alice\ndata_product: payments\n",
			expected:    1,
		},
		{
			name:        "valid source access request file",
			filePath:    "dataproducts/source/payments/access-requests/groups/dataverse-source-payments/alice.yaml",
			fileContent: "name: alice\ndata_product: payments\n",
			expected:    1,
		},
		{
			name:        "non matching path",
			filePath:    "dataproducts/aggregate/payments/product.yaml",
			fileContent: "name: alice\ndata_product: payments\n",
			expected:    0,
		},
		{
			name:        "empty content",
			filePath:    "dataproducts/aggregate/payments/access-requests/groups/dataverse-aggregate-payments/alice.yaml",
			fileContent: "   \n",
			expected:    0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lines := rule.GetCoveredLines(tt.filePath, tt.fileContent)
			assert.Len(t, lines, tt.expected)
		})
	}
}

func TestRule_ValidateLines(t *testing.T) {
	rule := NewRule()
	testLineRanges := []shared.LineRange{{StartLine: 1, EndLine: 2}}

	tests := []struct {
		name        string
		filePath    string
		fileContent string
		decision    shared.DecisionType
		reasonPart  string
	}{
		{
			name:     "approves when path and content match",
			filePath: "dataproducts/aggregate/payments/access-requests/groups/dataverse-aggregate-payments/alice.yaml",
			fileContent: `name: alice
data_product: payments
`,
			decision:   shared.Approve,
			reasonPart: "validated",
		},
		{
			name:     "approves source path when content matches",
			filePath: "dataproducts/source/payments/access-requests/groups/dataverse-source-payments/alice.yaml",
			fileContent: `name: alice
data_product: payments
`,
			decision:   shared.Approve,
			reasonPart: "validated",
		},
		{
			name:     "manual review when path does not match",
			filePath: "dataproducts/aggregate/payments/access-requests/groups/dataverse-aggregate-payments/alice.txt",
			fileContent: `name: alice
data_product: payments
`,
			decision:   shared.ManualReview,
			reasonPart: "Not a developer access-request file",
		},
		{
			name:     "manual review when YAML invalid",
			filePath: "dataproducts/aggregate/payments/access-requests/groups/dataverse-aggregate-payments/alice.yaml",
			fileContent: `name: alice
data_product: [payments
`,
			decision:   shared.ManualReview,
			reasonPart: "Failed to parse access-request YAML",
		},
		{
			name:     "manual review when name mismatches username",
			filePath: "dataproducts/aggregate/payments/access-requests/groups/dataverse-aggregate-payments/alice.yaml",
			fileContent: `name: bob
data_product: payments
`,
			decision:   shared.ManualReview,
			reasonPart: "does not match username",
		},
		{
			name:     "manual review when data_product mismatches path",
			filePath: "dataproducts/aggregate/payments/access-requests/groups/dataverse-aggregate-payments/alice.yaml",
			fileContent: `name: alice
data_product: orders
`,
			decision:   shared.ManualReview,
			reasonPart: "does not match dataproduct",
		},
		{
			name:     "manual review when name missing",
			filePath: "dataproducts/aggregate/payments/access-requests/groups/dataverse-aggregate-payments/alice.yaml",
			fileContent: `data_product: payments
`,
			decision:   shared.ManualReview,
			reasonPart: "Missing required field: name",
		},
		{
			name:     "manual review when data_product missing",
			filePath: "dataproducts/aggregate/payments/access-requests/groups/dataverse-aggregate-payments/alice.yaml",
			fileContent: `name: alice
`,
			decision:   shared.ManualReview,
			reasonPart: "Missing required field: data_product",
		},
		{
			name:     "manual review when name empty",
			filePath: "dataproducts/aggregate/payments/access-requests/groups/dataverse-aggregate-payments/alice.yaml",
			fileContent: `name: "   "
data_product: payments
`,
			decision:   shared.ManualReview,
			reasonPart: "Missing required field: name",
		},
		{
			name:     "manual review when data_product empty",
			filePath: "dataproducts/aggregate/payments/access-requests/groups/dataverse-aggregate-payments/alice.yaml",
			fileContent: `name: alice
data_product: ""
`,
			decision:   shared.ManualReview,
			reasonPart: "Missing required field: data_product",
		},
		{
			name:     "manual review when group does not match convention",
			filePath: "dataproducts/aggregate/payments/access-requests/groups/wrong-group/alice.yaml",
			fileContent: `name: alice
data_product: payments
`,
			decision:   shared.ManualReview,
			reasonPart: "does not match expected group",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			decision, reason := rule.ValidateLines(tt.filePath, tt.fileContent, testLineRanges)
			assert.Equal(t, tt.decision, decision)
			assert.Contains(t, reason, tt.reasonPart)
		})
	}
}
