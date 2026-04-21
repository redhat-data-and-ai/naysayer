package dataproduct_consumer

import (
	"fmt"
	"strings"
	"testing"

	"github.com/redhat-data-and-ai/naysayer/internal/gitlab"
	"github.com/redhat-data-and-ai/naysayer/internal/rules/shared"
	"github.com/stretchr/testify/assert"
)

// mockGitLabClient implements gitlab.GitLabClient for testing consumer group file validation
type mockGitLabClient struct {
	existingFiles map[string]map[string]bool // branch -> filePath -> exists
}

func newMockGitLabClient(files map[string]map[string]bool) *mockGitLabClient {
	return &mockGitLabClient{existingFiles: files}
}

func (m *mockGitLabClient) FetchFileContent(projectID int, filePath, ref string) (*gitlab.FileContent, error) {
	if branchFiles, ok := m.existingFiles[ref]; ok {
		if branchFiles[filePath] {
			return &gitlab.FileContent{Content: "mock content"}, nil
		}
	}
	return nil, fmt.Errorf("file not found: %s on branch %s", filePath, ref)
}

func (m *mockGitLabClient) GetMRTargetBranch(projectID, mrIID int) (string, error) {
	return "main", nil
}
func (m *mockGitLabClient) GetMRDetails(projectID, mrIID int) (*gitlab.MRDetails, error) {
	return nil, nil
}
func (m *mockGitLabClient) FetchMRChanges(projectID, mrIID int) ([]gitlab.FileChange, error) {
	return nil, nil
}
func (m *mockGitLabClient) AddMRComment(projectID, mrIID int, comment string) error { return nil }
func (m *mockGitLabClient) AddOrUpdateMRComment(projectID, mrIID int, commentBody, commentType string) error {
	return nil
}
func (m *mockGitLabClient) ListMRComments(projectID, mrIID int) ([]gitlab.MRComment, error) {
	return nil, nil
}
func (m *mockGitLabClient) UpdateMRComment(projectID, mrIID, commentID int, newBody string) error {
	return nil
}
func (m *mockGitLabClient) FindLatestNaysayerComment(projectID, mrIID int, commentType ...string) (*gitlab.MRComment, error) {
	return nil, nil
}
func (m *mockGitLabClient) ApproveMR(projectID, mrIID int) error { return nil }
func (m *mockGitLabClient) ApproveMRWithMessage(projectID, mrIID int, message string) error {
	return nil
}
func (m *mockGitLabClient) ResetNaysayerApproval(projectID, mrIID int) error { return nil }
func (m *mockGitLabClient) GetCurrentBotUsername() (string, error)           { return "", nil }
func (m *mockGitLabClient) IsNaysayerBotAuthor(author map[string]interface{}) bool {
	return false
}
func (m *mockGitLabClient) RebaseMR(projectID, mrIID int) (bool, error) { return false, nil }
func (m *mockGitLabClient) CompareBranches(sourceProjectID int, sourceBranch string, targetProjectID int, targetBranch string) (*gitlab.CompareResult, error) {
	return nil, nil
}
func (m *mockGitLabClient) GetBranchCommit(projectID int, branch string) (string, error) {
	return "", nil
}
func (m *mockGitLabClient) CompareCommits(projectID int, fromSHA, toSHA string) (*gitlab.CompareResult, error) {
	return nil, nil
}
func (m *mockGitLabClient) ListOpenMRs(projectID int) ([]int, error) { return nil, nil }
func (m *mockGitLabClient) ListOpenMRsWithDetails(projectID int) ([]gitlab.MRDetails, error) {
	return nil, nil
}
func (m *mockGitLabClient) GetPipelineJobs(projectID, pipelineID int) ([]gitlab.PipelineJob, error) {
	return nil, nil
}
func (m *mockGitLabClient) GetJobTrace(projectID, jobID int) (string, error) { return "", nil }
func (m *mockGitLabClient) FindLatestAtlantisComment(projectID, mrIID int) (*gitlab.MRComment, error) {
	return nil, nil
}
func (m *mockGitLabClient) AreAllPipelineJobsSucceeded(projectID, pipelineID int) (bool, error) {
	return false, nil
}
func (m *mockGitLabClient) CheckAtlantisCommentForPlanFailures(projectID, mrIID int) (bool, string) {
	return false, ""
}
func (m *mockGitLabClient) ListAllOpenMRsWithDetails(projectID int) ([]gitlab.MRDetails, error) {
	return nil, nil
}
func (m *mockGitLabClient) CloseMR(projectID, mrIID int) error { return nil }
func (m *mockGitLabClient) FindCommentByPattern(projectID, mrIID int, pattern string) (bool, error) {
	return false, nil
}
func (m *mockGitLabClient) FileExists(projectID int, filePath, ref string) (bool, error) {
	if branchFiles, ok := m.existingFiles[ref]; ok {
		return branchFiles[filePath], nil
	}
	return false, nil
}
func (m *mockGitLabClient) ListDirectoryFiles(projectID int, dirPath, ref string) ([]string, error) {
	var files []string
	if branchFiles, ok := m.existingFiles[ref]; ok {
		prefix := dirPath + "/"
		for filePath := range branchFiles {
			if strings.HasPrefix(filePath, prefix) && branchFiles[filePath] {
				fileName := filePath[len(prefix):]
				if !strings.Contains(fileName, "/") {
					files = append(files, fileName)
				}
			}
		}
	}
	return files, nil
}

