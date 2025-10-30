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
	tlsConfig := &tls.Config{
		MinVersion: tls.VersionTLS12, // Enforce TLS 1.2 minimum for security
	}

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

// ResetNaysayerApproval revokes naysayer's approval for a merge request
// This is called when naysayer changes its decision from approve to manual review
func (c *Client) ResetNaysayerApproval(projectID, mrIID int) error {
	url := fmt.Sprintf("%s/api/v4/projects/%d/merge_requests/%d/unapprove",
		strings.TrimRight(c.config.BaseURL, "/"), projectID, mrIID)

	req, err := http.NewRequest("POST", url, bytes.NewBuffer([]byte("{}")))
	if err != nil {
		return fmt.Errorf("failed to create reset approval request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+c.config.Token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("failed to reset naysayer approval: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	switch resp.StatusCode {
	case 201:
		return nil // Success
	case 401:
		return fmt.Errorf("reset approval failed: insufficient permissions")
	case 404:
		return fmt.Errorf("reset approval failed: MR not found")
	case 405:
		return fmt.Errorf("reset approval failed: MR not approved or cannot be reset")
	default:
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("reset approval failed with status %d: %s", resp.StatusCode, string(body))
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

// ListMRComments retrieves all comments for a merge request
func (c *Client) ListMRComments(projectID, mrIID int) ([]MRComment, error) {
	// Use a larger page size to get more comments (GitLab API supports up to 100 per page)
	url := fmt.Sprintf("%s/api/v4/projects/%d/merge_requests/%d/notes?sort=desc&order_by=created_at&per_page=100",
		strings.TrimRight(c.config.BaseURL, "/"), projectID, mrIID)

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

// FindLatestNaysayerComment searches for the most recent comment from the current naysayer bot instance
// If commentType is provided, only returns comments of that type. If empty, returns any naysayer comment.
func (c *Client) FindLatestNaysayerComment(projectID, mrIID int, commentType ...string) (*MRComment, error) {
	comments, err := c.ListMRComments(projectID, mrIID)
	if err != nil {
		return nil, fmt.Errorf("failed to list comments: %w", err)
	}

	// Get current bot username (fallback to any naysayer bot if fails)
	currentBotUsername, _ := c.GetCurrentBotUsername()

	// Determine if we need to filter by comment type
	filterByType := len(commentType) > 0 && commentType[0] != ""

	// Find the latest matching comment (comments are sorted by created_at desc)
	for _, comment := range comments {
		// Check if comment is from our bot and matches type (if specified)
		if c.isOurBotComment(comment.Author, currentBotUsername) &&
			(!filterByType || c.matchesCommentType(comment.Body, commentType[0])) {
			return &comment, nil
		}
	}

	return nil, nil // No matching comment found
}

// isOurBotComment checks if a comment is from our bot instance
func (c *Client) isOurBotComment(author map[string]interface{}, currentBotUsername string) bool {
	if currentBotUsername != "" {
		return author["username"] == currentBotUsername
	}
	return c.IsNaysayerBotAuthor(author)
}

// matchesCommentType checks if a comment body matches the expected comment type
func (c *Client) matchesCommentType(body, commentType string) bool {
	switch commentType {
	case "approval":
		return strings.Contains(body, "<!-- naysayer-comment-id: approval -->")
	case "manual-review":
		return strings.Contains(body, "<!-- naysayer-comment-id: manual-review -->")
	default:
		// For unknown types, match any naysayer comment
		return strings.Contains(body, "<!-- naysayer-comment-id:")
	}
}

// GetCurrentBotUsername identifies the current bot's username by calling GitLab API
func (c *Client) GetCurrentBotUsername() (string, error) {
	url := fmt.Sprintf("%s/api/v4/user", strings.TrimRight(c.config.BaseURL, "/"))

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create user info request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+c.config.Token)

	resp, err := c.http.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to get user info: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("user info request failed with status %d: %s", resp.StatusCode, string(body))
	}

	var userInfo map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&userInfo); err != nil {
		return "", fmt.Errorf("failed to decode user info response: %w", err)
	}

	if username, ok := userInfo["username"].(string); ok {
		return username, nil
	}

	return "", fmt.Errorf("username not found in user info response")
}

// IsNaysayerBotAuthor checks if the comment author is a naysayer bot
func (c *Client) IsNaysayerBotAuthor(author map[string]interface{}) bool {
	// Check username patterns
	if username, ok := author["username"].(string); ok {
		return (strings.HasPrefix(username, "project_") && strings.Contains(username, "_bot_")) ||
			strings.Contains(username, "naysayer-bot")
	}

	// Check name field as fallback
	if name, ok := author["name"].(string); ok {
		return name == "naysayer-bot"
	}

	return false
}

// AddOrUpdateMRComment adds a new comment or updates the latest existing naysayer comment of the same type
func (c *Client) AddOrUpdateMRComment(projectID, mrIID int, commentBody, commentType string) error {
	// Find the latest naysayer comment of the same type
	existingComment, err := c.FindLatestNaysayerComment(projectID, mrIID, commentType)
	if err != nil {
		return fmt.Errorf("failed to search for existing comment: %w", err)
	}

	// Update existing comment or create new one
	if existingComment != nil {
		if err := c.UpdateMRComment(projectID, mrIID, existingComment.ID, commentBody); err != nil {
			// If update fails due to permissions, fallback to creating new comment
			if strings.Contains(err.Error(), "cannot edit this comment") ||
				strings.Contains(err.Error(), "insufficient permissions") {
				return c.AddMRComment(projectID, mrIID, commentBody)
			}
			return err
		}
		return nil
	}

	// No existing comment found, create new one
	return c.AddMRComment(projectID, mrIID, commentBody)
}
