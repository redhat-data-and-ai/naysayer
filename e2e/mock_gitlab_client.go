package e2e

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/redhat-data-and-ai/naysayer/internal/gitlab"
)

// Verify that MockGitLabClient implements GitLabClient interface
var _ gitlab.GitLabClient = (*MockGitLabClient)(nil)

// MockGitLabClient implements a mock GitLab client for E2E testing
// It reads files from the filesystem instead of making HTTP calls
type MockGitLabClient struct {
	beforeDir string // Points to the before/ directory (target branch content)
	afterDir  string // Points to the after/ directory (source branch content)

	// MR details
	sourceBranch string
	targetBranch string

	// File changes (generated from before/after comparison)
	fileChanges []gitlab.FileChange

	// Captured interactions for validation
	CapturedComments  []CapturedComment
	CapturedApprovals []CapturedApproval
	FetchedFiles      []string

	// Optional: for auto-rebase E2E tests. When set, ListOpenMRs/ListOpenMRsWithDetails return these MRs.
	OpenMRsForAutoRebase []int
	// Optional: for auto-rebase E2E. When >= 0, CompareBranches returns this many commits (behind). When < 0, returns 0 (up-to-date).
	AutoRebaseBehindCount int
}

// CapturedComment represents a comment that would be posted to GitLab
type CapturedComment struct {
	ProjectID int
	MRIID     int
	Comment   string
	Tag       string // "approval" or "manual-review"
}

// CapturedApproval represents an approval that would be sent to GitLab
type CapturedApproval struct {
	ProjectID int
	MRIID     int
	Message   string
}

// NewMockGitLabClient creates a new mock GitLab client
// beforeDir should point to the before/ directory (represents target branch)
// afterDir should point to the after/ directory (represents source branch)
func NewMockGitLabClient(beforeDir, afterDir string) *MockGitLabClient {
	return &MockGitLabClient{
		beforeDir:         beforeDir,
		afterDir:          afterDir,
		sourceBranch:      "feature/test", // Default, can be overridden
		targetBranch:      "main",         // Default, can be overridden
		CapturedComments:  []CapturedComment{},
		CapturedApprovals: []CapturedApproval{},
		FetchedFiles:      []string{},
	}
}

// SetMRBranches sets the source and target branches for this mock MR
func (m *MockGitLabClient) SetMRBranches(sourceBranch, targetBranch string) {
	m.sourceBranch = sourceBranch
	m.targetBranch = targetBranch
}

// SetFileChanges sets the file changes for this mock
func (m *MockGitLabClient) SetFileChanges(changes []gitlab.FileChange) {
	m.fileChanges = changes
}

// GetFileContent reads file content from the appropriate directory based on the ref (branch)
func (m *MockGitLabClient) GetFileContent(projectID int, filePath, ref string) (string, error) {
	// Track which files were fetched
	m.FetchedFiles = append(m.FetchedFiles, filePath)

	// Determine which directory to read from based on branch
	// target branch = before/ directory
	// source branch = after/ directory
	var baseDir string
	if ref == m.targetBranch {
		baseDir = m.beforeDir
	} else {
		baseDir = m.afterDir
	}

	fullPath := filepath.Join(baseDir, filePath)
	content, err := os.ReadFile(fullPath) // #nosec G304 - reading test fixture files
	if err != nil {
		return "", fmt.Errorf("file not found: %s (ref: %s)", filePath, ref)
	}

	return string(content), nil
}

// FetchMRChanges returns the file changes set via SetFileChanges
func (m *MockGitLabClient) FetchMRChanges(projectID, mrID int) ([]gitlab.FileChange, error) {
	if m.fileChanges == nil {
		return []gitlab.FileChange{}, nil
	}
	return m.fileChanges, nil
}

// AddMRComment captures the comment instead of posting to GitLab
func (m *MockGitLabClient) AddMRComment(projectID, mrID int, comment string) error {
	m.CapturedComments = append(m.CapturedComments, CapturedComment{
		ProjectID: projectID,
		MRIID:     mrID,
		Comment:   comment,
		Tag:       "",
	})
	return nil
}

// AddOrUpdateMRComment captures the comment with a tag
func (m *MockGitLabClient) AddOrUpdateMRComment(projectID, mrID int, comment string, tag string) error {
	m.CapturedComments = append(m.CapturedComments, CapturedComment{
		ProjectID: projectID,
		MRIID:     mrID,
		Comment:   comment,
		Tag:       tag,
	})
	return nil
}