func TestNewDataProductConsumerRule(t *testing.T) {
	tests := []struct {
		name         string
		allowedEnvs  []string
		expectedName string
		expectedEnvs []string
	}{
		{
			name:         "with custom environments",
			allowedEnvs:  []string{"staging", "production"},
			expectedName: "dataproduct_consumer_rule",
			expectedEnvs: []string{"staging", "production"},
		},
		{
			name:         "with nil environments (should use defaults)",
			allowedEnvs:  nil,
			expectedName: "dataproduct_consumer_rule",
			expectedEnvs: []string{"preprod", "prod"},
		},
		{
			name:         "with empty environments",
			allowedEnvs:  []string{},
			expectedName: "dataproduct_consumer_rule",
			expectedEnvs: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rule := NewDataProductConsumerRule(tt.allowedEnvs, nil)

			assert.Equal(t, tt.expectedName, rule.Name())
			assert.Contains(t, rule.Description(), "consumer access")
			assert.Equal(t, tt.expectedEnvs, rule.config.AllowedEnvironments)
		})
	}
}

func TestDataProductConsumerRule_ValidateLines(t *testing.T) {
	consumerYaml := `---
name: rosettastone
kind: aggregated
rover_group: dataverse-aggregate-rosettastone
data_product_db:
- database: rosettastone_db
  presentation_schemas:
  - name: marts
    consumers:
    - name: journey
      kind: data_product`

	tests := []struct {
		name                   string
		filePath               string
		fileContent            string
		lineRanges             []shared.LineRange
		mrContext              *shared.MRContext
		expectedDecision       shared.DecisionType
		expectedReasonContains string
	}{
		{
			name:                   "non-product file should approve",
			filePath:               "dataproducts/test/README.md",
			fileContent:            "# Test",
			lineRanges:             []shared.LineRange{{StartLine: 1, EndLine: 1, FilePath: "dataproducts/test/README.md"}},
			mrContext:              &shared.MRContext{},
			expectedDecision:       shared.Approve,
			expectedReasonContains: "Not a product.yaml file",
		},
		{
			name:        "consumer changes in prod environment should auto-approve",
			filePath:    "dataproducts/analytics/prod/product.yaml",
			fileContent: consumerYaml,
			lineRanges: []shared.LineRange{
				{StartLine: 9, EndLine: 11, FilePath: "dataproducts/analytics/prod/product.yaml"},
			},
			mrContext: &shared.MRContext{
				Changes: []gitlab.FileChange{
					{
						OldPath: "dataproducts/analytics/prod/product.yaml",
						NewPath: "dataproducts/analytics/prod/product.yaml",
						NewFile: false,
					},
				},
			},
			expectedDecision:       shared.Approve,
			expectedReasonContains: "data product owner approval sufficient",
		},
		{
			name:        "consumer changes in preprod environment should auto-approve",
			filePath:    "dataproducts/analytics/preprod/product.yaml",
			fileContent: consumerYaml,
			lineRanges: []shared.LineRange{
				{StartLine: 9, EndLine: 11, FilePath: "dataproducts/analytics/preprod/product.yaml"},
			},
			mrContext: &shared.MRContext{
				Changes: []gitlab.FileChange{
					{
						OldPath: "dataproducts/analytics/preprod/product.yaml",
						NewPath: "dataproducts/analytics/preprod/product.yaml",
						NewFile: false,
					},
				},
			},
			expectedDecision:       shared.Approve,
			expectedReasonContains: "data product owner approval sufficient",
		},
		{
			name:        "consumer changes in dev environment should auto-approve",
			filePath:    "dataproducts/analytics/dev/product.yaml",
			fileContent: consumerYaml,
			lineRanges: []shared.LineRange{
				{StartLine: 9, EndLine: 11, FilePath: "dataproducts/analytics/dev/product.yaml"},
			},
			mrContext: &shared.MRContext{
				Changes: []gitlab.FileChange{
					{
						NewPath: "dataproducts/analytics/dev/product.yaml",
						NewFile: true,
					},
				},
			},
			expectedDecision:       shared.Approve,
			expectedReasonContains: "data product owner approval sufficient",
		},
		{
			name:        "non-consumer changes should approve with generic message",
			filePath:    "dataproducts/analytics/prod/product.yaml",
			fileContent: consumerYaml,
			lineRanges: []shared.LineRange{
				{StartLine: 1, EndLine: 4, FilePath: "dataproducts/analytics/prod/product.yaml"},
			},
			mrContext: &shared.MRContext{
				Changes: []gitlab.FileChange{
					{
						NewPath: "dataproducts/analytics/prod/product.yaml",
						NewFile: false,
					},
				},
			},
			expectedDecision:       shared.Approve,
			expectedReasonContains: "No consumer-only changes detected",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rule := NewDataProductConsumerRule([]string{"preprod", "prod"}, nil)
			rule.SetMRContext(tt.mrContext)

			decision, reason := rule.ValidateLines(tt.filePath, tt.fileContent, tt.lineRanges)

			assert.Equal(t, tt.expectedDecision, decision)
			assert.Contains(t, reason, tt.expectedReasonContains)
		})
	}
}

