package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

// Configuration for testing
type TestConfig struct {
	NaysayerURL   string // URL where your naysayer service is running
	GitLabBaseURL string // GitLab instance URL
	GitLabToken   string // GitLab API token for fetching real MR data
	WriteComments bool   // Whether to write test results as MR comments
}

// RealMRTestClient helps test with real GitLab MR data
type RealMRTestClient struct {
	config TestConfig
	client *http.Client
}

// NewRealMRTestClient creates a new test client
func NewRealMRTestClient() *RealMRTestClient {
	config := TestConfig{
		NaysayerURL:   getEnv("NAYSAYER_URL", "http://localhost:3000"),
		GitLabBaseURL: getEnv("GITLAB_BASE_URL", "https://gitlab.cee.redhat.com"),
		GitLabToken:   getEnv("GITLAB_TOKEN", ""),
		WriteComments: getEnv("WRITE_COMMENTS", "true") == "true",
	}

	if config.GitLabToken == "" {
		log.Fatal("GITLAB_TOKEN environment variable is required")
	}

	return &RealMRTestClient{
		config: config,
		client: &http.Client{Timeout: 30 * time.Second},
	}
}

// FetchProjectID fetches project ID from GitLab API using project path
func (c *RealMRTestClient) FetchProjectID(projectPath string) (int, error) {
	// URL encode the project path
	encodedPath := strings.ReplaceAll(projectPath, "/", "%2F")
	url := fmt.Sprintf("%s/api/v4/projects/%s",
		strings.TrimRight(c.config.GitLabBaseURL, "/"), encodedPath)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return 0, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+c.config.GitLabToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return 0, fmt.Errorf("failed to fetch project data: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return 0, fmt.Errorf("GitLab API returned status %d for project %s", resp.StatusCode, projectPath)
	}

	var projectData map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&projectData); err != nil {
		return 0, fmt.Errorf("failed to decode project data: %w", err)
	}

	// Extract project ID
	if id, ok := projectData["id"]; ok {
		switch v := id.(type) {
		case float64:
			return int(v), nil
		case int:
			return v, nil
		default:
			return 0, fmt.Errorf("unexpected project ID type: %T", v)
		}
	}

	return 0, fmt.Errorf("project ID not found in response")
}

// FetchRealMRData fetches real MR data from GitLab API
func (c *RealMRTestClient) FetchRealMRData(projectID int, mrIID int) (map[string]interface{}, error) {
	url := fmt.Sprintf("%s/api/v4/projects/%d/merge_requests/%d",
		strings.TrimRight(c.config.GitLabBaseURL, "/"), projectID, mrIID)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+c.config.GitLabToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch MR data: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("GitLab API returned status %d", resp.StatusCode)
	}

	var mrData map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&mrData); err != nil {
		return nil, fmt.Errorf("failed to decode MR data: %w", err)
	}

	return mrData, nil
}

// CreateWebhookPayload creates a webhook payload from real MR data
func (c *RealMRTestClient) CreateWebhookPayload(mrData map[string]interface{}) map[string]interface{} {
	// Extract necessary fields from real MR data
	payload := map[string]interface{}{
		"object_kind": "merge_request",
		"event_type":  "merge_request",
		"object_attributes": map[string]interface{}{
			"iid":           mrData["iid"],
			"title":         mrData["title"],
			"description":   mrData["description"],
			"state":         mrData["state"],
			"source_branch": mrData["source_branch"],
			"target_branch": mrData["target_branch"],
			"action":        "open", // You can change this to test different actions
		},
		"project": map[string]interface{}{
			"id":                  mrData["project_id"],
			"name":                "dataproduct-config", // Default for dataverse config
			"path_with_namespace": "dataverse/dataverse-config/dataproduct-config",
		},
		"user": map[string]interface{}{
			"username": extractNestedValue(mrData, "author.username", "test-user"),
			"name":     extractNestedValue(mrData, "author.name", "Test User"),
		},
	}

	return payload
}

