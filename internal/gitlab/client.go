package gitlab

import (
	"bytes"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/redhat-data-and-ai/naysayer/internal/config"
)

// Client handles GitLab API operations
type Client struct {
	config config.GitLabConfig
	http   *http.Client
}

// createHTTPClient creates an HTTP client with custom TLS configuration
func createHTTPClient(cfg config.GitLabConfig) (*http.Client, error) {
	transport := &http.Transport{}

	// Configure TLS settings
	tlsConfig := &tls.Config{}

	// Handle insecure TLS (skip certificate verification)
	if cfg.InsecureTLS {
		tlsConfig.InsecureSkipVerify = true
	}

	// Handle custom CA certificate
	if cfg.CACertPath != "" {
		caCert, err := os.ReadFile(cfg.CACertPath)
		if err != nil {
			return nil, fmt.Errorf("failed to read CA certificate from %s: %w", cfg.CACertPath, err)
		}

		caCertPool := x509.NewCertPool()
		if !caCertPool.AppendCertsFromPEM(caCert) {
			return nil, fmt.Errorf("failed to parse CA certificate from %s", cfg.CACertPath)
		}

		tlsConfig.RootCAs = caCertPool
	}

	transport.TLSClientConfig = tlsConfig

	return &http.Client{
		Transport: transport,
	}, nil
}

// NewClient creates a new GitLab API client
func NewClient(cfg config.GitLabConfig) *Client {
	httpClient, err := createHTTPClient(cfg)
	if err != nil {
		// Fallback to default client if TLS configuration fails
		httpClient = &http.Client{}
	}

	return &Client{
		config: cfg,
		http:   httpClient,
	}
}

// NewClientWithConfig creates a new GitLab API client with full config
func NewClientWithConfig(cfg *config.Config) *Client {
	httpClient, err := createHTTPClient(cfg.GitLab)
	if err != nil {
		// Fallback to default client if TLS configuration fails
		httpClient = &http.Client{}
	}

	return &Client{
		config: cfg.GitLab,
		http:   httpClient,
	}
}

// FetchMRChanges fetches merge request changes from GitLab API
func (c *Client) FetchMRChanges(projectID, mrIID int) ([]FileChange, error) {
	url := fmt.Sprintf("%s/api/v4/projects/%d/merge_requests/%d/changes",
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

	var response MRChanges
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, err
	}

	// Convert to FileChange slice
	fileChanges := make([]FileChange, len(response.Changes))
	for i, change := range response.Changes {
		fileChanges[i] = FileChange{
			OldPath:     change.OldPath,
			NewPath:     change.NewPath,
			AMode:       change.AMode,
			BMode:       change.BMode,
			NewFile:     change.NewFile,
			RenamedFile: change.RenamedFile,
			DeletedFile: change.DeletedFile,
			Diff:        change.Diff,
		}
	}

	return fileChanges, nil
}

// ExtractMRInfo extracts merge request information from webhook payload
func ExtractMRInfo(payload map[string]interface{}) (*MRInfo, error) {
	var projectID, mrIID int
	var title, author, sourceBranch, targetBranch, state string

	// Extract from object_attributes
	if objectAttrs, ok := payload["object_attributes"].(map[string]interface{}); ok {
		if iid, ok := objectAttrs["iid"]; ok {
			switch v := iid.(type) {
			case float64:
				mrIID = int(v)
			case int:
				mrIID = v
			case string:
				mrIID, _ = strconv.Atoi(v)
			}
		}

		if titleVal, ok := objectAttrs["title"].(string); ok {
			title = titleVal
		}

		if sourceVal, ok := objectAttrs["source_branch"].(string); ok {
			sourceBranch = sourceVal
		}

		if targetVal, ok := objectAttrs["target_branch"].(string); ok {
			targetBranch = targetVal
		}

		if stateVal, ok := objectAttrs["state"].(string); ok {
			state = stateVal
		}
	}

	// Extract project ID
	if project, ok := payload["project"].(map[string]interface{}); ok {
		if id, ok := project["id"]; ok {
			switch v := id.(type) {
			case float64:
				projectID = int(v)
			case int:
				projectID = v
			case string:
				projectID, _ = strconv.Atoi(v)
			}
		}
	}

	// Extract author from user
	if user, ok := payload["user"].(map[string]interface{}); ok {
		if username, ok := user["username"].(string); ok {
			author = username
		}
	}

	if projectID == 0 || mrIID == 0 {
		return nil, fmt.Errorf("missing project ID (%d) or MR IID (%d)", projectID, mrIID)
	}

	return &MRInfo{
		ProjectID:    projectID,
		MRIID:        mrIID,
		Title:        title,
		Author:       author,
		SourceBranch: sourceBranch,
		TargetBranch: targetBranch,
		State:        state,
	}, nil
}