func TestDataProductConsumerRule_GetCoveredLines(t *testing.T) {
	consumerYaml := `---
name: rosettastone
kind: aggregated
rover_group: dataverse-aggregate-rosettastone
data_product_db:
- database: rosettastone_db
  presentation_schemas:
  - name: marts
    consumers:
    - name: journey
      kind: data_product`

	tests := []struct {
		name                string
		filePath            string
		fileContent         string
		expectedCoverageLen int
		expectedStartLine   int
		expectedEndLine     int
	}{
		{
			name:                "non-product file should have no coverage",
			filePath:            "dataproducts/test/README.md",
			fileContent:         "# Test",
			expectedCoverageLen: 0,
		},
		{
			name:                "product.yaml with consumers should return placeholder",
			filePath:            "dataproducts/analytics/prod/product.yaml",
			fileContent:         consumerYaml,
			expectedCoverageLen: 1,
			expectedStartLine:   1,
			expectedEndLine:     1,
		},
		{
			name:     "product.yaml without consumers should return placeholder",
			filePath: "dataproducts/analytics/prod/product.yaml",
			fileContent: `---
name: test
kind: aggregated`,
			expectedCoverageLen: 1,
			expectedStartLine:   1,
			expectedEndLine:     1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rule := NewDataProductConsumerRule([]string{"preprod", "prod"}, nil)

			coveredLines := rule.GetCoveredLines(tt.filePath, tt.fileContent)

			assert.Len(t, coveredLines, tt.expectedCoverageLen)

			if tt.expectedCoverageLen > 0 {
				assert.Equal(t, tt.expectedStartLine, coveredLines[0].StartLine)
				assert.Equal(t, tt.expectedEndLine, coveredLines[0].EndLine)
			}
		})
	}
}

