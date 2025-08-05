package gitlab

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/redhat-data-and-ai/naysayer/internal/config"
)

// Client handles GitLab API operations
type Client struct {
	config config.GitLabConfig
	http   *http.Client
}

// NewClient creates a new GitLab API client
func NewClient(cfg config.GitLabConfig) *Client {
	return &Client{
		config: cfg,
		http:   &http.Client{},
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