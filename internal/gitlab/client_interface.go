package gitlab

// GitLabClient is an interface for GitLab API operations
// This interface allows for easy mocking in tests
type GitLabClient interface {
	// File operations
	FetchFileContent(projectID int, filePath, ref string) (*FileContent, error)
	GetMRTargetBranch(projectID, mrIID int) (string, error)
	GetMRDetails(projectID, mrIID int) (*MRDetails, error)

	// MR changes
	FetchMRChanges(projectID, mrIID int) ([]FileChange, error)

	// Comments
	AddMRComment(projectID, mrIID int, comment string) error
	AddOrUpdateMRComment(projectID, mrIID int, commentBody, commentType string) error
	ListMRComments(projectID, mrIID int) ([]MRComment, error)
	UpdateMRComment(projectID, mrIID, commentID int, newBody string) error
	FindLatestNaysayerComment(projectID, mrIID int, commentType ...string) (*MRComment, error)

	// Approvals
	ApproveMR(projectID, mrIID int) error
	ApproveMRWithMessage(projectID, mrIID int, message string) error
	ResetNaysayerApproval(projectID, mrIID int) error

	// Bot identity
	GetCurrentBotUsername() (string, error)
	IsNaysayerBotAuthor(author map[string]interface{}) bool
}

// Verify that Client implements GitLabClient interface
var _ GitLabClient = (*Client)(nil)