// ApproveMR captures the approval request
func (m *MockGitLabClient) ApproveMR(projectID, mrID int) error {
	m.CapturedApprovals = append(m.CapturedApprovals, CapturedApproval{
		ProjectID: projectID,
		MRIID:     mrID,
		Message:   "",
	})
	return nil
}

// ApproveMRWithMessage captures the approval with a message
func (m *MockGitLabClient) ApproveMRWithMessage(projectID, mrID int, message string) error {
	m.CapturedApprovals = append(m.CapturedApprovals, CapturedApproval{
		ProjectID: projectID,
		MRIID:     mrID,
		Message:   message,
	})
	return nil
}

// ResetNaysayerApproval is a no-op for mock client
func (m *MockGitLabClient) ResetNaysayerApproval(projectID, mrID int) error {
	// In tests, we don't need to reset approvals
	// Just return success
	return nil
}

// GetLatestCommentByTag retrieves the most recent comment with a specific tag
func (m *MockGitLabClient) GetLatestCommentByTag(tag string) (string, bool) {
	// Search in reverse to get the latest
	for i := len(m.CapturedComments) - 1; i >= 0; i-- {
		if m.CapturedComments[i].Tag == tag {
			return m.CapturedComments[i].Comment, true
		}
	}
	return "", false
}

// GetAllComments returns all captured comments
func (m *MockGitLabClient) GetAllComments() []string {
	comments := make([]string, len(m.CapturedComments))
	for i, captured := range m.CapturedComments {
		comments[i] = captured.Comment
	}
	return comments
}

// WasApproved returns true if ApproveMR was called
func (m *MockGitLabClient) WasApproved() bool {
	return len(m.CapturedApprovals) > 0
}

// GetApprovalMessage returns the approval message if approved
func (m *MockGitLabClient) GetApprovalMessage() string {
	if len(m.CapturedApprovals) > 0 {
		return m.CapturedApprovals[0].Message
	}
	return ""
}

// Reset clears all captured data
func (m *MockGitLabClient) Reset() {
	m.CapturedComments = []CapturedComment{}
	m.CapturedApprovals = []CapturedApproval{}
	m.FetchedFiles = []string{}
}

// ValidateFileWasFetched checks if a specific file was fetched
func (m *MockGitLabClient) ValidateFileWasFetched(filePath string) bool {
	for _, fetched := range m.FetchedFiles {
		if fetched == filePath {
			return true
		}
	}
	return false
}

// GetCommentCount returns the number of captured comments
func (m *MockGitLabClient) GetCommentCount() int {
	return len(m.CapturedComments)
}

// GetApprovalCount returns the number of captured approvals
func (m *MockGitLabClient) GetApprovalCount() int {
	return len(m.CapturedApprovals)
}

// ContainsCommentPhrase checks if any comment contains a specific phrase
func (m *MockGitLabClient) ContainsCommentPhrase(phrase string) bool {
	for _, comment := range m.CapturedComments {
		if strings.Contains(comment.Comment, phrase) {
			return true
		}
	}
	return false
}

// FetchFileContent reads file content and returns FileContent struct
func (m *MockGitLabClient) FetchFileContent(projectID int, filePath, ref string) (*gitlab.FileContent, error) {
	content, err := m.GetFileContent(projectID, filePath, ref)
	if err != nil {
		return nil, err
	}

	return &gitlab.FileContent{
		FileName: filepath.Base(filePath),
		FilePath: filePath,
		Content:  content,
		Ref:      ref,
	}, nil
}

// GetMRTargetBranch returns the target branch
func (m *MockGitLabClient) GetMRTargetBranch(projectID, mrIID int) (string, error) {
	return m.targetBranch, nil
}

