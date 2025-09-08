package main

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"time"
)

// GitLabWebhookPayload represents a GitLab MR webhook payload
type GitLabWebhookPayload struct {
	ObjectKind string `json:"object_kind"`
	EventType  string `json:"event_type"`
	User       struct {
		ID       int    `json:"id"`
		Name     string `json:"name"`
		Username string `json:"username"`
		Email    string `json:"email"`
	} `json:"user"`
	Project struct {
		ID                int    `json:"id"`
		Name              string `json:"name"`
		Description       string `json:"description"`
		WebURL            string `json:"web_url"`
		AvatarURL         string `json:"avatar_url"`
		GitSSHURL         string `json:"git_ssh_url"`
		GitHTTPURL        string `json:"git_http_url"`
		Namespace         string `json:"namespace"`
		VisibilityLevel   int    `json:"visibility_level"`
		PathWithNamespace string `json:"path_with_namespace"`
		DefaultBranch     string `json:"default_branch"`
		Homepage          string `json:"homepage"`
		URL               string `json:"url"`
		SSHURL            string `json:"ssh_url"`
		HTTPURL           string `json:"http_url"`
	} `json:"project"`
	ObjectAttributes struct {
		AssigneeID     int    `json:"assignee_id"`
		AuthorID       int    `json:"author_id"`
		CreatedAt      string `json:"created_at"`
		Description    string `json:"description"`
		HeadPipelineID int    `json:"head_pipeline_id"`
		ID             int    `json:"id"`
		IID            int    `json:"iid"`
		LastEditedAt   string `json:"last_edited_at"`
		LastEditedByID int    `json:"last_edited_by_id"`
		MergeCommitSHA string `json:"merge_commit_sha"`
		MergeError     string `json:"merge_error"`
		MergeParams    struct {
			ForceRemoveSourceBranch string `json:"force_remove_source_branch"`
		} `json:"merge_params"`
		MergeStatus               string `json:"merge_status"`
		MergeUserID               int    `json:"merge_user_id"`
		MergeWhenPipelineSucceeds bool   `json:"merge_when_pipeline_succeeds"`
		MilestoneID               int    `json:"milestone_id"`
		SourceBranch              string `json:"source_branch"`
		SourceProjectID           int    `json:"source_project_id"`
		State                     string `json:"state"`
		TargetBranch              string `json:"target_branch"`
		TargetProjectID           int    `json:"target_project_id"`
		TimeEstimate              int    `json:"time_estimate"`
		Title                     string `json:"title"`
		UpdatedAt                 string `json:"updated_at"`
		UpdatedByID               int    `json:"updated_by_id"`
		URL                       string `json:"url"`
		Source                    struct {
			ID                int    `json:"id"`
			Name              string `json:"name"`
			Description       string `json:"description"`
			WebURL            string `json:"web_url"`
			AvatarURL         string `json:"avatar_url"`
			GitSSHURL         string `json:"git_ssh_url"`
			GitHTTPURL        string `json:"git_http_url"`
			Namespace         string `json:"namespace"`
			VisibilityLevel   int    `json:"visibility_level"`
			PathWithNamespace string `json:"path_with_namespace"`
			DefaultBranch     string `json:"default_branch"`
			Homepage          string `json:"homepage"`
			URL               string `json:"url"`
			SSHURL            string `json:"ssh_url"`
			HTTPURL           string `json:"http_url"`
		} `json:"source"`
		Target struct {
			ID                int    `json:"id"`
			Name              string `json:"name"`
			Description       string `json:"description"`
			WebURL            string `json:"web_url"`
			AvatarURL         string `json:"avatar_url"`
			GitSSHURL         string `json:"git_ssh_url"`
			GitHTTPURL        string `json:"git_http_url"`
			Namespace         string `json:"namespace"`
			VisibilityLevel   int    `json:"visibility_level"`
			PathWithNamespace string `json:"path_with_namespace"`
			DefaultBranch     string `json:"default_branch"`
			Homepage          string `json:"homepage"`
			URL               string `json:"url"`
			SSHURL            string `json:"ssh_url"`
			HTTPURL           string `json:"http_url"`
		} `json:"target"`
		LastCommit struct {
			ID        string `json:"id"`
			Message   string `json:"message"`
			Timestamp string `json:"timestamp"`
			URL       string `json:"url"`
			Author    struct {
				Name  string `json:"name"`
				Email string `json:"email"`
			} `json:"author"`
		} `json:"last_commit"`
		WorkInProgress bool   `json:"work_in_progress"`
		Action         string `json:"action"`
	} `json:"object_attributes"`
}

// Configuration for the webhook test
type WebhookTestConfig struct {
	NaysayerURL   string
	WebhookSecret string
	ProjectID     int
	MRIID         int
	EventType     string
	Action        string
}

