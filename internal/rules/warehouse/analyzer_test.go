package warehouse

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/redhat-data-and-ai/naysayer/internal/gitlab"
)

func TestNewAnalyzer(t *testing.T) {
	client := &gitlab.Client{}
	analyzer := NewAnalyzer(client)

	assert.NotNil(t, analyzer)
	assert.Equal(t, client, analyzer.gitlabClient)
}

func TestAnalyzer_parseDataProduct(t *testing.T) {
	analyzer := NewAnalyzer(nil)

	tests := []struct {
		name          string
		yamlContent   string
		expected      *DataProduct
		expectedError bool
	}{
		{
			name: "valid yaml with multiple warehouses",
			yamlContent: `
name: "multi-warehouse-product"
rover_group: "analytics"
warehouses:
  - type: "snowflake"
    size: "LARGE"
  - type: "redshift"
    size: "XLARGE"
  - type: "bigquery"
    size: "SMALL"
tags:
  data_product: "analytics"
`,
			expected: &DataProduct{
				Name:       "multi-warehouse-product",
				RoverGroup: "analytics",
				Warehouses: []Warehouse{
					{Type: "snowflake", Size: "LARGE"},
					{Type: "redshift", Size: "XLARGE"},
					{Type: "bigquery", Size: "SMALL"},
				},
				Tags: Tags{DataProduct: "analytics"},
			},
			expectedError: false,
		},
		{
			name: "minimal valid yaml with empty warehouses",
			yamlContent: `
name: "minimal"
rover_group: "test"
warehouses: []
`,
			expected: &DataProduct{
				Name:       "minimal",
				RoverGroup: "test",
				Warehouses: []Warehouse{},
				Tags:       Tags{},
			},
			expectedError: false,
		},
		{
			name: "invalid yaml syntax",
			yamlContent: `
name: "broken
rover_group: "test
warehouses:
  - type: snowflake"
`,
			expected:      nil,
			expectedError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := analyzer.parseDataProduct(tt.yamlContent)

			if tt.expectedError {
				assert.Error(t, err, "parseDataProduct() should return an error")
				assert.Nil(t, result, "parseDataProduct() should return nil on error")
			} else {
				assert.NoError(t, err, "parseDataProduct() should not return an error")
				assert.Equal(t, tt.expected, result, "parseDataProduct() result mismatch")
			}
		})
	}
}