// GetMRDetails returns MR details (minimal implementation for tests).
// For autorebase eligibility, returns recent CreatedAt and success Pipeline when used with OpenMRsForAutoRebase.
func (m *MockGitLabClient) GetMRDetails(projectID, mrIID int) (*gitlab.MRDetails, error) {
	createdAt := time.Now().Add(-24 * time.Hour).Format(time.RFC3339) // 1 day ago
	return &gitlab.MRDetails{
		IID:                  mrIID,
		SourceBranch:         m.sourceBranch,
		TargetBranch:         m.targetBranch,
		ProjectID:            projectID,
		CreatedAt:            createdAt,
		Pipeline:             &gitlab.MRPipeline{ID: 1, Status: "success"},
		RebaseInProgress:     false,
		HasConflicts:         false,
		MergeStatus:          "can_be_merged",
		BehindCommitsCount:   0,
		DivergedCommitsCount: 0,
	}, nil
}

// ListMRComments returns captured comments as MRComment structs
func (m *MockGitLabClient) ListMRComments(projectID, mrIID int) ([]gitlab.MRComment, error) {
	var comments []gitlab.MRComment
	for i, captured := range m.CapturedComments {
		comments = append(comments, gitlab.MRComment{
			ID:   i + 1,
			Body: captured.Comment,
			Author: map[string]interface{}{
				"username": "naysayer-bot",
			},
		})
	}
	return comments, nil
}

// UpdateMRComment captures comment updates
func (m *MockGitLabClient) UpdateMRComment(projectID, mrIID, commentID int, newBody string) error {
	// In tests, just add as a new comment
	return m.AddMRComment(projectID, mrIID, newBody)
}

// FindLatestNaysayerComment finds the latest comment by type
func (m *MockGitLabClient) FindLatestNaysayerComment(projectID, mrIID int, commentType ...string) (*gitlab.MRComment, error) {
	// Search in reverse for latest comment
	for i := len(m.CapturedComments) - 1; i >= 0; i-- {
		if len(commentType) > 0 && m.CapturedComments[i].Tag == commentType[0] {
			return &gitlab.MRComment{
				ID:   i + 1,
				Body: m.CapturedComments[i].Comment,
				Author: map[string]interface{}{
					"username": "naysayer-bot",
				},
			}, nil
		}
	}
	return nil, nil
}

// GetCurrentBotUsername returns the bot username
func (m *MockGitLabClient) GetCurrentBotUsername() (string, error) {
	return "naysayer-bot", nil
}

// IsNaysayerBotAuthor checks if author is the naysayer bot
func (m *MockGitLabClient) IsNaysayerBotAuthor(author map[string]interface{}) bool {
	if username, ok := author["username"].(string); ok {
		return username == "naysayer-bot"
	}
	return false
}

// CompareBranches returns commits that target has but source doesn't (same-project only in real API).
func (m *MockGitLabClient) CompareBranches(sourceProjectID int, sourceBranch string, targetProjectID int, targetBranch string) (*gitlab.CompareResult, error) {
	count := 0
	if m.AutoRebaseBehindCount >= 0 {
		count = m.AutoRebaseBehindCount
	}
	commits := make([]gitlab.CompareCommit, count)
	for i := 0; i < count; i++ {
		commits[i] = gitlab.CompareCommit{
			ID:      fmt.Sprintf("e2e-commit-%d-%d", targetProjectID, i+1),
			ShortID: "e2e",
			Title:   "E2E test commit",
		}
	}
	return &gitlab.CompareResult{Commits: commits}, nil
}

// GetBranchCommit returns a dummy SHA for E2E.
func (m *MockGitLabClient) GetBranchCommit(projectID int, branch string) (string, error) {
	return "e2e-main-sha", nil
}

// CompareCommits returns behind count for E2E (fork MR path).
func (m *MockGitLabClient) CompareCommits(projectID int, fromSHA, toSHA string) (*gitlab.CompareResult, error) {
	count := 0
	if m.AutoRebaseBehindCount >= 0 {
		count = m.AutoRebaseBehindCount
	}
	commits := make([]gitlab.CompareCommit, count)
	for i := 0; i < count; i++ {
		commits[i] = gitlab.CompareCommit{
			ID:      fmt.Sprintf("e2e-commit-%d-%d", projectID, i+1),
			ShortID: "e2e",
			Title:   "E2E test commit",
		}
	}
	return &gitlab.CompareResult{Commits: commits}, nil
}

// RebaseMR simulates rebase for auto-rebase E2E. Returns success so the handler can post the automated comment.
func (m *MockGitLabClient) RebaseMR(projectID, mrIID int) (bool, error) {
	return true, nil
}

