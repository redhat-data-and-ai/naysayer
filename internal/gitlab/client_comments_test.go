package gitlab

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/redhat-data-and-ai/naysayer/internal/config"
	"github.com/stretchr/testify/assert"
)

func TestNewClientWithConfig(t *testing.T) {
	cfg := &config.Config{
		GitLab: config.GitLabConfig{
			BaseURL: "https://gitlab.example.com",
			Token:   "test-token",
		},
	}

	client := NewClientWithConfig(cfg)

	assert.NotNil(t, client)
	assert.Equal(t, cfg.GitLab.BaseURL, client.config.BaseURL)
	assert.Equal(t, cfg.GitLab.Token, client.config.Token)
}

func TestAddMRComment_Success(t *testing.T) {
	// Create test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request
		assert.Equal(t, "POST", r.Method)
		assert.Contains(t, r.URL.Path, "/api/v4/projects/123/merge_requests/456/notes")
		assert.Equal(t, "Bearer test-token", r.Header.Get("Authorization"))
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))

		// Verify request body
		body, _ := io.ReadAll(r.Body)
		var payload map[string]string
		_ = json.Unmarshal(body, &payload)
		assert.Equal(t, "Test comment", payload["body"])

		// Return success response
		w.WriteHeader(201)
		_, _ = w.Write([]byte(`{"id": 123, "body": "Test comment"}`))
	}))
	defer server.Close()

	cfg := &config.Config{
		GitLab: config.GitLabConfig{
			BaseURL: server.URL,
			Token:   "test-token",
		},
	}

	client := NewClientWithConfig(cfg)

	err := client.AddMRComment(123, 456, "Test comment")

	assert.NoError(t, err)
}

func TestAddMRComment_UnauthorizedError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(401)
		_, _ = w.Write([]byte(`{"message": "401 Unauthorized"}`))
	}))
	defer server.Close()

	cfg := &config.Config{
		GitLab: config.GitLabConfig{
			BaseURL: server.URL,
			Token:   "invalid-token",
		},
	}

	client := NewClientWithConfig(cfg)

	err := client.AddMRComment(123, 456, "Test comment")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "comment failed: insufficient permissions")
}

func TestAddMRComment_NotFoundError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(404)
		_, _ = w.Write([]byte(`{"message": "404 Not Found"}`))
	}))
	defer server.Close()

	cfg := &config.Config{
		GitLab: config.GitLabConfig{
			BaseURL: server.URL,
			Token:   "test-token",
		},
	}

	client := NewClientWithConfig(cfg)

	err := client.AddMRComment(123, 456, "Test comment")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "comment failed: MR not found")
}

func TestAddMRComment_GenericError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
		_, _ = w.Write([]byte(`{"message": "Internal Server Error"}`))
	}))
	defer server.Close()

	cfg := &config.Config{
		GitLab: config.GitLabConfig{
			BaseURL: server.URL,
			Token:   "test-token",
		},
	}

	client := NewClientWithConfig(cfg)

	err := client.AddMRComment(123, 456, "Test comment")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "comment failed with status 500")
	assert.Contains(t, err.Error(), "Internal Server Error")
}

func TestApproveMR_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request
		assert.Equal(t, "POST", r.Method)
		assert.Contains(t, r.URL.Path, "/api/v4/projects/123/merge_requests/456/approve")
		assert.Equal(t, "Bearer test-token", r.Header.Get("Authorization"))

		// Verify empty body for simple approval
		body, _ := io.ReadAll(r.Body)
		assert.Equal(t, "{}", string(body))

		// Return success response
		w.WriteHeader(201)
		_, _ = w.Write([]byte(`{"id": 123, "approved": true}`))
	}))
	defer server.Close()

	cfg := &config.Config{
		GitLab: config.GitLabConfig{
			BaseURL: server.URL,
			Token:   "test-token",
		},
	}

	client := NewClientWithConfig(cfg)

	err := client.ApproveMR(123, 456)

	assert.NoError(t, err)
}

func TestApproveMRWithMessage_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request
		assert.Equal(t, "POST", r.Method)
		assert.Contains(t, r.URL.Path, "/api/v4/projects/123/merge_requests/456/approve")
		assert.Equal(t, "Bearer test-token", r.Header.Get("Authorization"))

		// Verify request body contains message
		body, _ := io.ReadAll(r.Body)
		var payload map[string]string
		_ = json.Unmarshal(body, &payload)
		assert.Equal(t, "Auto-approved: Safe changes", payload["note"])

		// Return success response
		w.WriteHeader(201)
		_, _ = w.Write([]byte(`{"id": 123, "approved": true}`))
	}))
	defer server.Close()

	cfg := &config.Config{
		GitLab: config.GitLabConfig{
			BaseURL: server.URL,
			Token:   "test-token",
		},
	}

	client := NewClientWithConfig(cfg)

	err := client.ApproveMRWithMessage(123, 456, "Auto-approved: Safe changes")

	assert.NoError(t, err)
}