// AddMRComment adds a comment to a merge request
func (c *Client) AddMRComment(projectID, mrIID int, comment string) error {
	url := fmt.Sprintf("%s/api/v4/projects/%d/merge_requests/%d/notes",
		strings.TrimRight(c.config.BaseURL, "/"), projectID, mrIID)

	payload := map[string]string{
		"body": comment,
	}

	jsonPayload, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal comment payload: %w", err)
	}

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonPayload))
	if err != nil {
		return fmt.Errorf("failed to create comment request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+c.config.Token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("failed to add comment: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	switch resp.StatusCode {
	case 201:
		return nil // Success
	case 401:
		return fmt.Errorf("comment failed: insufficient permissions")
	case 404:
		return fmt.Errorf("comment failed: MR not found")
	default:
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("comment failed with status %d: %s", resp.StatusCode, string(body))
	}
}

// ApproveMR approves a merge request (simple approval without message)
func (c *Client) ApproveMR(projectID, mrIID int) error {
	return c.ApproveMRWithMessage(projectID, mrIID, "")
}

// ApproveMRWithMessage approves a merge request with a custom approval message
func (c *Client) ApproveMRWithMessage(projectID, mrIID int, message string) error {
	url := fmt.Sprintf("%s/api/v4/projects/%d/merge_requests/%d/approve",
		strings.TrimRight(c.config.BaseURL, "/"), projectID, mrIID)

	var jsonPayload []byte
	var err error

	if message != "" {
		payload := map[string]string{
			"note": message,
		}
		jsonPayload, err = json.Marshal(payload)
	} else {
		jsonPayload = []byte("{}")
	}

	if err != nil {
		return fmt.Errorf("failed to marshal approval payload: %w", err)
	}

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonPayload))
	if err != nil {
		return fmt.Errorf("failed to create approval request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+c.config.Token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("failed to approve MR: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	switch resp.StatusCode {
	case 201:
		return nil // Success
	case 401:
		return fmt.Errorf("approval failed: insufficient permissions")
	case 404:
		return fmt.Errorf("approval failed: MR not found")
	case 405:
		return fmt.Errorf("approval failed: MR already approved or cannot be approved")
	default:
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("approval failed with status %d: %s", resp.StatusCode, string(body))
	}
}

// MRComment represents a GitLab merge request comment
type MRComment struct {
	ID        int                    `json:"id"`
	Body      string                 `json:"body"`
	CreatedAt string                 `json:"created_at"`
	UpdatedAt string                 `json:"updated_at"`
	Author    map[string]interface{} `json:"author"`
}

// listMRComments retrieves all comments for a merge request (internal use only)
func (c *Client) listMRComments(projectID, mrIID int) ([]MRComment, error) {
	// Use a larger page size to get more comments (GitLab API supports up to 100 per page)
	url := fmt.Sprintf("%s/api/v4/projects/%d/merge_requests/%d/notes?sort=desc&order_by=created_at&per_page=100",
		strings.TrimRight(c.config.BaseURL, "/"), projectID, mrIID)

	fmt.Printf("DEBUG listMRComments: Fetching comments from URL: %s\n", url)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create list comments request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+c.config.Token)

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to list comments: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	switch resp.StatusCode {
	case 200:
		var comments []MRComment
		if err := json.NewDecoder(resp.Body).Decode(&comments); err != nil {
			return nil, fmt.Errorf("failed to decode comments response: %w", err)
		}
		fmt.Printf("DEBUG listMRComments: Successfully retrieved %d comments\n", len(comments))
		return comments, nil
	case 401:
		return nil, fmt.Errorf("list comments failed: insufficient permissions")
	case 404:
		return nil, fmt.Errorf("list comments failed: MR not found")
	default:
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("list comments failed with status %d: %s", resp.StatusCode, string(body))
	}
}

