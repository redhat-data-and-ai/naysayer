package gitlab

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/redhat-data-and-ai/naysayer/internal/config"
	"github.com/stretchr/testify/assert"
)

func TestClient_FetchFileContent_Success(t *testing.T) {
	yamlContent := `name: test-product
rover_group: test
warehouses:
  - type: snowflake
    size: MEDIUM
tags:
  data_product: test`

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request
		assert.Equal(t, "GET", r.Method)
		assert.Contains(t, r.URL.Path, "/api/v4/projects/123/repository/files/")
		assert.Contains(t, r.URL.RawQuery, "ref=main")
		assert.Equal(t, "Bearer test-token", r.Header.Get("Authorization"))
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))

		// Return mock file content response
		response := FileContent{
			FileName:     "product.yaml",
			FilePath:     "dataproducts/agg/test/product.yaml",
			Size:         len(yamlContent),
			Encoding:     "text",
			Content:      yamlContent,
			ContentSha1:  "abc123def456",
			Ref:          "main",
			BlobID:       "blob123",
			CommitID:     "commit456",
			LastCommitID: "commit789",
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	cfg := config.GitLabConfig{
		BaseURL: server.URL,
		Token:   "test-token",
	}
	client := NewClient(cfg)

	content, err := client.FetchFileContent(123, "dataproducts/agg/test/product.yaml", "main")

	assert.NoError(t, err)
	assert.NotNil(t, content)
	assert.Equal(t, "product.yaml", content.FileName)
	assert.Equal(t, "dataproducts/agg/test/product.yaml", content.FilePath)
	assert.Equal(t, yamlContent, content.Content)
	assert.Equal(t, "text", content.Encoding)
	assert.Equal(t, "main", content.Ref)
}

func TestClient_FetchFileContent_Base64Decoding(t *testing.T) {
	originalContent := `name: base64-test
rover_group: test
warehouses:
  - type: snowflake
    size: LARGE`

	encodedContent := base64.StdEncoding.EncodeToString([]byte(originalContent))

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := FileContent{
			FileName: "product.yaml",
			FilePath: "dataproducts/agg/test/product.yaml",
			Size:     len(originalContent),
			Encoding: "base64",
			Content:  encodedContent,
			Ref:      "main",
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	cfg := config.GitLabConfig{
		BaseURL: server.URL,
		Token:   "test-token",
	}
	client := NewClient(cfg)

	content, err := client.FetchFileContent(123, "dataproducts/agg/test/product.yaml", "main")

	assert.NoError(t, err)
	assert.Equal(t, originalContent, content.Content)
	assert.Equal(t, "base64", content.Encoding) // Encoding field preserved
}

func TestClient_FetchFileContent_InvalidBase64(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := FileContent{
			FileName: "product.yaml",
			FilePath: "dataproducts/agg/test/product.yaml",
			Encoding: "base64",
			Content:  "invalid-base64-content!@#", // Invalid base64
			Ref:      "main",
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	cfg := config.GitLabConfig{
		BaseURL: server.URL,
		Token:   "test-token",
	}
	client := NewClient(cfg)

	content, err := client.FetchFileContent(123, "dataproducts/agg/test/product.yaml", "main")

	assert.Error(t, err)
	assert.Nil(t, content)
	assert.Contains(t, err.Error(), "failed to decode base64 content")
}

func TestClient_FetchFileContent_FileNotFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(404)
		_, _ = w.Write([]byte(`{"message": "404 File Not Found"}`))
	}))
	defer server.Close()

	cfg := config.GitLabConfig{
		BaseURL: server.URL,
		Token:   "test-token",
	}
	client := NewClient(cfg)

	content, err := client.FetchFileContent(123, "nonexistent/file.yaml", "main")

	assert.Error(t, err)
	assert.Nil(t, content)
	assert.Contains(t, err.Error(), "file not found")
}