// TestRealMR tests the naysayer endpoint with a real MR
func (c *RealMRTestClient) TestRealMR(projectID int, mrIID int) error {
	fmt.Printf("🔍 Fetching real MR data for project %d, MR !%d...\n", projectID, mrIID)

	// Fetch real MR data from GitLab
	mrData, err := c.FetchRealMRData(projectID, mrIID)
	if err != nil {
		return fmt.Errorf("failed to fetch MR data: %w", err)
	}

	fmt.Printf("✅ Successfully fetched MR data: %s\n", extractNestedValue(mrData, "title", "Unknown Title"))

	// Create webhook payload
	payload := c.CreateWebhookPayload(mrData)

	// Send to naysayer endpoint
	return c.sendWebhookPayload(payload, projectID, mrIID)
}

// TestRealMRFromPath tests the naysayer endpoint with a real MR using project path
func (c *RealMRTestClient) TestRealMRFromPath(projectPath string, mrIID int) error {
	fmt.Printf("🔍 Fetching project ID for %s...\n", projectPath)

	// First, fetch the project ID
	projectID, err := c.FetchProjectID(projectPath)
	if err != nil {
		return fmt.Errorf("failed to fetch project ID: %w", err)
	}

	fmt.Printf("✅ Found project ID: %d\n", projectID)

	// Now test the MR
	return c.TestRealMR(projectID, mrIID)
}

// sendWebhookPayload sends the webhook payload to naysayer and optionally writes response to MR
func (c *RealMRTestClient) sendWebhookPayload(payload map[string]interface{}, projectID, mrIID int) error {
	fmt.Printf("📤 Sending webhook payload to naysayer...\n")

	jsonPayload, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal payload: %w", err)
	}

	// Print payload for debugging
	fmt.Printf("📄 Webhook payload:\n%s\n", string(jsonPayload))

	url := c.config.NaysayerURL + "/dataverse-product-config-review"
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonPayload))
	if err != nil {
		return fmt.Errorf("failed to create webhook request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Gitlab-Event", "Merge Request Hook")

	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send webhook: %w", err)
	}
	defer resp.Body.Close()

	fmt.Printf("📥 Naysayer response status: %d\n", resp.StatusCode)

	// Read and print response
	var respBody bytes.Buffer
	if _, err := respBody.ReadFrom(resp.Body); err != nil {
		return fmt.Errorf("failed to read response: %w", err)
	}

	responseText := respBody.String()
	fmt.Printf("📄 Naysayer response:\n%s\n", responseText)

	if resp.StatusCode >= 400 {
		return fmt.Errorf("naysayer returned error status %d", resp.StatusCode)
	}

	fmt.Printf("✅ Successfully tested MR project %d, MR !%d\n", projectID, mrIID)
	return nil
}

// extractNestedValue safely extracts nested values from maps
func extractNestedValue(data map[string]interface{}, path string, defaultValue string) string {
	parts := strings.Split(path, ".")
	current := data

	for i, part := range parts {
		if i == len(parts)-1 {
			// Last part - get the actual value
			if val, ok := current[part]; ok {
				if str, ok := val.(string); ok {
					return str
				}
			}
			return defaultValue
		} else {
			// Intermediate part - navigate deeper
			if val, ok := current[part]; ok {
				if nested, ok := val.(map[string]interface{}); ok {
					current = nested
				} else {
					return defaultValue
				}
			} else {
				return defaultValue
			}
		}
	}

	return defaultValue
}

