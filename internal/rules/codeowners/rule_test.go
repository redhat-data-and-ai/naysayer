package codeowners

import (
	"fmt"
	"testing"

	"github.com/redhat-data-and-ai/naysayer/internal/gitlab"
	"github.com/redhat-data-and-ai/naysayer/internal/rules/shared"
	"github.com/stretchr/testify/assert"
)

// MockGitLabClient implements gitlab.GitLabClient for testing
type MockGitLabClient struct {
	fileContents map[string]*gitlab.FileContent
}

func NewMockGitLabClient() *MockGitLabClient {
	return &MockGitLabClient{fileContents: make(map[string]*gitlab.FileContent)}
}

func (m *MockGitLabClient) FetchFileContent(projectID int, filePath, ref string) (*gitlab.FileContent, error) {
	key := ref + ":" + filePath
	if content, exists := m.fileContents[key]; exists {
		return content, nil
	}
	return nil, fmt.Errorf("file not found: %s", filePath)
}

func (m *MockGitLabClient) SetFileContent(ref, filePath, content string) {
	m.fileContents[ref+":"+filePath] = &gitlab.FileContent{
		FileName: filePath, FilePath: filePath, Content: content, Encoding: "text", Ref: ref,
	}
}

// Stub interface methods
func (m *MockGitLabClient) FetchMRChanges(projectID, mrIID int) ([]gitlab.FileChange, error) {
	return nil, nil
}
func (m *MockGitLabClient) AddMRComment(projectID, mrIID int, comment string) error { return nil }
func (m *MockGitLabClient) ApproveMR(projectID, mrIID int) error                    { return nil }
func (m *MockGitLabClient) ApproveMRWithMessage(projectID, mrIID int, message string) error {
	return nil
}
func (m *MockGitLabClient) ResetNaysayerApproval(projectID, mrIID int) error { return nil }
func (m *MockGitLabClient) GetMRTargetBranch(projectID, mrIID int) (string, error) {
	return "main", nil
}
func (m *MockGitLabClient) GetMRDetails(projectID, mrIID int) (*gitlab.MRDetails, error) {
	return nil, nil
}
func (m *MockGitLabClient) ListMRComments(projectID, mrIID int) ([]gitlab.MRComment, error) {
	return nil, nil
}
func (m *MockGitLabClient) UpdateMRComment(projectID, mrIID, commentID int, newBody string) error {
	return nil
}
func (m *MockGitLabClient) AddOrUpdateMRComment(projectID, mrIID int, commentBody, commentType string) error {
	return nil
}
func (m *MockGitLabClient) FindLatestNaysayerComment(projectID, mrIID int, commentType ...string) (*gitlab.MRComment, error) {
	return nil, nil
}
func (m *MockGitLabClient) GetCurrentBotUsername() (string, error)                 { return "", nil }
func (m *MockGitLabClient) IsNaysayerBotAuthor(author map[string]interface{}) bool { return false }
func (m *MockGitLabClient) RebaseMR(projectID, mrIID int) (bool, error)            { return false, nil }
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
func (m *MockGitLabClient) ListAllOpenMRsWithDetails(projectID int) ([]gitlab.MRDetails, error) {
	return nil, nil
}
func (m *MockGitLabClient) CloseMR(projectID, mrIID int) error { return nil }
func (m *MockGitLabClient) GetPipelineJobs(projectID, pipelineID int) ([]gitlab.PipelineJob, error) {
	return nil, nil
}
func (m *MockGitLabClient) GetJobTrace(projectID, jobID int) (string, error) { return "", nil }
func (m *MockGitLabClient) FindLatestAtlantisComment(projectID, mrIID int) (*gitlab.MRComment, error) {
	return nil, nil
}
func (m *MockGitLabClient) AreAllPipelineJobsSucceeded(projectID, pipelineID int) (bool, error) {
	return false, nil
}
func (m *MockGitLabClient) CheckAtlantisCommentForPlanFailures(projectID, mrIID int) (bool, string) {
	return false, ""
}
func (m *MockGitLabClient) FindCommentByPattern(projectID, mrIID int, pattern string) (bool, error) {
	return false, nil
}

func TestCODEOWNERSSyncRule_isCODEOWNERSFile(t *testing.T) {
	rule := NewCODEOWNERSSyncRule(nil)

	tests := []struct {
		filePath string
		expected bool
	}{
		{"CODEOWNERS", true},
		{"codeowners", true},
		{".github/CODEOWNERS", true},
		{"README.md", false},
		{"developers.yaml", false},
	}

	for _, tt := range tests {
		t.Run(tt.filePath, func(t *testing.T) {
			assert.Equal(t, tt.expected, rule.isCODEOWNERSFile(tt.filePath))
		})
	}
}