func main() {
	if len(os.Args) < 2 {
		fmt.Printf("🔧 GitLab Webhook Test Script\n\n")
		fmt.Printf("Usage: %s <mr_iid> [action] [naysayer_url]\n", os.Args[0])
		fmt.Printf("Examples:\n")
		fmt.Printf("  %s 1764                    # Test MR 1764 with 'open' action\n", os.Args[0])
		fmt.Printf("  %s 1764 update             # Test MR 1764 with 'update' action\n", os.Args[0])
		fmt.Printf("  %s 1764 open http://localhost:3001  # Custom naysayer URL\n", os.Args[0])
		fmt.Printf("\nActions: open, update, close, reopen, merge\n")
		return
	}

	// Parse command line arguments
	mrIID, err := strconv.Atoi(os.Args[1])
	if err != nil {
		fmt.Printf("❌ Invalid MR IID: %s\n", os.Args[1])
		return
	}

	action := "open"
	if len(os.Args) > 2 {
		action = os.Args[2]
	}

	naysayerURL := "http://localhost:3001"
	if len(os.Args) > 3 {
		naysayerURL = os.Args[3]
	}

	config := WebhookTestConfig{
		NaysayerURL:   naysayerURL,
		WebhookSecret: "gR32t62UfsbmbTJ", // From secrets.yaml
		ProjectID:     106670,            // Found from earlier tests
		MRIID:         mrIID,
		EventType:     "merge_request",
		Action:        action,
	}

	fmt.Printf("🚀 Testing Naysayer Webhook\n\n")
	fmt.Printf("📋 Configuration:\n")
	fmt.Printf("   🔗 Naysayer URL: %s\n", config.NaysayerURL)
	fmt.Printf("   🆔 Project ID: %d\n", config.ProjectID)
	fmt.Printf("   📄 MR IID: %d\n", config.MRIID)
	fmt.Printf("   ⚡ Action: %s\n", config.Action)
	fmt.Printf("   🔐 Webhook Secret: %s...\n\n", config.WebhookSecret[:8])

	// Test the webhook
	if err := testWebhook(config); err != nil {
		fmt.Printf("❌ Webhook test failed: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("✅ Webhook test completed successfully!\n")
	fmt.Printf("🔗 Check MR: https://gitlab.cee.redhat.com/dataverse/dataverse-config/dataproduct-config/-/merge_requests/%d\n", config.MRIID)
}

func testWebhook(config WebhookTestConfig) error {
	// Create the webhook payload
	payload := createWebhookPayload(config)

	// Marshal to JSON
	jsonPayload, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal webhook payload: %w", err)
	}

	// Create HTTP request
	url := fmt.Sprintf("%s/dataverse-product-config-review", config.NaysayerURL)
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonPayload))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	// Add GitLab webhook headers
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "GitLab/16.0.0")
	req.Header.Set("X-Gitlab-Event", "Merge Request Hook")
	req.Header.Set("X-Gitlab-Event-UUID", fmt.Sprintf("test-uuid-%d", time.Now().Unix()))

	// Add webhook signature if secret is provided
	if config.WebhookSecret != "" {
		signature := generateWebhookSignature(jsonPayload, config.WebhookSecret)
		req.Header.Set("X-Gitlab-Token", signature)
	}

	fmt.Printf("📡 Sending webhook to %s...\n", url)

	// Send the request
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send webhook: %w", err)
	}
	defer resp.Body.Close()

	// Read response
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response: %w", err)
	}

	fmt.Printf("📨 Response Status: %s\n", resp.Status)
	fmt.Printf("📄 Response Body: %s\n", string(body))

	if resp.StatusCode != 200 {
		return fmt.Errorf("webhook returned status %d: %s", resp.StatusCode, string(body))
	}

	return nil
}