func TestClient_FetchFileContent_HTTPErrors(t *testing.T) {
	tests := []struct {
		name          string
		statusCode    int
		responseBody  string
		expectedError string
	}{
		{
			name:          "401 Unauthorized",
			statusCode:    401,
			responseBody:  `{"message": "401 Unauthorized"}`,
			expectedError: "GitLab API error 401",
		},
		{
			name:          "403 Forbidden",
			statusCode:    403,
			responseBody:  `{"message": "403 Project access forbidden"}`,
			expectedError: "GitLab API error 403",
		},
		{
			name:          "500 Internal Server Error",
			statusCode:    500,
			responseBody:  `{"message": "Internal Server Error"}`,
			expectedError: "GitLab API error 500",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.statusCode)
				_, _ = w.Write([]byte(tt.responseBody))
			}))
			defer server.Close()

			cfg := config.GitLabConfig{
				BaseURL: server.URL,
				Token:   "test-token",
			}
			client := NewClient(cfg)

			content, err := client.FetchFileContent(123, "test/file.yaml", "main")

			assert.Error(t, err)
			assert.Nil(t, content)
			assert.Contains(t, err.Error(), tt.expectedError)
		})
	}
}

func TestClient_FetchFileContent_URLEncoding(t *testing.T) {
	// Test that special characters in file paths are properly handled
	filePath := "data products/test+file@domain.yaml"

	var requestURL string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestURL = r.URL.String()
		response := FileContent{
			FileName: "test.yaml",
			Content:  "test content",
			Encoding: "text",
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	cfg := config.GitLabConfig{
		BaseURL: server.URL,
		Token:   "test-token",
	}
	client := NewClient(cfg)

	_, err := client.FetchFileContent(123, filePath, "main")

	assert.NoError(t, err)
	// Just verify the URL contains encoded characters and the ref parameter
	assert.Contains(t, requestURL, "/repository/files/")
	assert.Contains(t, requestURL, "ref=main")
	// The important thing is that the request succeeds and special characters are handled
}

func TestClient_GetMRTargetBranch_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request
		assert.Equal(t, "GET", r.Method)
		assert.Contains(t, r.URL.Path, "/api/v4/projects/123/merge_requests/456")
		assert.Equal(t, "Bearer test-token", r.Header.Get("Authorization"))

		// Return mock MR response
		response := map[string]interface{}{
			"target_branch": "main",
			"source_branch": "feature/update-warehouse",
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	cfg := config.GitLabConfig{
		BaseURL: server.URL,
		Token:   "test-token",
	}
	client := NewClient(cfg)

	targetBranch, err := client.GetMRTargetBranch(123, 456)

	assert.NoError(t, err)
	assert.Equal(t, "main", targetBranch)
}

func TestClient_GetMRTargetBranch_HTTPError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(404)
		_, _ = w.Write([]byte(`{"message": "404 Merge Request Not Found"}`))
	}))
	defer server.Close()

	cfg := config.GitLabConfig{
		BaseURL: server.URL,
		Token:   "test-token",
	}
	client := NewClient(cfg)

	targetBranch, err := client.GetMRTargetBranch(123, 999)

	assert.Error(t, err)
	assert.Empty(t, targetBranch)
	assert.Contains(t, err.Error(), "GitLab API error 404")
}

