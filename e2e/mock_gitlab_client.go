package e2e

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

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

// GetMRDetails returns MR details (minimal implementation for tests)
func (m *MockGitLabClient) GetMRDetails(projectID, mrIID int) (*gitlab.MRDetails, error) {
	return &gitlab.MRDetails{
		IID:          mrIID,
		SourceBranch: m.sourceBranch,
		TargetBranch: m.targetBranch,
		ProjectID:    projectID,
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