func TestCODEOWNERSSyncRule_extractDataProductInfo(t *testing.T) {
	rule := NewCODEOWNERSSyncRule(nil)

	tests := []struct {
		filePath string
		expected *DataProductInfo
	}{
		{"dataproducts/aggregate/bookingsmaster/developers.yaml", &DataProductInfo{Type: "aggregate", Name: "bookingsmaster", Path: "dataproducts/aggregate/bookingsmaster"}},
		{"dataproducts/source/marketo/groups/foo.yaml", &DataProductInfo{Type: "source", Name: "marketo", Path: "dataproducts/source/marketo"}},
		{"other/path/file.yaml", nil},
		{"dataproducts/invalid/test/file.yaml", nil},
	}

	for _, tt := range tests {
		t.Run(tt.filePath, func(t *testing.T) {
			result := rule.extractDataProductInfo(tt.filePath)
			if tt.expected == nil {
				assert.Nil(t, result)
			} else {
				assert.Equal(t, tt.expected.Type, result.Type)
				assert.Equal(t, tt.expected.Name, result.Name)
				assert.Equal(t, tt.expected.Path, result.Path)
			}
		})
	}
}

func TestCODEOWNERSSyncRule_parseCODEOWNERSLine(t *testing.T) {
	rule := NewCODEOWNERSSyncRule(nil)

	tests := []struct {
		line     string
		expected *CODEOWNERSEntry
	}{
		{"/dataproducts/aggregate/bookingsmaster/ @alice @bob", &CODEOWNERSEntry{Path: "/dataproducts/aggregate/bookingsmaster/", Owners: []string{"alice", "bob"}}},
		{"# Comment", nil},
		{"[Section]", nil},
		{"", nil},
		{"* @dataverse/dataverse-groups/maintainers", nil}, // Group refs skipped
	}

	for _, tt := range tests {
		t.Run(tt.line, func(t *testing.T) {
			result := rule.parseCODEOWNERSLine(tt.line)
			if tt.expected == nil {
				assert.Nil(t, result)
			} else {
				assert.Equal(t, tt.expected.Path, result.Path)
				assert.ElementsMatch(t, tt.expected.Owners, result.Owners)
			}
		})
	}
}

func TestCODEOWNERSSyncRule_entriesMatch(t *testing.T) {
	rule := NewCODEOWNERSSyncRule(nil)

	tests := []struct {
		name     string
		expected CODEOWNERSEntry
		actual   CODEOWNERSEntry
		match    bool
	}{
		{"exact", CODEOWNERSEntry{"/path/", []string{"a", "b"}}, CODEOWNERSEntry{"/path/", []string{"a", "b"}}, true},
		{"reordered", CODEOWNERSEntry{"/path/", []string{"a", "b"}}, CODEOWNERSEntry{"/path/", []string{"b", "a"}}, true},
		{"diff path", CODEOWNERSEntry{"/path1/", []string{"a"}}, CODEOWNERSEntry{"/path2/", []string{"a"}}, false},
		{"diff owners", CODEOWNERSEntry{"/path/", []string{"a"}}, CODEOWNERSEntry{"/path/", []string{"b"}}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.match, rule.entriesMatch(tt.expected, tt.actual))
		})
	}
}

func TestCODEOWNERSSyncRule_ValidateLines_NotCODEOWNERS(t *testing.T) {
	rule := NewCODEOWNERSSyncRule(nil)
	decision, reason := rule.ValidateLines("README.md", "", nil)
	assert.Equal(t, shared.Approve, decision)
	assert.Contains(t, reason, "Not a CODEOWNERS file")
}

func TestCODEOWNERSSyncRule_ValidateLines_NoMRContext(t *testing.T) {
	rule := NewCODEOWNERSSyncRule(nil)
	decision, reason := rule.ValidateLines("CODEOWNERS", "", nil)
	assert.Equal(t, shared.ManualReview, decision)
	assert.Contains(t, reason, "MR context not available")
}