func TestDataProductConsumerRule_extractEnvironmentFromPath(t *testing.T) {
	tests := []struct {
		name     string
		filePath string
		expected string
	}{
		{
			name:     "prod in directory path",
			filePath: "dataproducts/analytics/prod/product.yaml",
			expected: "prod",
		},
		{
			name:     "preprod in directory path",
			filePath: "dataproducts/source/preprod/product.yaml",
			expected: "preprod",
		},
		{
			name:     "dev in directory path",
			filePath: "dataproducts/analytics/dev/product.yaml",
			expected: "dev",
		},
		{
			name:     "sandbox in directory path",
			filePath: "dataproducts/analytics/sandbox/product.yaml",
			expected: "sandbox",
		},
		{
			name:     "environment with underscore",
			filePath: "dataproducts/analytics/my_prod_setup/product.yaml",
			expected: "prod",
		},
		{
			name:     "case insensitive matching",
			filePath: "dataproducts/analytics/PROD/product.yaml",
			expected: "prod",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rule := NewDataProductConsumerRule([]string{"preprod", "prod"}, nil)

			result := rule.extractEnvironmentFromPath(tt.filePath)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestDataProductConsumerRule_isConsumerRelatedLine(t *testing.T) {
	tests := []struct {
		name     string
		line     string
		expected bool
	}{
		{
			name:     "consumers keyword",
			line:     "    consumers:",
			expected: true,
		},
		{
			name:     "name field in consumer",
			line:     "    - name: journey",
			expected: true,
		},
		{
			name:     "kind field in consumer",
			line:     "      kind: data_product",
			expected: true,
		},
		{
			name:     "non-consumer line",
			line:     "    warehouse:",
			expected: false,
		},
		{
			name:     "database line",
			line:     "- database: test_db",
			expected: false,
		},
		{
			name:     "empty line",
			line:     "",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rule := NewDataProductConsumerRule([]string{"preprod", "prod"}, nil)

			result := rule.isConsumerRelatedLine(tt.line)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestDataProductConsumerRule_containsConsumersKey(t *testing.T) {
	tests := []struct {
		name        string
		fileContent string
		expected    bool
	}{
		{
			name: "file with consumers section",
			fileContent: `---
name: rosettastone
data_product_db:
- database: rosettastone_db
  presentation_schemas:
  - name: marts
    consumers:
    - name: journey
      kind: data_product`,
			expected: true,
		},
		{
			name: "file without consumers section",
			fileContent: `---
name: test
kind: aggregated
data_product_db:
- database: test_db
  presentation_schemas:
  - name: marts`,
			expected: false,
		},
		{
			name: "empty consumers section",
			fileContent: `---
name: test
data_product_db:
- database: test_db
  presentation_schemas:
  - name: marts
    consumers: []`,
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rule := NewDataProductConsumerRule([]string{"preprod", "prod"}, nil)

			yamlContent, err := readYaml(tt.fileContent)
			assert.NoError(t, err)
			result := rule.containsConsumersKey(yamlContent)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestDataProductConsumerRule_extractConsumerGroupNames(t *testing.T) {
	tests := []struct {
		name           string
		fileContent    string
		expectedGroups []string
	}{
		{
			name: "single consumer_group in list format",
			fileContent: `data_product_db:
- database: test_db
  presentation_schemas:
  - name: marts
    consumers:
    - name: dataverse-consumer-test-marts
      kind: consumer_group`,
			expectedGroups: []string{"dataverse-consumer-test-marts"},
		},
		{
			name: "single consumer_group in map format",
			fileContent: `data_product_db:
  presentation_schemas:
  - name: marts
    consumers:
    - name: dataverse-consumer-test-marts
      kind: consumer_group`,
			expectedGroups: []string{"dataverse-consumer-test-marts"},
		},
		{
			name: "mixed consumer kinds",
			fileContent: `data_product_db:
- database: test_db
  presentation_schemas:
  - name: marts
    consumers:
    - name: some_service_account
      kind: service_account
    - name: dataverse-consumer-test-marts
      kind: consumer_group`,
			expectedGroups: []string{"dataverse-consumer-test-marts"},
		},
		{
			name: "no consumer_group entries",
			fileContent: `data_product_db:
- database: test_db
  presentation_schemas:
  - name: marts
    consumers:
    - name: journey
      kind: data_product`,
			expectedGroups: nil,
		},
		{
			name: "multiple consumer_groups across schemas",
			fileContent: `data_product_db:
- database: test_db
  presentation_schemas:
  - name: marts
    consumers:
    - name: dataverse-consumer-test-marts
      kind: consumer_group
  - name: staging
    consumers:
    - name: dataverse-consumer-test-staging
      kind: consumer_group`,
			expectedGroups: []string{"dataverse-consumer-test-marts", "dataverse-consumer-test-staging"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rule := NewDataProductConsumerRule([]string{"preprod", "prod"}, nil)
			yamlContent, err := readYaml(tt.fileContent)
			assert.NoError(t, err)
			result := rule.extractConsumerGroupNames(yamlContent)
			assert.Equal(t, tt.expectedGroups, result)
		})
	}
}

func TestDataProductConsumerRule_extractConsumerGroupNames_invalidYaml(t *testing.T) {
	_, err := readYaml("not: [valid: yaml")
	assert.Error(t, err)
}

func TestDataProductConsumerRule_listFilesOnBranch(t *testing.T) {
	mockClient := newMockGitLabClient(map[string]map[string]bool{
		"main": {
			"dataproducts/analytics/groups/existing-group.yaml": true,
			"dataproducts/analytics/groups/another-group.yaml":  true,
		},
	})

	t.Run("lists files from existing directory", func(t *testing.T) {
		rule := NewDataProductConsumerRule([]string{"preprod", "prod"}, mockClient)
		result := rule.listFilesOnBranch(1, "dataproducts/analytics/groups", "main")
		assert.True(t, result["existing-group.yaml"])
		assert.True(t, result["another-group.yaml"])
		assert.False(t, result["missing-group.yaml"])
	})

	t.Run("empty result for non-existent branch", func(t *testing.T) {
		rule := NewDataProductConsumerRule([]string{"preprod", "prod"}, mockClient)
		result := rule.listFilesOnBranch(1, "dataproducts/analytics/groups", "feature-branch")
		assert.Empty(t, result)
	})

	t.Run("empty branch returns empty set", func(t *testing.T) {
		rule := NewDataProductConsumerRule([]string{"preprod", "prod"}, mockClient)
		result := rule.listFilesOnBranch(1, "dataproducts/analytics/groups", "")
		assert.Empty(t, result)
	})

	t.Run("nil client returns empty set", func(t *testing.T) {
		rule := NewDataProductConsumerRule([]string{"preprod", "prod"}, nil)
		result := rule.listFilesOnBranch(1, "any/path", "main")
		assert.Empty(t, result)
	})
}

func TestDataProductConsumerRule_ValidateLines_ConsumerGroupValidation(t *testing.T) {
	consumerGroupYaml := `---
name: subscriptionwatch
kind: source-aligned
data_product_db:
- database: subscriptionwatch_db
  presentation_schemas:
  - name: marts
    consumers:
    - name: dataverse-consumer-subscriptionwatch-marts
      kind: consumer_group`

	mixedConsumerYaml := `---
name: analytics
kind: source-aligned
data_product_db:
- database: analytics_db
  presentation_schemas:
  - name: marts
    consumers:
    - name: journey
      kind: data_product`

	tests := []struct {
		name                   string
		filePath               string
		fileContent            string
		lineRanges             []shared.LineRange
		groupsFiles            map[string]map[string]bool
		mrContext              *shared.MRContext
		expectedDecision       shared.DecisionType
		expectedReasonContains string
	}{
		{
			name:        "consumer_group with existing file on source branch should approve",
			filePath:    "dataproducts/subscriptionwatch/prod/product.yaml",
			fileContent: consumerGroupYaml,
			lineRanges: []shared.LineRange{
				{StartLine: 8, EndLine: 10, FilePath: "dataproducts/subscriptionwatch/prod/product.yaml"},
			},
			groupsFiles: map[string]map[string]bool{
				"feature/add-consumer": {
					"dataproducts/subscriptionwatch/groups/dataverse-consumer-subscriptionwatch-marts.yaml": true,
				},
			},
			mrContext: &shared.MRContext{
				ProjectID: 1,
				MRInfo: &gitlab.MRInfo{
					ProjectID:    1,
					SourceBranch: "feature/add-consumer",
					TargetBranch: "main",
				},
			},
			expectedDecision:       shared.Approve,
			expectedReasonContains: "data product owner approval sufficient",
		},
		{
			name:        "consumer_group with existing file on target branch should approve",
			filePath:    "dataproducts/subscriptionwatch/prod/product.yaml",
			fileContent: consumerGroupYaml,
			lineRanges: []shared.LineRange{
				{StartLine: 8, EndLine: 10, FilePath: "dataproducts/subscriptionwatch/prod/product.yaml"},
			},
			groupsFiles: map[string]map[string]bool{
				"main": {
					"dataproducts/subscriptionwatch/groups/dataverse-consumer-subscriptionwatch-marts.yaml": true,
				},
			},
			mrContext: &shared.MRContext{
				ProjectID: 1,
				MRInfo: &gitlab.MRInfo{
					ProjectID:    1,
					SourceBranch: "feature/add-consumer",
					TargetBranch: "main",
				},
			},
			expectedDecision:       shared.Approve,
			expectedReasonContains: "data product owner approval sufficient",
		},
		{
			name:        "consumer_group with missing file should reject",
			filePath:    "dataproducts/subscriptionwatch/prod/product.yaml",
			fileContent: consumerGroupYaml,
			lineRanges: []shared.LineRange{
				{StartLine: 8, EndLine: 10, FilePath: "dataproducts/subscriptionwatch/prod/product.yaml"},
			},
			groupsFiles: map[string]map[string]bool{},
			mrContext: &shared.MRContext{
				ProjectID: 1,
				MRInfo: &gitlab.MRInfo{
					ProjectID:    1,
					SourceBranch: "feature/add-consumer",
					TargetBranch: "main",
				},
			},
			expectedDecision:       shared.ManualReview,
			expectedReasonContains: "Consumer group file(s) not found",
		},
		{
			name:        "data_product consumer (not consumer_group) skips group check and approves",
			filePath:    "dataproducts/analytics/prod/product.yaml",
			fileContent: mixedConsumerYaml,
			lineRanges: []shared.LineRange{
				{StartLine: 8, EndLine: 10, FilePath: "dataproducts/analytics/prod/product.yaml"},
			},
			groupsFiles: map[string]map[string]bool{},
			mrContext: &shared.MRContext{
				ProjectID: 1,
				MRInfo: &gitlab.MRInfo{
					ProjectID:    1,
					SourceBranch: "feature/add-consumer",
					TargetBranch: "main",
				},
			},
			expectedDecision:       shared.Approve,
			expectedReasonContains: "data product owner approval sufficient",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := newMockGitLabClient(tt.groupsFiles)
			rule := NewDataProductConsumerRule([]string{"preprod", "prod"}, mockClient)
			rule.SetMRContext(tt.mrContext)

			decision, reason := rule.ValidateLines(tt.filePath, tt.fileContent, tt.lineRanges)

			assert.Equal(t, tt.expectedDecision, decision)
			assert.Contains(t, reason, tt.expectedReasonContains)
		})
	}
}
