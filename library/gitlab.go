package library

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

type GitLabClient struct {
	baseURL    string
	authToken  string
	httpClient *http.Client
}

// NOTE: We are not using gitlab.com/gitlab-org/api/client-go, because
// it requires gitlab authentication to install the package.

// NewGitLabClient initializes a new GitLab REST API client.
func NewGitLabClient(baseUrl string, token string) *GitLabClient {
	gitlabConfig := GitLabClient{}
	gitlabConfig.baseURL = baseUrl
	gitlabConfig.authToken = token
	gitlabConfig.httpClient = &http.Client{}
	return &gitlabConfig
}

// doRequest performs an HTTP request with the given method, endpoint, and body.
func (c *GitLabClient) doRequest(ctx context.Context, method, endpoint string, body any) ([]byte, error) {
	url := fmt.Sprintf("%s%s", c.baseURL, endpoint)

	var reqBody []byte
	var err error
	if body != nil {
		reqBody, err = json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal request body: %w", err)
		}
	}

	req, err := http.NewRequestWithContext(ctx, method, url, bytes.NewBuffer(reqBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.authToken))
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("request failed with status code %d", resp.StatusCode)
	}

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	return respBody, nil
}

// GetProject fetches details of a specific project by ID.
func (c *GitLabClient) GetProject(ctx context.Context, projectID int) (map[string]any, error) {
	endpoint := fmt.Sprintf("projects/%d", projectID)
	respBody, err := c.doRequest(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}

	var project map[string]any
	if err := json.Unmarshal(respBody, &project); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return project, nil
}

func (c *GitLabClient) AssignReviewers(ctx context.Context, projectID int, mergeRequestID int, reviewers []string) error {
	endpoint := fmt.Sprintf("projects/%d/merge_requests/%d", projectID, mergeRequestID)
	body := map[string]any{
		"assignee_ids": reviewers,
	}

	_, err := c.doRequest(ctx, http.MethodPut, endpoint, body)
	if err != nil {
		return fmt.Errorf("failed to assign reviewers: %w", err)
	}

	return nil
}