func TestCODEOWNERSSyncRule_ValidateLines_NoYAMLChanges(t *testing.T) {
	rule := NewCODEOWNERSSyncRule(NewMockGitLabClient())
	rule.SetMRContext(&shared.MRContext{
		ProjectID: 1, MRIID: 1,
		Changes: []gitlab.FileChange{{NewPath: "CODEOWNERS", Diff: "+/path/ @user"}},
		MRInfo:  &gitlab.MRInfo{SourceBranch: "feature", TargetBranch: "main"},
	})

	decision, reason := rule.ValidateLines("CODEOWNERS", "", nil)
	assert.Equal(t, shared.ManualReview, decision)
	assert.Contains(t, reason, "without corresponding YAML changes")
}

func TestCODEOWNERSSyncRule_ValidateLines_NewDataProduct(t *testing.T) {
	mock := NewMockGitLabClient()
	mock.SetFileContent("feature", "dataproducts/aggregate/new/developers.yaml", "group:\n  owners: [alice]")

	rule := NewCODEOWNERSSyncRule(mock)
	rule.SetMRContext(&shared.MRContext{
		ProjectID: 1, MRIID: 1,
		Changes: []gitlab.FileChange{
			{NewPath: "CODEOWNERS", Diff: "+/dataproducts/aggregate/new/ @alice"},
			{NewPath: "dataproducts/aggregate/new/developers.yaml", NewFile: true},
		},
		MRInfo: &gitlab.MRInfo{SourceBranch: "feature", TargetBranch: "main"},
	})

	decision, reason := rule.ValidateLines("CODEOWNERS", "", nil)
	assert.Equal(t, shared.ManualReview, decision)
	assert.Contains(t, reason, "New data product")
}

func TestCODEOWNERSSyncRule_ValidateLines_ExistingDataProductUpdate(t *testing.T) {
	mock := NewMockGitLabClient()
	mock.SetFileContent("feature", "dataproducts/aggregate/dp/developers.yaml", "group:\n  owners:\n    - alice\n    - bob")
	mock.SetFileContent("main", "dataproducts/aggregate/dp/developers.yaml", "group:\n  owners: [alice]")

	rule := NewCODEOWNERSSyncRule(mock)
	rule.SetMRContext(&shared.MRContext{
		ProjectID: 1, MRIID: 1,
		Changes: []gitlab.FileChange{
			{NewPath: "CODEOWNERS", Diff: "+/dataproducts/aggregate/dp/ @alice @bob"},
			{NewPath: "dataproducts/aggregate/dp/developers.yaml", NewFile: false},
		},
		MRInfo: &gitlab.MRInfo{SourceBranch: "feature", TargetBranch: "main"},
	})

	decision, reason := rule.ValidateLines("CODEOWNERS", "", nil)
	assert.Equal(t, shared.Approve, decision)
	assert.Contains(t, reason, "Auto-approved")
}

func TestCODEOWNERSSyncRule_ValidateLines_NewGroupInExistingDP(t *testing.T) {
	mock := NewMockGitLabClient()
	mock.SetFileContent("feature", "dataproducts/aggregate/dp/groups/grp.yaml", "group_name: grp\napprovers:\n  - approver1")
	mock.SetFileContent("main", "dataproducts/aggregate/dp/developers.yaml", "group:\n  owners: [alice]")

	rule := NewCODEOWNERSSyncRule(mock)
	rule.SetMRContext(&shared.MRContext{
		ProjectID: 1, MRIID: 1,
		Changes: []gitlab.FileChange{
			{NewPath: "CODEOWNERS", Diff: "+/dataproducts/aggregate/dp/groups/grp.yaml @approver1\n+/dataproducts/aggregate/dp/access-requests/groups/grp/ @approver1"},
			{NewPath: "dataproducts/aggregate/dp/groups/grp.yaml", NewFile: true},
		},
		MRInfo: &gitlab.MRInfo{SourceBranch: "feature", TargetBranch: "main"},
	})

	decision, reason := rule.ValidateLines("CODEOWNERS", "", nil)
	assert.Equal(t, shared.Approve, decision)
	assert.Contains(t, reason, "Auto-approved")
}

func TestCODEOWNERSSyncRule_ValidateLines_NewGroupInNewDP(t *testing.T) {
	mock := NewMockGitLabClient()
	mock.SetFileContent("feature", "dataproducts/aggregate/new/groups/grp.yaml", "group_name: grp\napprovers: [a]")
	// No developers.yaml in main - new data product

	rule := NewCODEOWNERSSyncRule(mock)
	rule.SetMRContext(&shared.MRContext{
		ProjectID: 1, MRIID: 1,
		Changes: []gitlab.FileChange{
			{NewPath: "CODEOWNERS", Diff: "+/path/ @a"},
			{NewPath: "dataproducts/aggregate/new/groups/grp.yaml", NewFile: true},
		},
		MRInfo: &gitlab.MRInfo{SourceBranch: "feature", TargetBranch: "main"},
	})

	decision, reason := rule.ValidateLines("CODEOWNERS", "", nil)
	assert.Equal(t, shared.ManualReview, decision)
	assert.Contains(t, reason, "New group in new data product")
}