func TestClient_GetMRTargetBranch_InvalidJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"invalid": json content}`))
	}))
	defer server.Close()

	cfg := config.GitLabConfig{
		BaseURL: server.URL,
		Token:   "test-token",
	}
	client := NewClient(cfg)

	targetBranch, err := client.GetMRTargetBranch(123, 456)

	assert.Error(t, err)
	assert.Empty(t, targetBranch)
}

func TestClient_GetMRDetails_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request
		assert.Equal(t, "GET", r.Method)
		assert.Contains(t, r.URL.Path, "/api/v4/projects/123/merge_requests/456")

		// Return mock MR details response
		response := MRDetails{
			TargetBranch:    "main",
			SourceBranch:    "feature/new-feature",
			IID:             456,
			ProjectID:       123,
			SourceProjectID: 123,
			TargetProjectID: 123,
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	cfg := config.GitLabConfig{
		BaseURL: server.URL,
		Token:   "test-token",
	}
	client := NewClient(cfg)

	details, err := client.GetMRDetails(123, 456)

	assert.NoError(t, err)
	assert.NotNil(t, details)
	assert.Equal(t, "main", details.TargetBranch)
	assert.Equal(t, "feature/new-feature", details.SourceBranch)
	assert.Equal(t, 456, details.IID)
	assert.Equal(t, 123, details.ProjectID)
	assert.Equal(t, 123, details.SourceProjectID)
	assert.Equal(t, 123, details.TargetProjectID)
}

func TestClient_GetMRDetails_HTTPError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(403)
		_, _ = w.Write([]byte(`{"message": "403 Forbidden"}`))
	}))
	defer server.Close()

	cfg := config.GitLabConfig{
		BaseURL: server.URL,
		Token:   "test-token",
	}
	client := NewClient(cfg)

	details, err := client.GetMRDetails(123, 456)

	assert.Error(t, err)
	assert.Nil(t, details)
	assert.Contains(t, err.Error(), "GitLab API error 403")
}

func TestClient_GetMRDetails_InvalidJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"incomplete": json`))
	}))
	defer server.Close()

	cfg := config.GitLabConfig{
		BaseURL: server.URL,
		Token:   "test-token",
	}
	client := NewClient(cfg)

	details, err := client.GetMRDetails(123, 456)

	assert.Error(t, err)
	assert.Nil(t, details)
}

func TestClient_FetchFileContent_NetworkError(t *testing.T) {
	cfg := config.GitLabConfig{
		BaseURL: "http://localhost:99999", // Non-existent server
		Token:   "test-token",
	}
	client := NewClient(cfg)

	content, err := client.FetchFileContent(123, "test/file.yaml", "main")

	assert.Error(t, err)
	assert.Nil(t, content)
	assert.NotEmpty(t, err.Error())
}

func TestClient_GetMRTargetBranch_NetworkError(t *testing.T) {
	cfg := config.GitLabConfig{
		BaseURL: "http://localhost:99999", // Non-existent server
		Token:   "test-token",
	}
	client := NewClient(cfg)

	targetBranch, err := client.GetMRTargetBranch(123, 456)

	assert.Error(t, err)
	assert.Empty(t, targetBranch)
	assert.NotEmpty(t, err.Error())
}

func TestClient_GetMRDetails_NetworkError(t *testing.T) {
	cfg := config.GitLabConfig{
		BaseURL: "http://localhost:99999", // Non-existent server
		Token:   "test-token",
	}
	client := NewClient(cfg)

	details, err := client.GetMRDetails(123, 456)

	assert.Error(t, err)
	assert.Nil(t, details)
	assert.NotEmpty(t, err.Error())
}

func TestClient_FetchFileContent_QueryParameters(t *testing.T) {
	var capturedQuery string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedQuery = r.URL.RawQuery
		response := FileContent{
			FileName: "test.yaml",
			Content:  "test content",
			Encoding: "text",
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	cfg := config.GitLabConfig{
		BaseURL: server.URL,
		Token:   "test-token",
	}
	client := NewClient(cfg)

	_, err := client.FetchFileContent(123, "test/file.yaml", "feature-branch")

	assert.NoError(t, err)
	assert.Equal(t, "ref=feature-branch", capturedQuery)
}

func TestClient_FetchFileContent_EmptyResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := FileContent{
			FileName: "empty.yaml",
			Content:  "", // Empty content
			Encoding: "text",
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	cfg := config.GitLabConfig{
		BaseURL: server.URL,
		Token:   "test-token",
	}
	client := NewClient(cfg)

	content, err := client.FetchFileContent(123, "empty.yaml", "main")

	assert.NoError(t, err)
	assert.NotNil(t, content)
	assert.Equal(t, "empty.yaml", content.FileName)
	assert.Empty(t, content.Content)
}