// ListOpenMRs returns open MR IIDs. For auto-rebase E2E, set OpenMRsForAutoRebase to return specific MRs.
func (m *MockGitLabClient) ListOpenMRs(projectID int) ([]int, error) {
	if len(m.OpenMRsForAutoRebase) > 0 {
		return m.OpenMRsForAutoRebase, nil
	}
	return []int{}, nil
}

// ListOpenMRsWithDetails is a stub for mock client
func (m *MockGitLabClient) ListOpenMRsWithDetails(projectID int) ([]gitlab.MRDetails, error) {
	// Simulate the new behavior: call GetMRDetails for each open MR
	// This mimics the real implementation's N+1 query pattern
	openMRs, err := m.ListOpenMRs(projectID)
	if err != nil {
		return nil, err
	}

	details := make([]gitlab.MRDetails, 0, len(openMRs))
	for _, mrIID := range openMRs {
		mrDetail, err := m.GetMRDetails(projectID, mrIID)
		if err != nil {
			// Skip MRs that fail to fetch
			continue
		}
		details = append(details, *mrDetail)
	}

	return details, nil
}

// ListAllOpenMRsWithDetails lists all open merge requests (mock implementation)
func (m *MockGitLabClient) ListAllOpenMRsWithDetails(projectID int) ([]gitlab.MRDetails, error) {
	// For mock, return same as ListOpenMRsWithDetails
	return m.ListOpenMRsWithDetails(projectID)
}

// CloseMR closes a merge request (mock implementation)
func (m *MockGitLabClient) CloseMR(projectID, mrIID int) error {
	// Mock implementation - just log the action
	return nil
}

// FindCommentByPattern checks if a comment with the pattern exists (mock implementation)
func (m *MockGitLabClient) FindCommentByPattern(projectID, mrIID int, pattern string) (bool, error) {
	// Mock implementation - check captured comments
	for _, comment := range m.CapturedComments {
		if comment.ProjectID == projectID && comment.MRIID == mrIID {
			if strings.Contains(comment.Comment, pattern) {
				return true, nil
			}
		}
	}
	return false, nil
}

// GetPipelineJobs is a stub for mock client
func (m *MockGitLabClient) GetPipelineJobs(projectID, pipelineID int) ([]gitlab.PipelineJob, error) {
	// Return empty jobs for e2e tests
	return []gitlab.PipelineJob{}, nil
}

// GetJobTrace is a stub for mock client
func (m *MockGitLabClient) GetJobTrace(projectID, jobID int) (string, error) {
	// Return empty trace for e2e tests
	return "", nil
}

// FindLatestAtlantisComment is a stub for mock client
func (m *MockGitLabClient) FindLatestAtlantisComment(projectID, mrIID int) (*gitlab.MRComment, error) {
	// Return nil for e2e tests (no atlantis comments)
	return nil, nil
}

// AreAllPipelineJobsSucceeded is a stub for mock client
func (m *MockGitLabClient) AreAllPipelineJobsSucceeded(projectID, pipelineID int) (bool, error) {
	// Return true for e2e tests (all jobs succeeded)
	return true, nil
}

// CheckAtlantisCommentForPlanFailures is a stub for mock client
func (m *MockGitLabClient) CheckAtlantisCommentForPlanFailures(projectID, mrIID int) (bool, string) {
	// Return false for e2e tests (no plan failures, allow rebase)
	return false, ""
}

// ListDirectoryFiles lists files in a directory from the mock filesystem
func (m *MockGitLabClient) ListDirectoryFiles(projectID int, dirPath, ref string) ([]gitlab.RepositoryFile, error) {
	// Determine which directory to read from based on branch
	var baseDir string
	if ref == m.targetBranch {
		baseDir = m.beforeDir
	} else {
		baseDir = m.afterDir
	}

	fullPath := filepath.Join(baseDir, dirPath)
	entries, err := os.ReadDir(fullPath)
	if err != nil {
		// Directory doesn't exist, return empty list
		return []gitlab.RepositoryFile{}, nil
	}

	var files []gitlab.RepositoryFile
	for _, entry := range entries {
		fileType := "blob"
		if entry.IsDir() {
			fileType = "tree"
		}
		files = append(files, gitlab.RepositoryFile{
			Name: entry.Name(),
			Path: filepath.Join(dirPath, entry.Name()),
			Type: fileType,
		})
	}

	return files, nil
}