func TestApproveMRWithMessage_EmptyMessage(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request body is empty JSON object for empty message
		body, _ := io.ReadAll(r.Body)
		assert.Equal(t, "{}", string(body))

		w.WriteHeader(201)
		_, _ = w.Write([]byte(`{"id": 123, "approved": true}`))
	}))
	defer server.Close()

	cfg := &config.Config{
		GitLab: config.GitLabConfig{
			BaseURL: server.URL,
			Token:   "test-token",
		},
	}

	client := NewClientWithConfig(cfg)

	err := client.ApproveMRWithMessage(123, 456, "")

	assert.NoError(t, err)
}

func TestApproveMRWithMessage_UnauthorizedError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(401)
		_, _ = w.Write([]byte(`{"message": "401 Unauthorized"}`))
	}))
	defer server.Close()

	cfg := &config.Config{
		GitLab: config.GitLabConfig{
			BaseURL: server.URL,
			Token:   "invalid-token",
		},
	}

	client := NewClientWithConfig(cfg)

	err := client.ApproveMRWithMessage(123, 456, "Test approval")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "approval failed: insufficient permissions")
}

func TestApproveMRWithMessage_NotFoundError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(404)
		_, _ = w.Write([]byte(`{"message": "404 Not Found"}`))
	}))
	defer server.Close()

	cfg := &config.Config{
		GitLab: config.GitLabConfig{
			BaseURL: server.URL,
			Token:   "test-token",
		},
	}

	client := NewClientWithConfig(cfg)

	err := client.ApproveMRWithMessage(123, 456, "Test approval")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "approval failed: MR not found")
}

func TestApproveMRWithMessage_AlreadyApprovedError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(405)
		_, _ = w.Write([]byte(`{"message": "Method Not Allowed"}`))
	}))
	defer server.Close()

	cfg := &config.Config{
		GitLab: config.GitLabConfig{
			BaseURL: server.URL,
			Token:   "test-token",
		},
	}

	client := NewClientWithConfig(cfg)

	err := client.ApproveMRWithMessage(123, 456, "Test approval")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "approval failed: MR already approved or cannot be approved")
}

func TestApproveMRWithMessage_GenericError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
		_, _ = w.Write([]byte(`{"message": "Internal Server Error"}`))
	}))
	defer server.Close()

	cfg := &config.Config{
		GitLab: config.GitLabConfig{
			BaseURL: server.URL,
			Token:   "test-token",
		},
	}

	client := NewClientWithConfig(cfg)

	err := client.ApproveMRWithMessage(123, 456, "Test approval")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "approval failed with status 500")
	assert.Contains(t, err.Error(), "Internal Server Error")
}

func TestAddMRComment_NetworkError(t *testing.T) {
	cfg := &config.Config{
		GitLab: config.GitLabConfig{
			BaseURL: "http://non-existent-server.invalid",
			Token:   "test-token",
		},
	}

	client := NewClientWithConfig(cfg)

	err := client.AddMRComment(123, 456, "Test comment")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to add comment")
}

func TestApproveMRWithMessage_NetworkError(t *testing.T) {
	cfg := &config.Config{
		GitLab: config.GitLabConfig{
			BaseURL: "http://non-existent-server.invalid",
			Token:   "test-token",
		},
	}

	client := NewClientWithConfig(cfg)

	err := client.ApproveMRWithMessage(123, 456, "Test approval")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to approve MR")
}

func TestApproveMR_CallsApproveMRWithMessage(t *testing.T) {
	// Test that ApproveMR correctly calls ApproveMRWithMessage with empty message
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify that empty message results in empty JSON object
		body, _ := io.ReadAll(r.Body)
		assert.Equal(t, "{}", string(body))

		w.WriteHeader(201)
		_, _ = w.Write([]byte(`{"id": 123, "approved": true}`))
	}))
	defer server.Close()

	cfg := &config.Config{
		GitLab: config.GitLabConfig{
			BaseURL: server.URL,
			Token:   "test-token",
		},
	}

	client := NewClientWithConfig(cfg)

	err := client.ApproveMR(123, 456)

	assert.NoError(t, err)
}

// TestListMRComments_Pagination tests basic pagination functionality
func TestListMRComments_Pagination(t *testing.T) {
	requestCount := 0
	var serverURL string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++

		if requestCount == 1 {
			// Page 1: Return 100 comments with Link header
			comments := make([]MRComment, 100)
			for i := 0; i < 100; i++ {
				comments[i] = MRComment{ID: i + 1, Body: fmt.Sprintf("Comment %d", i+1)}
			}
			w.Header().Set("Link", fmt.Sprintf(`<%s?page=2>; rel="next"`, serverURL))
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(comments)
		} else {
			// Page 2: Return 50 comments, no Link header
			comments := make([]MRComment, 50)
			for i := 0; i < 50; i++ {
				comments[i] = MRComment{ID: i + 101, Body: fmt.Sprintf("Comment %d", i+101)}
			}
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(comments)
		}
	}))
	defer server.Close()
	serverURL = server.URL

	client := NewClient(config.GitLabConfig{BaseURL: server.URL, Token: "test-token"})
	comments, err := client.ListMRComments(123, 456)

	assert.NoError(t, err)
	assert.Len(t, comments, 150)
	assert.Equal(t, 2, requestCount)
}
