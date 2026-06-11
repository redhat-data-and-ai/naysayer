//nolint:staticcheck // ST1003 accepted here
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
func (m *mockGitLabClient) ListDirectoryFiles(projectID int, dirPath, ref string) ([]gitlab.RepositoryFile, error) {
	var files []gitlab.RepositoryFile
	if branchFiles, ok := m.existingFiles[ref]; ok {
		prefix := dirPath + "/"
		for filePath := range branchFiles {
			if strings.HasPrefix(filePath, prefix) && branchFiles[filePath] {
				fileName := filePath[len(prefix):]
				if !strings.Contains(fileName, "/") {
					files = append(files, gitlab.RepositoryFile{Name: fileName})
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

func TestDataProductConsumerRule_fileContainsConsumersSection(t *testing.T) {
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

			// Pre-parse the YAML content to match the new function signature
			parsedContent := rule.parseYAMLContent(tt.fileContent)
			result := rule.fileContainsConsumersSection(parsedContent)
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

func TestDataProductConsumerRule_detectSelfConsumer(t *testing.T) {
	tests := []struct {
		name                 string
		filePath             string
		fileContent          string
		expectedSelfConsumer bool
		expectedName         string
	}{
		{
			name:     "self-consumer with data_product kind should be detected",
			filePath: "dataproducts/aggregate/analytics/prod/product.yaml",
			fileContent: `---
name: analytics
kind: aggregated
rover_group: dataverse-aggregate-analytics
data_product_db:
- database: analytics_db
  presentation_schemas:
  - name: marts
    consumers:
    - name: analytics
      kind: data_product`,
			expectedSelfConsumer: true,
			expectedName:         "analytics",
		},
		{
			name:     "different consumer should not be flagged",
			filePath: "dataproducts/aggregate/analytics/prod/product.yaml",
			fileContent: `---
name: analytics
kind: aggregated
rover_group: dataverse-aggregate-analytics
data_product_db:
- database: analytics_db
  presentation_schemas:
  - name: marts
    consumers:
    - name: journey
      kind: data_product`,
			expectedSelfConsumer: false,
			expectedName:         "",
		},
		{
			name:     "self-consumer with consumer_group kind should NOT be flagged",
			filePath: "dataproducts/aggregate/analytics/prod/product.yaml",
			fileContent: `---
name: analytics
kind: aggregated
rover_group: dataverse-aggregate-analytics
data_product_db:
- database: analytics_db
  presentation_schemas:
  - name: marts
    consumers:
    - name: analytics
      kind: consumer_group`,
			expectedSelfConsumer: false,
			expectedName:         "",
		},
		{
			name:     "self-consumer with service_account kind should NOT be flagged",
			filePath: "dataproducts/aggregate/analytics/prod/product.yaml",
			fileContent: `---
name: analytics
kind: aggregated
data_product_db:
- database: analytics_db
  presentation_schemas:
  - name: marts
    consumers:
    - name: analytics
      kind: service_account`,
			expectedSelfConsumer: false,
			expectedName:         "",
		},
		{
			name:     "multiple consumers with one self-consumer should be detected",
			filePath: "dataproducts/source/sfsales/prod/product.yaml",
			fileContent: `---
name: sfsales
kind: source-aligned
rover_group: dataverse-source-sfsales
data_product_db:
- database: sfsales_db
  presentation_schemas:
  - name: marts
    consumers:
    - name: journey
      kind: data_product
    - name: sfsales
      kind: data_product
    - name: forecasting
      kind: data_product`,
			expectedSelfConsumer: true,
			expectedName:         "sfsales",
		},
		{
			name:     "self-consumer in second schema should be detected",
			filePath: "dataproducts/aggregate/analytics/prod/product.yaml",
			fileContent: `---
name: analytics
kind: aggregated
data_product_db:
- database: analytics_db
  presentation_schemas:
  - name: marts
    consumers:
    - name: journey
      kind: data_product
  - name: staging
    consumers:
    - name: analytics
      kind: data_product`,
			expectedSelfConsumer: true,
			expectedName:         "analytics",
		},
		{
			name:     "empty consumers list should not be flagged",
			filePath: "dataproducts/aggregate/test/prod/product.yaml",
			fileContent: `---
name: test
kind: aggregated
data_product_db:
- database: test_db
  presentation_schemas:
  - name: marts
    consumers: []`,
			expectedSelfConsumer: false,
			expectedName:         "",
		},
		{
			name:     "no consumers section should not be flagged",
			filePath: "dataproducts/aggregate/test/prod/product.yaml",
			fileContent: `---
name: test
kind: aggregated
data_product_db:
- database: test_db
  presentation_schemas:
  - name: marts`,
			expectedSelfConsumer: false,
			expectedName:         "",
		},
		{
			name:     "file without name field should use path extraction",
			filePath: "dataproducts/aggregate/test/prod/product.yaml",
			fileContent: `---
kind: aggregated
data_product_db:
- database: test_db
  presentation_schemas:
  - name: marts
    consumers:
    - name: test
      kind: data_product`,
			expectedSelfConsumer: true,
			expectedName:         "test",
		},
		{
			name:     "section content without name field should use path extraction",
			filePath: "dataproducts/aggregate/analytics/prod/product.yaml",
			fileContent: `- database: analytics_db
  presentation_schemas:
  - name: marts
    consumers:
    - name: analytics
      kind: data_product`,
			expectedSelfConsumer: true,
			expectedName:         "analytics",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rule := NewDataProductConsumerRule([]string{"preprod", "prod"}, nil)

			// Pre-parse the YAML content to match the new function signature
			parsedContent := rule.parseYAMLContent(tt.fileContent)
			isSelfConsumer, name := rule.detectSelfConsumer(tt.filePath, parsedContent)

			assert.Equal(t, tt.expectedSelfConsumer, isSelfConsumer)
			assert.Equal(t, tt.expectedName, name)
		})
	}
}

func TestDataProductConsumerRule_extractProductNameFromPath(t *testing.T) {
	tests := []struct {
		name         string
		filePath     string
		expectedName string
	}{
		{
			name:         "aggregate product in prod",
			filePath:     "dataproducts/aggregate/analytics/prod/product.yaml",
			expectedName: "analytics",
		},
		{
			name:         "source product in dev",
			filePath:     "dataproducts/source/sfsales/dev/product.yaml",
			expectedName: "sfsales",
		},
		{
			name:         "platform product in sandbox",
			filePath:     "dataproducts/platform/myproduct/sandbox/product.yaml",
			expectedName: "myproduct",
		},
		{
			name:         "with absolute path",
			filePath:     "/some/root/dataproducts/aggregate/analytics/prod/product.yaml",
			expectedName: "analytics",
		},
		{
			name:         "invalid path without dataproducts",
			filePath:     "some/other/path/product.yaml",
			expectedName: "",
		},
		{
			name:         "path too short",
			filePath:     "dataproducts/source/product.yaml",
			expectedName: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rule := NewDataProductConsumerRule([]string{"preprod", "prod"}, nil)

			result := rule.extractProductNameFromPath(tt.filePath)
			assert.Equal(t, tt.expectedName, result)
		})
	}
}

func TestDataProductConsumerRule_ValidateLines_SelfConsumer(t *testing.T) {
	selfConsumerYaml := `---
name: analytics
kind: aggregated
rover_group: dataverse-aggregate-analytics
data_product_db:
- database: analytics_db
  presentation_schemas:
  - name: marts
    consumers:
    - name: dataverse-consumer-analytics-marts
      kind: consumer_group
    - name: analytics
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
			name:        "self-consumer in prod should require manual review",
			filePath:    "dataproducts/aggregate/analytics/prod/product.yaml",
			fileContent: selfConsumerYaml,
			lineRanges: []shared.LineRange{
				{StartLine: 12, EndLine: 14, FilePath: "dataproducts/aggregate/analytics/prod/product.yaml"},
			},
			mrContext: &shared.MRContext{
				Changes: []gitlab.FileChange{
					{
						OldPath: "dataproducts/aggregate/analytics/prod/product.yaml",
						NewPath: "dataproducts/aggregate/analytics/prod/product.yaml",
						NewFile: false,
					},
				},
			},
			expectedDecision:       shared.ManualReview,
			expectedReasonContains: "Self-consumer detected",
		},
		{
			name:        "self-consumer in preprod should require manual review",
			filePath:    "dataproducts/aggregate/analytics/preprod/product.yaml",
			fileContent: selfConsumerYaml,
			lineRanges: []shared.LineRange{
				{StartLine: 12, EndLine: 14, FilePath: "dataproducts/aggregate/analytics/preprod/product.yaml"},
			},
			mrContext: &shared.MRContext{
				Changes: []gitlab.FileChange{
					{
						NewPath: "dataproducts/aggregate/analytics/preprod/product.yaml",
						NewFile: false,
					},
				},
			},
			expectedDecision:       shared.ManualReview,
			expectedReasonContains: "cannot be added as a consumer of itself",
		},
		{
			name:        "self-consumer in dev should also require manual review",
			filePath:    "dataproducts/aggregate/analytics/dev/product.yaml",
			fileContent: selfConsumerYaml,
			lineRanges: []shared.LineRange{
				{StartLine: 12, EndLine: 14, FilePath: "dataproducts/aggregate/analytics/dev/product.yaml"},
			},
			mrContext: &shared.MRContext{
				Changes: []gitlab.FileChange{
					{
						NewPath: "dataproducts/aggregate/analytics/dev/product.yaml",
						NewFile: true,
					},
				},
			},
			expectedDecision:       shared.ManualReview,
			expectedReasonContains: "analytics",
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

func TestDataProductConsumerRule_parseYAMLContent_EdgeCases(t *testing.T) {
	rule := NewDataProductConsumerRule([]string{"preprod", "prod"}, nil)

	tests := []struct {
		name      string
		content   string
		expectNil bool
	}{
		{
			name:      "invalid YAML should return nil",
			content:   "{{invalid yaml content",
			expectNil: true,
		},
		{
			name:      "empty string should return nil",
			content:   "",
			expectNil: true,
		},
		{
			name:      "valid YAML should not return nil",
			content:   "name: test",
			expectNil: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := rule.parseYAMLContent(tt.content)
			if tt.expectNil {
				assert.Nil(t, result)
			} else {
				assert.NotNil(t, result)
			}
		})
	}
}

func TestDataProductConsumerRule_fileContainsConsumersSection_EdgeCases(t *testing.T) {
	rule := NewDataProductConsumerRule([]string{"preprod", "prod"}, nil)

	tests := []struct {
		name     string
		content  string
		expected bool
	}{
		{
			name:     "nil content should return false",
			content:  "{{invalid",
			expected: false,
		},
		{
			name:     "array content (non-map) should return false",
			content:  "- item1\n- item2",
			expected: false,
		},
		{
			name:     "map without data_product_db should return false",
			content:  "name: test\nkind: aggregated",
			expected: false,
		},
		{
			name: "data_product_db with non-map entries should handle gracefully",
			content: `data_product_db:
- "string_entry"
- 123`,
			expected: false,
		},
		{
			name: "presentation_schemas with non-map entries should handle gracefully",
			content: `data_product_db:
- database: test_db
  presentation_schemas:
  - "string_schema"`,
			expected: false,
		},
		{
			name: "schema without consumers key should return false",
			content: `data_product_db:
- database: test_db
  presentation_schemas:
  - name: marts
    other_field: value`,
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parsedContent := rule.parseYAMLContent(tt.content)
			result := rule.fileContainsConsumersSection(parsedContent)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestDataProductConsumerRule_detectSelfConsumer_EdgeCases(t *testing.T) {
	rule := NewDataProductConsumerRule([]string{"preprod", "prod"}, nil)

	tests := []struct {
		name                 string
		filePath             string
		content              string
		expectedSelfConsumer bool
		expectedName         string
	}{
		{
			name:     "consumers with non-map entries should be skipped",
			filePath: "dataproducts/aggregate/analytics/prod/product.yaml",
			content: `name: analytics
data_product_db:
- database: analytics_db
  presentation_schemas:
  - name: marts
    consumers:
    - "string_consumer"
    - 123`,
			expectedSelfConsumer: false,
			expectedName:         "",
		},
		{
			name:     "consumer missing name field should not match",
			filePath: "dataproducts/aggregate/analytics/prod/product.yaml",
			content: `name: analytics
data_product_db:
- database: analytics_db
  presentation_schemas:
  - name: marts
    consumers:
    - kind: data_product`,
			expectedSelfConsumer: false,
			expectedName:         "",
		},
		{
			name:     "consumer missing kind field should not match",
			filePath: "dataproducts/aggregate/analytics/prod/product.yaml",
			content: `name: analytics
data_product_db:
- database: analytics_db
  presentation_schemas:
  - name: marts
    consumers:
    - name: analytics`,
			expectedSelfConsumer: false,
			expectedName:         "",
		},
		{
			name:                 "nil parsed content should return false",
			filePath:             "dataproducts/aggregate/analytics/prod/product.yaml",
			content:              "{{invalid yaml",
			expectedSelfConsumer: false,
			expectedName:         "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parsedContent := rule.parseYAMLContent(tt.content)
			isSelfConsumer, name := rule.detectSelfConsumer(tt.filePath, parsedContent)
			assert.Equal(t, tt.expectedSelfConsumer, isSelfConsumer)
			assert.Equal(t, tt.expectedName, name)
		})
	}
}

func TestDataProductConsumerRule_extractConsumersFromContent_EdgeCases(t *testing.T) {
	rule := NewDataProductConsumerRule([]string{"preprod", "prod"}, nil)

	t.Run("nil content should return nil", func(t *testing.T) {
		result := rule.extractConsumersFromContent(nil)
		assert.Nil(t, result)
	})

	t.Run("non-map non-array content should return nil", func(t *testing.T) {
		result := rule.extractConsumersFromContent("string content")
		assert.Nil(t, result)
	})

	t.Run("integer content should return nil", func(t *testing.T) {
		result := rule.extractConsumersFromContent(123)
		assert.Nil(t, result)
	})

	t.Run("array content should be processed as DBArray", func(t *testing.T) {
		content := `- database: test_db
  presentation_schemas:
  - name: marts
    consumers:
    - name: consumer1
      kind: data_product`
		parsedContent := rule.parseYAMLContent(content)
		result := rule.extractConsumersFromContent(parsedContent)
		assert.NotNil(t, result)
		assert.Len(t, result, 1)
	})
}

func TestDataProductConsumerRule_extractConsumersFromMap_EdgeCases(t *testing.T) {
	rule := NewDataProductConsumerRule([]string{"preprod", "prod"}, nil)

	t.Run("data_product_db as array should be processed", func(t *testing.T) {
		content := `data_product_db:
- database: test_db
  presentation_schemas:
  - name: marts
    consumers:
    - name: consumer1
      kind: data_product`
		parsedContent := rule.parseYAMLContent(content)
		result := rule.extractConsumersFromContent(parsedContent)
		assert.NotNil(t, result)
		assert.Len(t, result, 1)
	})

	t.Run("data_product_db with unexpected type should fallback to direct extraction", func(t *testing.T) {
		content := `data_product_db: "string_value"
presentation_schemas:
- name: marts
  consumers:
  - name: consumer1
    kind: data_product`
		parsedContent := rule.parseYAMLContent(content)
		result := rule.extractConsumersFromContent(parsedContent)
		assert.NotNil(t, result)
		assert.Len(t, result, 1)
	})

	t.Run("map without data_product_db should check presentation_schemas directly", func(t *testing.T) {
		content := `presentation_schemas:
- name: marts
  consumers:
  - name: consumer1
    kind: data_product`
		parsedContent := rule.parseYAMLContent(content)
		result := rule.extractConsumersFromContent(parsedContent)
		assert.NotNil(t, result)
		assert.Len(t, result, 1)
	})
}