// UpdateMRComment updates an existing comment on a merge request
func (c *Client) UpdateMRComment(projectID, mrIID, commentID int, newBody string) error {
	url := fmt.Sprintf("%s/api/v4/projects/%d/merge_requests/%d/notes/%d",
		strings.TrimRight(c.config.BaseURL, "/"), projectID, mrIID, commentID)

	payload := map[string]string{
		"body": newBody,
	}

	jsonPayload, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal update comment payload: %w", err)
	}

	req, err := http.NewRequest("PUT", url, bytes.NewBuffer(jsonPayload))
	if err != nil {
		return fmt.Errorf("failed to create update comment request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+c.config.Token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("failed to update comment: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	switch resp.StatusCode {
	case 200:
		return nil // Success
	case 401:
		return fmt.Errorf("update comment failed: insufficient permissions")
	case 404:
		return fmt.Errorf("update comment failed: comment or MR not found")
	case 403:
		return fmt.Errorf("update comment failed: cannot edit this comment")
	default:
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("update comment failed with status %d: %s", resp.StatusCode, string(body))
	}
}


// FindLatestNaysayerComment searches for the most recent naysayer comment regardless of type
func (c *Client) FindLatestNaysayerComment(projectID, mrIID int) (*MRComment, error) {
	comments, err := c.listMRComments(projectID, mrIID)
	if err != nil {
		return nil, fmt.Errorf("failed to list comments: %w", err)
	}

	fmt.Printf("DEBUG FindLatestNaysayerComment: Retrieved %d comments from GitLab API\n", len(comments))

	// Look for any comment from naysayer bot (comments are already sorted by created_at desc)
	// Since the HTML identifiers aren't actually being included in real comments,
	// we'll just find the latest comment from the naysayer bot
	for i, comment := range comments {
		fmt.Printf("DEBUG FindLatestNaysayerComment: Comment %d - ID: %d, Author: %+v\n", i, comment.ID, comment.Author)
		if c.isNaysayerBotAuthor(comment.Author) {
			fmt.Printf("DEBUG FindLatestNaysayerComment: FOUND existing bot comment ID %d - will UPDATE\n", comment.ID)
			return &comment, nil
		}
	}
	
	fmt.Printf("DEBUG FindLatestNaysayerComment: No bot comment found - will CREATE NEW\n")
	return nil, nil // No naysayer comment found
}

// isNaysayerBotAuthor checks if the comment author is a naysayer bot
func (c *Client) isNaysayerBotAuthor(author map[string]interface{}) bool {	
	// Check username field first
	if username, ok := author["username"].(string); ok {
		// Match the exact pattern we found: project_<ID>_bot_<hash>
		if strings.HasPrefix(username, "project_") && strings.Contains(username, "_bot_") {
			return true
		}
		// Also check for the simpler naysayer-bot pattern
		if strings.Contains(username, "naysayer-bot") {
			return true
		}
	}
	
	// Check name field as fallback
	if name, ok := author["name"].(string); ok {
		if name == "naysayer-bot" {
			return true
		}
	}
	
	return false
}

// AddOrUpdateMRComment adds a new comment or updates the latest existing naysayer comment
func (c *Client) AddOrUpdateMRComment(projectID, mrIID int, commentBody, commentType string) error {
	fmt.Printf("DEBUG AddOrUpdateMRComment: Looking for existing comment...\n")
	// Try to find the latest naysayer comment regardless of type
	existingComment, err := c.FindLatestNaysayerComment(projectID, mrIID)
	if err != nil {
		fmt.Printf("DEBUG AddOrUpdateMRComment: Error finding existing comment: %v\n", err)
		return fmt.Errorf("failed to search for existing naysayer comment: %w", err)
	}

	if existingComment != nil {
		fmt.Printf("DEBUG AddOrUpdateMRComment: UPDATING existing comment ID %d\n", existingComment.ID)
		// Update the latest naysayer comment
		return c.UpdateMRComment(projectID, mrIID, existingComment.ID, commentBody)
	} else {
		fmt.Printf("DEBUG AddOrUpdateMRComment: CREATING new comment\n")
		// Create new comment
		return c.AddMRComment(projectID, mrIID, commentBody)
	}
}