func TestAnalyzer_compareWarehouses(t *testing.T) {
	analyzer := NewAnalyzer(nil)
	filePath := "dataproducts/agg/test/product.yaml"

	tests := []struct {
		name     string
		oldDP    *DataProduct
		newDP    *DataProduct
		expected []WarehouseChange
	}{
		{
			name: "no changes",
			oldDP: &DataProduct{
				Warehouses: []Warehouse{
					{Type: "snowflake", Size: "MEDIUM"},
				},
			},
			newDP: &DataProduct{
				Warehouses: []Warehouse{
					{Type: "snowflake", Size: "MEDIUM"},
				},
			},
			expected: []WarehouseChange{},
		},
		{
			name: "size increase",
			oldDP: &DataProduct{
				Warehouses: []Warehouse{
					{Type: "snowflake", Size: "MEDIUM"},
				},
			},
			newDP: &DataProduct{
				Warehouses: []Warehouse{
					{Type: "snowflake", Size: "LARGE"},
				},
			},
			expected: []WarehouseChange{
				{
					FilePath:   "dataproducts/agg/test/product.yaml (type: snowflake)",
					FromSize:   "MEDIUM",
					ToSize:     "LARGE",
					IsDecrease: false,
				},
			},
		},
		{
			name: "size decrease",
			oldDP: &DataProduct{
				Warehouses: []Warehouse{
					{Type: "snowflake", Size: "XLARGE"},
				},
			},
			newDP: &DataProduct{
				Warehouses: []Warehouse{
					{Type: "snowflake", Size: "LARGE"},
				},
			},
			expected: []WarehouseChange{
				{
					FilePath:   "dataproducts/agg/test/product.yaml (type: snowflake)",
					FromSize:   "XLARGE",
					ToSize:     "LARGE",
					IsDecrease: true,
				},
			},
		},
		{
			name: "multiple warehouse changes",
			oldDP: &DataProduct{
				Warehouses: []Warehouse{
					{Type: "snowflake", Size: "MEDIUM"},
					{Type: "redshift", Size: "LARGE"},
					{Type: "bigquery", Size: "SMALL"},
				},
			},
			newDP: &DataProduct{
				Warehouses: []Warehouse{
					{Type: "snowflake", Size: "LARGE"}, // increase
					{Type: "redshift", Size: "MEDIUM"}, // decrease
					{Type: "bigquery", Size: "SMALL"},  // no change
				},
			},
			expected: []WarehouseChange{
				{
					FilePath:   "dataproducts/agg/test/product.yaml (type: snowflake)",
					FromSize:   "MEDIUM",
					ToSize:     "LARGE",
					IsDecrease: false,
				},
				{
					FilePath:   "dataproducts/agg/test/product.yaml (type: redshift)",
					FromSize:   "LARGE",
					ToSize:     "MEDIUM",
					IsDecrease: true,
				},
			},
		},
		{
			name: "new warehouse added",
			oldDP: &DataProduct{
				Warehouses: []Warehouse{
					{Type: "snowflake", Size: "MEDIUM"},
				},
			},
			newDP: &DataProduct{
				Warehouses: []Warehouse{
					{Type: "snowflake", Size: "MEDIUM"},
					{Type: "redshift", Size: "LARGE"},
				},
			},
			expected: []WarehouseChange{
				{
					FilePath:   "dataproducts/agg/test/product.yaml (type: redshift)",
					FromSize:   "",
					ToSize:     "LARGE",
					IsDecrease: false,
				},
			},
		},
		{
			name: "warehouse removed",
			oldDP: &DataProduct{
				Warehouses: []Warehouse{
					{Type: "snowflake", Size: "MEDIUM"},
					{Type: "redshift", Size: "LARGE"},
				},
			},
			newDP: &DataProduct{
				Warehouses: []Warehouse{
					{Type: "snowflake", Size: "MEDIUM"},
				},
			},
			expected: []WarehouseChange{},
		},
		{
			name: "unknown warehouse size",
			oldDP: &DataProduct{
				Warehouses: []Warehouse{
					{Type: "snowflake", Size: "UNKNOWN_SIZE"},
				},
			},
			newDP: &DataProduct{
				Warehouses: []Warehouse{
					{Type: "snowflake", Size: "MEDIUM"},
				},
			},
			expected: []WarehouseChange{},
		},
		{
			name: "empty warehouses",
			oldDP: &DataProduct{
				Warehouses: []Warehouse{},
			},
			newDP: &DataProduct{
				Warehouses: []Warehouse{},
			},
			expected: []WarehouseChange{},
		},
		{
			name: "new file with warehouses (like rosettastone)",
			oldDP: &DataProduct{
				Warehouses: []Warehouse{}, // Empty - new file scenario
			},
			newDP: &DataProduct{
				Warehouses: []Warehouse{
					{Type: "user", Size: "XSMALL"},
					{Type: "service_account", Size: "XSMALL"},
				},
			},
			expected: []WarehouseChange{
				{
					FilePath:   "dataproducts/agg/test/product.yaml (type: user)",
					FromSize:   "",
					ToSize:     "XSMALL",
					IsDecrease: false,
				},
				{
					FilePath:   "dataproducts/agg/test/product.yaml (type: service_account)",
					FromSize:   "",
					ToSize:     "XSMALL",
					IsDecrease: false,
				},
			},
		},
		{
			name: "extreme size changes",
			oldDP: &DataProduct{
				Warehouses: []Warehouse{
					{Type: "snowflake", Size: "XSMALL"},
					{Type: "redshift", Size: "X6LARGE"},
				},
			},
			newDP: &DataProduct{
				Warehouses: []Warehouse{
					{Type: "snowflake", Size: "X6LARGE"},
					{Type: "redshift", Size: "XSMALL"},
				},
			},
			expected: []WarehouseChange{
				{
					FilePath:   "dataproducts/agg/test/product.yaml (type: snowflake)",
					FromSize:   "XSMALL",
					ToSize:     "X6LARGE",
					IsDecrease: false,
				},
				{
					FilePath:   "dataproducts/agg/test/product.yaml (type: redshift)",
					FromSize:   "X6LARGE",
					ToSize:     "XSMALL",
					IsDecrease: true,
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := analyzer.compareWarehouses(filePath, tt.oldDP, tt.newDP)
			assert.ElementsMatch(t, tt.expected, result, "compareWarehouses() result mismatch")
		})
	}
}

func TestAnalyzer_AnalyzeChanges_FilteringLogic(t *testing.T) {
	// Create mock client that will return specific responses
	var mockClient GitLabClientInterface = &MockGitLabClient{}
	analyzer := NewAnalyzer(mockClient)

	tests := []struct {
		name     string
		changes  []gitlab.FileChange
		expected []WarehouseChange
	}{
		{
			name:     "no changes",
			changes:  []gitlab.FileChange{},
			expected: []WarehouseChange{},
		},
		{
			name: "only non-dataproduct files",
			changes: []gitlab.FileChange{
				{NewPath: "README.md"},
				{NewPath: "config/settings.yaml"},
				{NewPath: "src/main.go"},
			},
			expected: []WarehouseChange{},
		},
		{
			name: "deleted dataproduct file",
			changes: []gitlab.FileChange{
				{
					OldPath:     "dataproducts/agg/test/product.yaml",
					NewPath:     "",
					DeletedFile: true,
				},
			},
			expected: []WarehouseChange{},
		},
		{
			name: "mixed file types with deleted files",
			changes: []gitlab.FileChange{
				{NewPath: "README.md"},
				{
					OldPath:     "dataproducts/agg/old/product.yaml",
					NewPath:     "",
					DeletedFile: true,
				},
				{NewPath: "dataproducts/source/test/sourcebinding.yaml"},
			},
			expected: []WarehouseChange{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := analyzer.AnalyzeChanges(123, 456, tt.changes)
			assert.NoError(t, err, "AnalyzeChanges should not return error for filtering tests")
			assert.Equal(t, tt.expected, result, "AnalyzeChanges filtering result mismatch")
		})
	}
}

func TestAnalyzer_analyzeFileChange_ErrorHandling(t *testing.T) {
	tests := []struct {
		name           string
		mockClient     *MockGitLabClient
		expectedError  string
		expectedResult *[]WarehouseChange
	}{
		{
			name: "get target branch error",
			mockClient: &MockGitLabClient{
				targetBranchError: fmt.Errorf("network timeout"),
			},
			expectedError:  "failed to get target branch: network timeout",
			expectedResult: nil,
		},
		{
			name: "fetch old file content error",
			mockClient: &MockGitLabClient{
				targetBranch: "main",
				oldFileError: fmt.Errorf("API rate limit"),
				mrDetails:    &gitlab.MRDetails{SourceBranch: "feature", ProjectID: 123, SourceProjectID: 123, TargetProjectID: 123},
			},
			expectedError:  "failed to fetch old file content from target project 123, branch main: API rate limit",
			expectedResult: nil,
		},
		{
			name: "file not found - should be handled gracefully",
			mockClient: &MockGitLabClient{
				targetBranch:   "main",
				oldFileError:   fmt.Errorf("file not found"),
				mrDetails:      &gitlab.MRDetails{SourceBranch: "feature", ProjectID: 123, SourceProjectID: 123, TargetProjectID: 123},
				newFileContent: &gitlab.FileContent{Content: "name: test\nrover_group: test"},
			},
			expectedError:  "",
			expectedResult: &[]WarehouseChange{},
		},
		{
			name: "get MR details error",
			mockClient: &MockGitLabClient{
				targetBranch:   "main",
				oldFileContent: &gitlab.FileContent{Content: "name: test\nrover_group: test"},
				mrDetailsError: fmt.Errorf("unauthorized"),
			},
			expectedError:  "failed to get MR details: unauthorized",
			expectedResult: nil,
		},
		{
			name: "fetch new file content error",
			mockClient: &MockGitLabClient{
				targetBranch:   "main",
				oldFileContent: &gitlab.FileContent{Content: "name: test\nrover_group: test"},
				mrDetails:      &gitlab.MRDetails{SourceBranch: "feature", ProjectID: 123, SourceProjectID: 123, TargetProjectID: 123},
				newFileError:   fmt.Errorf("file corrupted"),
			},
			expectedError:  "failed to fetch new file content from source project 123, branch feature: file corrupted",
			expectedResult: nil,
		},
		{
			name: "invalid old YAML content",
			mockClient: &MockGitLabClient{
				targetBranch:   "main",
				oldFileContent: &gitlab.FileContent{Content: "invalid: yaml: content:"},
				mrDetails:      &gitlab.MRDetails{SourceBranch: "feature", ProjectID: 123, SourceProjectID: 123, TargetProjectID: 123},
				newFileContent: &gitlab.FileContent{Content: "name: test\nrover_group: test"},
			},
			expectedError:  "failed to parse old YAML:",
			expectedResult: nil,
		},
		{
			name: "invalid new YAML content",
			mockClient: &MockGitLabClient{
				targetBranch:   "main",
				oldFileContent: &gitlab.FileContent{Content: "name: test\nrover_group: test"},
				mrDetails:      &gitlab.MRDetails{SourceBranch: "feature", ProjectID: 123, SourceProjectID: 123, TargetProjectID: 123},
				newFileContent: &gitlab.FileContent{Content: "invalid: yaml: content:"},
			},
			expectedError:  "failed to parse new YAML:",
			expectedResult: nil,
		},
		{
			name: "cross-fork MR - different source project",
			mockClient: &MockGitLabClient{
				targetBranch:   "main",
				oldFileContent: &gitlab.FileContent{Content: "name: test\nrover_group: test\nwarehouses:\n  - type: snowflake\n    size: MEDIUM"},
				mrDetails:      &gitlab.MRDetails{SourceBranch: "feature", ProjectID: 123, SourceProjectID: 456, TargetProjectID: 123},
				newFileContent: &gitlab.FileContent{Content: "name: test\nrover_group: test\nwarehouses:\n  - type: snowflake\n    size: LARGE"},
			},
			expectedError: "",
			expectedResult: &[]WarehouseChange{{
				FilePath:   "dataproducts/agg/test/product.yaml (type: snowflake)",
				FromSize:   "MEDIUM",
				ToSize:     "LARGE",
				IsDecrease: false,
			}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var mockClient GitLabClientInterface = tt.mockClient
			analyzer := NewAnalyzer(mockClient)
			result, err := analyzer.analyzeFileChange(123, 456, "dataproducts/agg/test/product.yaml")

			if tt.expectedError != "" {
				assert.Error(t, err, "analyzeFileChange should return error")
				assert.Contains(t, err.Error(), tt.expectedError, "Error message should contain expected text")
				assert.Nil(t, result, "Result should be nil on error")
			} else {
				assert.NoError(t, err, "analyzeFileChange should not return error")
				assert.Equal(t, tt.expectedResult, result, "Result mismatch")
			}
		})
	}
}

// MockGitLabClient is a test implementation of the GitLab client interface
type MockGitLabClient struct {
	targetBranch       string
	targetBranchError  error
	oldFileContent     *gitlab.FileContent
	oldFileError       error
	newFileContent     *gitlab.FileContent
	newFileError       error
	mrDetails          *gitlab.MRDetails
	mrDetailsError     error
	lastFetchProjectID int    // Track which project ID was used for last fetch
	lastFetchBranch    string // Track which branch was used for last fetch
}

func (m *MockGitLabClient) GetMRTargetBranch(projectID, mrIID int) (string, error) {
	if m.targetBranchError != nil {
		return "", m.targetBranchError
	}
	return m.targetBranch, nil
}

func (m *MockGitLabClient) FetchFileContent(projectID int, filePath, branch string) (*gitlab.FileContent, error) {
	// Track the last fetch call
	m.lastFetchProjectID = projectID
	m.lastFetchBranch = branch

	// Return different content based on which branch is requested
	// This is a simple way to distinguish between old and new content requests
	if branch == m.targetBranch && m.oldFileError != nil {
		return nil, m.oldFileError
	}
	if branch == m.targetBranch && m.oldFileContent != nil {
		return m.oldFileContent, nil
	}

	if m.newFileError != nil {
		return nil, m.newFileError
	}
	return m.newFileContent, nil
}

func (m *MockGitLabClient) GetMRDetails(projectID, mrIID int) (*gitlab.MRDetails, error) {
	if m.mrDetailsError != nil {
		return nil, m.mrDetailsError
	}
	return m.mrDetails, nil
}

// Stub implementations for interface compliance (not used in warehouse tests)
func (m *MockGitLabClient) FetchMRChanges(projectID, mrIID int) ([]gitlab.FileChange, error) {
	return []gitlab.FileChange{}, nil
}

func (m *MockGitLabClient) AddMRComment(projectID, mrIID int, comment string) error {
	return nil
}

func (m *MockGitLabClient) AddOrUpdateMRComment(projectID, mrIID int, commentBody, commentType string) error {
	return nil
}

func (m *MockGitLabClient) ListMRComments(projectID, mrIID int) ([]gitlab.MRComment, error) {
	return []gitlab.MRComment{}, nil
}

func (m *MockGitLabClient) UpdateMRComment(projectID, mrIID, commentID int, newBody string) error {
	return nil
}

func (m *MockGitLabClient) FindLatestNaysayerComment(projectID, mrIID int, commentType ...string) (*gitlab.MRComment, error) {
	return nil, nil
}

func (m *MockGitLabClient) ApproveMR(projectID, mrIID int) error {
	return nil
}

func (m *MockGitLabClient) ApproveMRWithMessage(projectID, mrIID int, message string) error {
	return nil
}

func (m *MockGitLabClient) ResetNaysayerApproval(projectID, mrIID int) error {
	return nil
}

func (m *MockGitLabClient) GetCurrentBotUsername() (string, error) {
	return "naysayer-bot", nil
}

func (m *MockGitLabClient) IsNaysayerBotAuthor(author map[string]interface{}) bool {
	return false
}

func (m *MockGitLabClient) RebaseMR(projectID, mrIID int) error {
	return nil
}

func TestAnalyzer_hasNonWarehouseChanges(t *testing.T) {
	analyzer := NewAnalyzer(nil)

	tests := []struct {
		name       string
		oldContent string
		newContent string
		oldDP      *DataProduct
		newDP      *DataProduct
		expected   bool
	}{
		{
			name: "only warehouse size changes",
			oldContent: `name: test
warehouses:
  - type: snowflake
    size: LARGE
tags:
  data_product: test`,
			newContent: `name: test
warehouses:
  - type: snowflake
    size: MEDIUM
tags:
  data_product: test`,
			oldDP: &DataProduct{
				Name:       "test",
				Warehouses: []Warehouse{{Type: "snowflake", Size: "LARGE"}},
				Tags:       Tags{DataProduct: "test"},
			},
			newDP: &DataProduct{
				Name:       "test",
				Warehouses: []Warehouse{{Type: "snowflake", Size: "MEDIUM"}},
				Tags:       Tags{DataProduct: "test"},
			},
			expected: false,
		},
		{
			name: "name change detected",
			oldContent: `name: old-name
warehouses:
  - type: snowflake
    size: LARGE`,
			newContent: `name: new-name
warehouses:
  - type: snowflake
    size: LARGE`,
			oldDP: &DataProduct{
				Name:       "old-name",
				Warehouses: []Warehouse{{Type: "snowflake", Size: "LARGE"}},
			},
			newDP: &DataProduct{
				Name:       "new-name",
				Warehouses: []Warehouse{{Type: "snowflake", Size: "LARGE"}},
			},
			expected: true,
		},
		{
			name: "consumer changes detected (unparsed field)",
			oldContent: `name: test
warehouses:
  - type: snowflake
    size: LARGE
data_product_db:
  - database: test_db
    consumers:
      - name: consumer1`,
			newContent: `name: test
warehouses:
  - type: snowflake
    size: LARGE
data_product_db:
  - database: test_db
    consumers:
      - name: consumer1
      - name: consumer2`,
			oldDP: &DataProduct{
				Name:       "test",
				Warehouses: []Warehouse{{Type: "snowflake", Size: "LARGE"}},
			},
			newDP: &DataProduct{
				Name:       "test",
				Warehouses: []Warehouse{{Type: "snowflake", Size: "LARGE"}},
			},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := analyzer.hasNonWarehouseChanges(tt.oldContent, tt.newContent, tt.oldDP, tt.newDP)
			assert.Equal(t, tt.expected, result, "hasNonWarehouseChanges() result mismatch")
		})
	}
}
