package tag

import (
	"testing"

	"github.com/redhat-data-and-ai/naysayer/internal/gitlab"
	"github.com/redhat-data-and-ai/naysayer/internal/rules/shared"
	"github.com/stretchr/testify/assert"
)

// MockGitLabClient for testing
type MockGitLabClient struct {
	DirectoryFiles map[string][]gitlab.RepositoryFile
	FileContents   map[string]string
}

func (m *MockGitLabClient) ListDirectoryFiles(projectID int, dirPath, ref string) ([]gitlab.RepositoryFile, error) {
	if files, ok := m.DirectoryFiles[dirPath]; ok {
		return files, nil
	}
	return []gitlab.RepositoryFile{}, nil
}

func (m *MockGitLabClient) FetchFileContent(projectID int, filePath, ref string) (*gitlab.FileContent, error) {
	if content, ok := m.FileContents[filePath]; ok {
		return &gitlab.FileContent{Content: content}, nil
	}
	return nil, nil
}

// Stub implementations for other interface methods
func (m *MockGitLabClient) GetMRTargetBranch(projectID, mrIID int) (string, error) {
	return "main", nil
}
func (m *MockGitLabClient) GetMRDetails(projectID, mrIID int) (*gitlab.MRDetails, error) {
	return nil, nil
}
func (m *MockGitLabClient) FetchMRChanges(projectID, mrIID int) ([]gitlab.FileChange, error) {
	return nil, nil
}
func (m *MockGitLabClient) AddMRComment(projectID, mrIID int, comment string) error { return nil }
func (m *MockGitLabClient) AddOrUpdateMRComment(projectID, mrIID int, commentBody, commentType string) error {
	return nil
}
func (m *MockGitLabClient) ListMRComments(projectID, mrIID int) ([]gitlab.MRComment, error) {
	return nil, nil
}
func (m *MockGitLabClient) UpdateMRComment(projectID, mrIID, commentID int, newBody string) error {
	return nil
}
func (m *MockGitLabClient) FindLatestNaysayerComment(projectID, mrIID int, commentType ...string) (*gitlab.MRComment, error) {
	return nil, nil
}
func (m *MockGitLabClient) ApproveMR(projectID, mrIID int) error { return nil }
func (m *MockGitLabClient) ApproveMRWithMessage(projectID, mrIID int, message string) error {
	return nil
}
func (m *MockGitLabClient) ResetNaysayerApproval(projectID, mrIID int) error { return nil }
func (m *MockGitLabClient) GetCurrentBotUsername() (string, error)           { return "test-bot", nil }
func (m *MockGitLabClient) IsNaysayerBotAuthor(author map[string]interface{}) bool {
	return false
}
func (m *MockGitLabClient) RebaseMR(projectID, mrIID int) (bool, error) { return true, nil }
func (m *MockGitLabClient) CompareBranches(sourceProjectID int, sourceBranch string, targetProjectID int, targetBranch string) (*gitlab.CompareResult, error) {
	return nil, nil
}
func (m *MockGitLabClient) GetBranchCommit(projectID int, branch string) (string, error) {
	return "", nil
}
func (m *MockGitLabClient) CompareCommits(projectID int, fromSHA, toSHA string) (*gitlab.CompareResult, error) {
	return nil, nil
}
func (m *MockGitLabClient) ListOpenMRs(projectID int) ([]int, error) { return nil, nil }
func (m *MockGitLabClient) ListOpenMRsWithDetails(projectID int) ([]gitlab.MRDetails, error) {
	return nil, nil
}
func (m *MockGitLabClient) GetPipelineJobs(projectID, pipelineID int) ([]gitlab.PipelineJob, error) {
	return nil, nil
}
func (m *MockGitLabClient) GetJobTrace(projectID, jobID int) (string, error) { return "", nil }
func (m *MockGitLabClient) FindLatestAtlantisComment(projectID, mrIID int) (*gitlab.MRComment, error) {
	return nil, nil
}
func (m *MockGitLabClient) AreAllPipelineJobsSucceeded(projectID, pipelineID int) (bool, error) {
	return true, nil
}
func (m *MockGitLabClient) CheckAtlantisCommentForPlanFailures(projectID, mrIID int) (bool, string) {
	return false, ""
}
func (m *MockGitLabClient) ListAllOpenMRsWithDetails(projectID int) ([]gitlab.MRDetails, error) {
	return nil, nil
}
func (m *MockGitLabClient) CloseMR(projectID, mrIID int) error { return nil }
func (m *MockGitLabClient) FindCommentByPattern(projectID, mrIID int, pattern string) (bool, error) {
	return false, nil
}

func TestRule_Name(t *testing.T) {
	r := NewRule(nil)
	assert.Equal(t, "tag_rule", r.Name())
}

func TestRule_Description(t *testing.T) {
	r := NewRule(nil)
	assert.Contains(t, r.Description(), "tag")
}

