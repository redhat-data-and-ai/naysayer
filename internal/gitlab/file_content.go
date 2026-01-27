package gitlab

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

// FileContent represents a file's content from GitLab API
type FileContent struct {
	FileName     string `json:"file_name"`
	FilePath     string `json:"file_path"`
	Size         int    `json:"size"`
	Encoding     string `json:"encoding"`
	Content      string `json:"content"`
	ContentSha1  string `json:"content_sha1"`
	Ref          string `json:"ref"`
	BlobID       string `json:"blob_id"`
	CommitID     string `json:"commit_id"`
	LastCommitID string `json:"last_commit_id"`
}

// FetchFileContent fetches file content from a specific commit/branch
func (c *Client) FetchFileContent(projectID int, filePath, ref string) (*FileContent, error) {
	// URL encode the file path
	encodedPath := url.QueryEscape(filePath)

	url := fmt.Sprintf("%s/api/v4/projects/%d/repository/files/%s?ref=%s",
		strings.TrimRight(c.config.BaseURL, "/"), projectID, encodedPath, ref)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", "Bearer "+c.config.Token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == 404 {
		return nil, fmt.Errorf("file not found: %s", filePath)
	}

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("GitLab API error %d: %s", resp.StatusCode, string(body))
	}

	var fileContent FileContent
	if err := json.NewDecoder(resp.Body).Decode(&fileContent); err != nil {
		return nil, err
	}

	// Decode base64 content if needed
	if fileContent.Encoding == "base64" {
		decodedContent, err := base64.StdEncoding.DecodeString(fileContent.Content)
		if err != nil {
			return nil, fmt.Errorf("failed to decode base64 content: %v", err)
		}
		fileContent.Content = string(decodedContent)
	}

	return &fileContent, nil
}

// GetMRTargetBranch fetches the target branch of a merge request
func (c *Client) GetMRTargetBranch(projectID, mrIID int) (string, error) {
	url := fmt.Sprintf("%s/api/v4/projects/%d/merge_requests/%d",
		strings.TrimRight(c.config.BaseURL, "/"), projectID, mrIID)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return "", err
	}

	req.Header.Set("Authorization", "Bearer "+c.config.Token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return "", err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("GitLab API error %d: %s", resp.StatusCode, string(body))
	}

	var mr struct {
		TargetBranch string `json:"target_branch"`
		SourceBranch string `json:"source_branch"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&mr); err != nil {
		return "", err
	}

	return mr.TargetBranch, nil
}

// MRDetails represents merge request details
type MRDetails struct {
	TargetBranch         string      `json:"target_branch"`
	SourceBranch         string      `json:"source_branch"`
	IID                  int         `json:"iid"`
	ProjectID            int         `json:"project_id"`             // Target project ID
	SourceProjectID      int         `json:"source_project_id"`      // Source project ID (for cross-fork MRs)
	TargetProjectID      int         `json:"target_project_id"`      // Target project ID (same as ProjectID)
	CreatedAt            string      `json:"created_at"`             // ISO 8601 format timestamp
	UpdatedAt            string      `json:"updated_at"`             // ISO 8601 format timestamp of last activity
	Pipeline             *MRPipeline `json:"pipeline"`               // Pipeline info (can be nil if no pipeline)
	BehindCommitsCount   int         `json:"behind_commits_count"`   // Number of commits behind target branch
	DivergedCommitsCount int         `json:"diverged_commits_count"` // Number of diverged commits
	MergeStatus          string      `json:"merge_status"`           // "can_be_merged", "cannot_be_merged", "checking", "unchecked"
	RebaseInProgress     bool        `json:"rebase_in_progress"`     // True if rebase is currently in progress
	HasConflicts         bool        `json:"has_conflicts"`          // True if MR has merge conflicts
}

// MRPipeline represents pipeline information for an MR
type MRPipeline struct {
	ID     int    `json:"id"`
	Status string `json:"status"` // running, pending, success, failed, canceled, skipped
}

// GetMRDetails fetches merge request details
func (c *Client) GetMRDetails(projectID, mrIID int) (*MRDetails, error) {
	url := fmt.Sprintf("%s/api/v4/projects/%d/merge_requests/%d",
		strings.TrimRight(c.config.BaseURL, "/"), projectID, mrIID)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", "Bearer "+c.config.Token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("GitLab API error %d: %s", resp.StatusCode, string(body))
	}

	var mrDetails MRDetails
	if err := json.NewDecoder(resp.Body).Decode(&mrDetails); err != nil {
		return nil, err
	}

	return &mrDetails, nil
}