func TestCODEOWNERSSyncRule_ValidateLines_ExtraUnexpectedEntry(t *testing.T) {
	mock := NewMockGitLabClient()
	mock.SetFileContent("feature", "dataproducts/aggregate/dp/developers.yaml", "group:\n  owners:\n    - alice\n    - bob")
	mock.SetFileContent("main", "dataproducts/aggregate/dp/developers.yaml", "group:\n  owners: [old]")

	rule := NewCODEOWNERSSyncRule(mock)
	rule.SetMRContext(&shared.MRContext{
		ProjectID: 1, MRIID: 1,
		Changes: []gitlab.FileChange{
			{NewPath: "CODEOWNERS", Diff: "+/dataproducts/aggregate/dp/ @alice @bob\n+* @mallory"},
			{NewPath: "dataproducts/aggregate/dp/developers.yaml", NewFile: false},
		},
		MRInfo: &gitlab.MRInfo{SourceBranch: "feature", TargetBranch: "main"},
	})

	decision, reason := rule.ValidateLines("CODEOWNERS", "", nil)
	assert.Equal(t, shared.ManualReview, decision)
	assert.Contains(t, reason, "Unexpected CODEOWNERS entry")
}

func TestCODEOWNERSSyncRule_ValidateLines_OrphanDeletion(t *testing.T) {
	mock := NewMockGitLabClient()
	mock.SetFileContent("feature", "dataproducts/aggregate/dp/developers.yaml", "group:\n  owners:\n    - alice\n    - bob")
	mock.SetFileContent("main", "dataproducts/aggregate/dp/developers.yaml", "group:\n  owners: [old]")

	rule := NewCODEOWNERSSyncRule(mock)
	rule.SetMRContext(&shared.MRContext{
		ProjectID: 1, MRIID: 1,
		Changes: []gitlab.FileChange{
			{NewPath: "CODEOWNERS", Diff: "+/dataproducts/aggregate/dp/ @alice @bob\n-/serviceaccounts/ @platformadmin"},
			{NewPath: "dataproducts/aggregate/dp/developers.yaml", NewFile: false},
		},
		MRInfo: &gitlab.MRInfo{SourceBranch: "feature", TargetBranch: "main"},
	})

	decision, reason := rule.ValidateLines("CODEOWNERS", "", nil)
	assert.Equal(t, shared.ManualReview, decision)
	assert.Contains(t, reason, "Unrelated CODEOWNERS deletion")
}

func TestCODEOWNERSSyncRule_ValidateLines_ValidModification(t *testing.T) {
	mock := NewMockGitLabClient()
	mock.SetFileContent("feature", "dataproducts/aggregate/dp/developers.yaml", "group:\n  owners:\n    - bob")
	mock.SetFileContent("main", "dataproducts/aggregate/dp/developers.yaml", "group:\n  owners: [alice, bob]")

	rule := NewCODEOWNERSSyncRule(mock)
	rule.SetMRContext(&shared.MRContext{
		ProjectID: 1, MRIID: 1,
		Changes: []gitlab.FileChange{
			{NewPath: "CODEOWNERS", Diff: "-/dataproducts/aggregate/dp/ @alice @bob\n+/dataproducts/aggregate/dp/ @bob"},
			{NewPath: "dataproducts/aggregate/dp/developers.yaml", NewFile: false},
		},
		MRInfo: &gitlab.MRInfo{SourceBranch: "feature", TargetBranch: "main"},
	})

	decision, reason := rule.ValidateLines("CODEOWNERS", "", nil)
	assert.Equal(t, shared.Approve, decision)
	assert.Contains(t, reason, "Auto-approved")
}

func TestCODEOWNERSSyncRule_GetCoveredLines(t *testing.T) {
	rule := NewCODEOWNERSSyncRule(nil)

	result := rule.GetCoveredLines("CODEOWNERS", "line1\nline2\nline3")
	assert.Len(t, result, 1)
	assert.Equal(t, 1, result[0].StartLine)
	assert.Equal(t, 3, result[0].EndLine)

	result = rule.GetCoveredLines("README.md", "# README")
	assert.Len(t, result, 0)
}
