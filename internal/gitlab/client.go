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
func (c *Client) FetchMRChanges(projectID, mrIID int) (*MRChanges, error) {
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

	var changes MRChanges
	if err := json.NewDecoder(resp.Body).Decode(&changes); err != nil {
		return nil, err
	}

	return &changes, nil
}

// ExtractMRInfo extracts merge request information from webhook payload
func ExtractMRInfo(payload map[string]interface{}) (*MRInfo, error) {
	var projectID, mrIID int

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

	if projectID == 0 || mrIID == 0 {
		return nil, fmt.Errorf("missing project ID (%d) or MR IID (%d)", projectID, mrIID)
	}

	return &MRInfo{
		ProjectID: projectID,
		MRIID:     mrIID,
	}, nil
}