func TestRule_isTagFile(t *testing.T) {
	r := NewRule(nil)

	tests := []struct {
		name     string
		path     string
		content  string
		expected bool
	}{
		{
			name:     "valid tag file with kind",
			path:     "dataproducts/source/analytics/sandbox/tag_pii.yaml",
			content:  "kind: Tag\nname: analytics_pii",
			expected: true,
		},
		{
			name:     "masking policy file",
			path:     "dataproducts/source/analytics/sandbox/masking_policy.yaml",
			content:  "kind: MaskingPolicy\nname: analytics_pii_string_policy",
			expected: false,
		},
		{
			name:     "tag file by name pattern (no content)",
			path:     "dataproducts/source/analytics/sandbox/tag_pii.yaml",
			content:  "",
			expected: true,
		},
		{
			name:     "non-yaml file",
			path:     "dataproducts/source/analytics/sandbox/readme.md",
			content:  "",
			expected: false,
		},
		{
			name:     "file outside dataproducts",
			path:     "configs/tag_pii.yaml",
			content:  "kind: Tag",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := r.isTagFile(tt.path, tt.content)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestRule_extractPathInfo(t *testing.T) {
	r := NewRule(nil)

	tests := []struct {
		name         string
		path         string
		expectedType string
		expectedDP   string
		expectedEnv  string
	}{
		{
			name:         "source type",
			path:         "dataproducts/source/analytics/sandbox/tag_pii.yaml",
			expectedType: "source",
			expectedDP:   "analytics",
			expectedEnv:  "sandbox",
		},
		{
			name:         "aggregate type",
			path:         "dataproducts/aggregate/hellosource/production/tag_restricted.yaml",
			expectedType: "aggregate",
			expectedDP:   "hellosource",
			expectedEnv:  "production",
		},
		{
			name:         "platform type",
			path:         "dataproducts/platform/cloudoscope/stage/tag_pii.yaml",
			expectedType: "platform",
			expectedDP:   "cloudoscope",
			expectedEnv:  "stage",
		},
		{
			name:         "invalid path",
			path:         "configs/tag_pii.yaml",
			expectedType: "",
			expectedDP:   "",
			expectedEnv:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dpType, dp, env := r.extractPathInfo(tt.path)
			assert.Equal(t, tt.expectedType, dpType)
			assert.Equal(t, tt.expectedDP, dp)
			assert.Equal(t, tt.expectedEnv, env)
		})
	}
}

func TestRule_ValidateLines_ValidTag(t *testing.T) {
	r := NewRule(nil)

	content := `kind: Tag
name: analytics_pii
description: String and Float tag for pii data masking
data_product: analytics
masking_policies:
  - name: analytics_pii_string_policy
  - name: analytics_pii_float_policy
allowed_values:
  - default`

	path := "dataproducts/source/analytics/sandbox/tag_pii.yaml"
	lineRanges := r.GetCoveredLines(path, content)

	decision, reason := r.ValidateLines(path, content, lineRanges)

	assert.Equal(t, shared.Approve, decision)
	assert.Contains(t, reason, "passed")
}

func TestRule_ValidateLines_InvalidTagName(t *testing.T) {
	r := NewRule(nil)

	content := `kind: Tag
name: invalid-name
description: Invalid tag
data_product: analytics
masking_policies:
  - name: analytics_pii_string_policy
allowed_values:
  - default`

	path := "dataproducts/source/analytics/sandbox/tag_pii.yaml"
	lineRanges := r.GetCoveredLines(path, content)

	decision, reason := r.ValidateLines(path, content, lineRanges)

	assert.Equal(t, shared.ManualReview, decision)
	assert.Contains(t, reason, "name")
}

func TestRule_ValidateLines_DataProductMismatch(t *testing.T) {
	r := NewRule(nil)

	content := `kind: Tag
name: analytics_pii
description: Valid tag
data_product: different
masking_policies:
  - name: different_pii_string_policy
allowed_values:
  - default`

	path := "dataproducts/source/analytics/sandbox/tag_pii.yaml"
	lineRanges := r.GetCoveredLines(path, content)

	decision, reason := r.ValidateLines(path, content, lineRanges)

	assert.Equal(t, shared.ManualReview, decision)
	assert.Contains(t, reason, "mismatch")
}

func TestRule_ValidateLines_DeletedFile(t *testing.T) {
	r := NewRule(nil)

	path := "dataproducts/source/analytics/sandbox/tag_pii.yaml"
	content := ""
	lineRanges := r.GetCoveredLines(path, content)

	decision, reason := r.ValidateLines(path, content, lineRanges)

	assert.Equal(t, shared.ManualReview, decision)
	assert.Contains(t, reason, "deletion")
}

func TestRule_ValidateLines_MaskingPolicyExists(t *testing.T) {
	mockClient := &MockGitLabClient{
		DirectoryFiles: map[string][]gitlab.RepositoryFile{
			"dataproducts/source/analytics/sandbox": {
				{Name: "masking_pii_string.yaml", Path: "dataproducts/source/analytics/sandbox/masking_pii_string.yaml", Type: "blob"},
			},
		},
		FileContents: map[string]string{
			"dataproducts/source/analytics/sandbox/masking_pii_string.yaml": `kind: MaskingPolicy
name: analytics_pii_string_policy`,
		},
	}

	r := NewRule(mockClient)
	r.SetMRContext(&shared.MRContext{
		ProjectID: 123,
		Changes:   []gitlab.FileChange{},
		MRInfo:    &gitlab.MRInfo{TargetBranch: "main"},
	})

	content := `kind: Tag
name: analytics_pii
description: Valid tag
data_product: analytics
masking_policies:
  - name: analytics_pii_string_policy
allowed_values:
  - default`

	path := "dataproducts/source/analytics/sandbox/tag_pii.yaml"
	lineRanges := r.GetCoveredLines(path, content)

	decision, reason := r.ValidateLines(path, content, lineRanges)

	assert.Equal(t, shared.Approve, decision)
	assert.Contains(t, reason, "passed")
}

func TestRule_ValidateLines_MaskingPolicyMissing(t *testing.T) {
	mockClient := &MockGitLabClient{
		DirectoryFiles: map[string][]gitlab.RepositoryFile{},
		FileContents:   map[string]string{},
	}

	r := NewRule(mockClient)
	r.SetMRContext(&shared.MRContext{
		ProjectID: 123,
		Changes:   []gitlab.FileChange{},
		MRInfo:    &gitlab.MRInfo{TargetBranch: "main"},
	})

	content := `kind: Tag
name: analytics_pii
description: Valid tag
data_product: analytics
masking_policies:
  - name: analytics_pii_string_policy
allowed_values:
  - default`

	path := "dataproducts/source/analytics/sandbox/tag_pii.yaml"
	lineRanges := r.GetCoveredLines(path, content)

	decision, reason := r.ValidateLines(path, content, lineRanges)

	assert.Equal(t, shared.ManualReview, decision)
	assert.Contains(t, reason, "Missing masking policies")
}

func TestRule_ValidateLines_MaskingPolicyInSameMR(t *testing.T) {
	mockClient := &MockGitLabClient{
		DirectoryFiles: map[string][]gitlab.RepositoryFile{},
		FileContents:   map[string]string{},
	}

	r := NewRule(mockClient)
	r.SetMRContext(&shared.MRContext{
		ProjectID: 123,
		Changes: []gitlab.FileChange{
			{
				NewPath: "dataproducts/source/analytics/sandbox/masking_pii_string.yaml",
				Diff:    "+kind: MaskingPolicy\n+name: analytics_pii_string_policy",
			},
		},
		MRInfo: &gitlab.MRInfo{TargetBranch: "main"},
	})

	content := `kind: Tag
name: analytics_pii
description: Valid tag
data_product: analytics
masking_policies:
  - name: analytics_pii_string_policy
allowed_values:
  - default`

	path := "dataproducts/source/analytics/sandbox/tag_pii.yaml"
	lineRanges := r.GetCoveredLines(path, content)

	decision, reason := r.ValidateLines(path, content, lineRanges)

	assert.Equal(t, shared.Approve, decision)
	assert.Contains(t, reason, "passed")
}

func TestRule_CheckDuplicateTagNames(t *testing.T) {
	r := NewRule(nil)
	r.SetMRContext(&shared.MRContext{
		ProjectID: 123,
		Changes: []gitlab.FileChange{
			{
				NewPath: "dataproducts/source/analytics/sandbox/tag_pii.yaml",
				Diff:    "+kind: Tag\n+name: analytics_pii",
			},
			{
				NewPath: "dataproducts/source/analytics/production/tag_pii.yaml",
				Diff:    "+kind: Tag\n+name: analytics_pii",
			},
		},
	})

	duplicates := r.CheckDuplicateTagNames()
	assert.Contains(t, duplicates, "analytics_pii")
}

func TestRule_GetCoveredLines(t *testing.T) {
	r := NewRule(nil)

	content := `kind: Tag
name: analytics_pii
description: desc
data_product: analytics
masking_policies:
  - name: analytics_pii_string_policy
allowed_values:
  - default`

	path := "dataproducts/source/analytics/sandbox/tag_pii.yaml"
	lineRanges := r.GetCoveredLines(path, content)

	assert.Len(t, lineRanges, 1)
	assert.Equal(t, 1, lineRanges[0].StartLine)
	assert.Equal(t, 8, lineRanges[0].EndLine)
}

func TestRule_SkipsNonTagKind(t *testing.T) {
	r := NewRule(nil)

	content := `kind: MaskingPolicy
name: analytics_pii_string_policy
description: A masking policy
data_product: analytics`

	path := "dataproducts/source/analytics/sandbox/masking.yaml"
	lineRanges := r.GetCoveredLines(path, content)

	// Should not match as it's not a Tag kind
	assert.Nil(t, lineRanges)
}