// parseMRURL parses a GitLab MR URL to extract project path and MR IID
func parseMRURL(mrURL string) (string, int, error) {
	// Example: https://gitlab.cee.redhat.com/dataverse/dataverse-config/dataproduct-config/-/merge_requests/1764
	parts := strings.Split(mrURL, "/")

	// Find merge_requests in the URL
	mrIndex := -1
	for i, part := range parts {
		if part == "merge_requests" {
			mrIndex = i
			break
		}
	}

	if mrIndex == -1 || mrIndex+1 >= len(parts) {
		return "", 0, fmt.Errorf("invalid MR URL format")
	}

	// Get MR IID
	mrIID, err := strconv.Atoi(parts[mrIndex+1])
	if err != nil {
		return "", 0, fmt.Errorf("invalid MR IID: %w", err)
	}

	// Extract project path from URL
	// The URL structure is: https://domain/group/subgroup/project/-/merge_requests/123
	// We need to find everything between the domain and "/-/"

	// Find the domain (parts[2] after splitting by /)
	if len(parts) < 3 {
		return "", 0, fmt.Errorf("invalid URL format: too short")
	}

	// Find the "/-/" marker
	dashIndex := -1
	for i := 3; i < len(parts); i++ { // Start after protocol and domain
		if parts[i] == "-" {
			dashIndex = i
			break
		}
	}

	if dashIndex == -1 {
		return "", 0, fmt.Errorf("invalid URL format: no /-/ marker found")
	}

	// Project path is from index 3 (after domain) to dashIndex
	pathParts := parts[3:dashIndex]
	if len(pathParts) == 0 {
		return "", 0, fmt.Errorf("invalid URL format: no project path found")
	}

	projectPath := strings.Join(pathParts, "/")

	return projectPath, mrIID, nil
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func main() {
	fmt.Println("🚀 Naysayer Real MR Testing Tool")
	fmt.Println("=====================================")

	if len(os.Args) < 2 {
		fmt.Println("Usage:")
		fmt.Println("  go run test_real_mr.go <project_id> <mr_iid>")
		fmt.Println("  go run test_real_mr.go <gitlab_mr_url>")
		fmt.Println("")
		fmt.Println("Examples:")
		fmt.Println("  go run test_real_mr.go 51 1764")
		fmt.Println("  go run test_real_mr.go https://gitlab.cee.redhat.com/dataverse/dataverse-config/dataproduct-config/-/merge_requests/1764")
		fmt.Println("")
		fmt.Println("Environment variables:")
		fmt.Println("  GITLAB_TOKEN   - GitLab API token (required)")
		fmt.Println("  NAYSAYER_URL   - Naysayer service URL (default: http://localhost:3000)")
		fmt.Println("  GITLAB_BASE_URL - GitLab base URL (default: https://gitlab.cee.redhat.com)")
		fmt.Println("  WRITE_COMMENTS - Write test results as MR comments (default: true)")
		os.Exit(1)
	}

	client := NewRealMRTestClient()

	fmt.Printf("🔗 Naysayer URL: %s\n", client.config.NaysayerURL)
	fmt.Printf("🔗 GitLab URL: %s\n", client.config.GitLabBaseURL)
	fmt.Println("")

	if len(os.Args) == 2 {
		// Single argument - assume it's a URL
		mrURL := os.Args[1]
		projectPath, mrIID, err := parseMRURL(mrURL)
		if err != nil {
			log.Fatalf("Failed to parse MR URL: %v", err)
		}

		fmt.Printf("🎯 Testing project %s, MR !%d\n", projectPath, mrIID)

		if err := client.TestRealMRFromPath(projectPath, mrIID); err != nil {
			log.Fatalf("❌ Test failed: %v", err)
		}
	} else if len(os.Args) == 3 {
		// Two arguments - project ID and MR IID
		projectID, err := strconv.Atoi(os.Args[1])
		if err != nil {
			log.Fatalf("Invalid project ID: %v", err)
		}

		mrIID, err := strconv.Atoi(os.Args[2])
		if err != nil {
			log.Fatalf("Invalid MR IID: %v", err)
		}

		fmt.Printf("🎯 Testing project %d, MR !%d\n", projectID, mrIID)

		if err := client.TestRealMR(projectID, mrIID); err != nil {
			log.Fatalf("❌ Test failed: %v", err)
		}
	} else {
		log.Fatal("Invalid number of arguments")
	}

	fmt.Println("🎉 Test completed successfully!")
}