func createWebhookPayload(config WebhookTestConfig) GitLabWebhookPayload {
	now := time.Now().Format(time.RFC3339)

	return GitLabWebhookPayload{
		ObjectKind: "merge_request",
		EventType:  config.EventType,
		User: struct {
			ID       int    `json:"id"`
			Name     string `json:"name"`
			Username string `json:"username"`
			Email    string `json:"email"`
		}{
			ID:       12345,
			Name:     "Test User",
			Username: "testuser",
			Email:    "test@redhat.com",
		},
		Project: struct {
			ID                int    `json:"id"`
			Name              string `json:"name"`
			Description       string `json:"description"`
			WebURL            string `json:"web_url"`
			AvatarURL         string `json:"avatar_url"`
			GitSSHURL         string `json:"git_ssh_url"`
			GitHTTPURL        string `json:"git_http_url"`
			Namespace         string `json:"namespace"`
			VisibilityLevel   int    `json:"visibility_level"`
			PathWithNamespace string `json:"path_with_namespace"`
			DefaultBranch     string `json:"default_branch"`
			Homepage          string `json:"homepage"`
			URL               string `json:"url"`
			SSHURL            string `json:"ssh_url"`
			HTTPURL           string `json:"http_url"`
		}{
			ID:                config.ProjectID,
			Name:              "dataproduct-config",
			Description:       "Data product configuration repository",
			WebURL:            "https://gitlab.cee.redhat.com/dataverse/dataverse-config/dataproduct-config",
			PathWithNamespace: "dataverse/dataverse-config/dataproduct-config",
			DefaultBranch:     "main",
		},
		ObjectAttributes: struct {
			AssigneeID     int    `json:"assignee_id"`
			AuthorID       int    `json:"author_id"`
			CreatedAt      string `json:"created_at"`
			Description    string `json:"description"`
			HeadPipelineID int    `json:"head_pipeline_id"`
			ID             int    `json:"id"`
			IID            int    `json:"iid"`
			LastEditedAt   string `json:"last_edited_at"`
			LastEditedByID int    `json:"last_edited_by_id"`
			MergeCommitSHA string `json:"merge_commit_sha"`
			MergeError     string `json:"merge_error"`
			MergeParams    struct {
				ForceRemoveSourceBranch string `json:"force_remove_source_branch"`
			} `json:"merge_params"`
			MergeStatus               string `json:"merge_status"`
			MergeUserID               int    `json:"merge_user_id"`
			MergeWhenPipelineSucceeds bool   `json:"merge_when_pipeline_succeeds"`
			MilestoneID               int    `json:"milestone_id"`
			SourceBranch              string `json:"source_branch"`
			SourceProjectID           int    `json:"source_project_id"`
			State                     string `json:"state"`
			TargetBranch              string `json:"target_branch"`
			TargetProjectID           int    `json:"target_project_id"`
			TimeEstimate              int    `json:"time_estimate"`
			Title                     string `json:"title"`
			UpdatedAt                 string `json:"updated_at"`
			UpdatedByID               int    `json:"updated_by_id"`
			URL                       string `json:"url"`
			Source                    struct {
				ID                int    `json:"id"`
				Name              string `json:"name"`
				Description       string `json:"description"`
				WebURL            string `json:"web_url"`
				AvatarURL         string `json:"avatar_url"`
				GitSSHURL         string `json:"git_ssh_url"`
				GitHTTPURL        string `json:"git_http_url"`
				Namespace         string `json:"namespace"`
				VisibilityLevel   int    `json:"visibility_level"`
				PathWithNamespace string `json:"path_with_namespace"`
				DefaultBranch     string `json:"default_branch"`
				Homepage          string `json:"homepage"`
				URL               string `json:"url"`
				SSHURL            string `json:"ssh_url"`
				HTTPURL           string `json:"http_url"`
			} `json:"source"`
			Target struct {
				ID                int    `json:"id"`
				Name              string `json:"name"`
				Description       string `json:"description"`
				WebURL            string `json:"web_url"`
				AvatarURL         string `json:"avatar_url"`
				GitSSHURL         string `json:"git_ssh_url"`
				GitHTTPURL        string `json:"git_http_url"`
				Namespace         string `json:"namespace"`
				VisibilityLevel   int    `json:"visibility_level"`
				PathWithNamespace string `json:"path_with_namespace"`
				DefaultBranch     string `json:"default_branch"`
				Homepage          string `json:"homepage"`
				URL               string `json:"url"`
				SSHURL            string `json:"ssh_url"`
				HTTPURL           string `json:"http_url"`
			} `json:"target"`
			LastCommit struct {
				ID        string `json:"id"`
				Message   string `json:"message"`
				Timestamp string `json:"timestamp"`
				URL       string `json:"url"`
				Author    struct {
					Name  string `json:"name"`
					Email string `json:"email"`
				} `json:"author"`
			} `json:"last_commit"`
			WorkInProgress bool   `json:"work_in_progress"`
			Action         string `json:"action"`
		}{
			ID:              123456,
			IID:             config.MRIID,
			Title:           fmt.Sprintf("Test MR %d - Webhook Test", config.MRIID),
			Description:     "This is a test MR for webhook testing",
			State:           "opened",
			CreatedAt:       now,
			UpdatedAt:       now,
			TargetBranch:    "main",
			SourceBranch:    "test-naysayer",
			TargetProjectID: config.ProjectID,
			SourceProjectID: config.ProjectID,
			AuthorID:        12345,
			Action:          config.Action,
			URL:             fmt.Sprintf("https://gitlab.cee.redhat.com/dataverse/dataverse-config/dataproduct-config/-/merge_requests/%d", config.MRIID),
		},
	}
}

func generateWebhookSignature(payload []byte, secret string) string {
	h := hmac.New(sha256.New, []byte(secret))
	h.Write(payload)
	return hex.EncodeToString(h.Sum(nil))
}
