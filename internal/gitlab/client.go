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
	defer resp.Body.Close()

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
	var title, author, sourceBranch, targetBranch string

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
	defer resp.Body.Close()

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
	defer resp.Body.Close()

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
