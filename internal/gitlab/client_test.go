package gitlab

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/redhat-data-and-ai/naysayer/internal/config"
	"github.com/stretchr/testify/assert"
)

func TestNewClient(t *testing.T) {
	cfg := config.GitLabConfig{
		BaseURL: "https://gitlab.example.com",
		Token:   "test-token",
	}

	client := NewClient(cfg)

	assert.NotNil(t, client)
	assert.Equal(t, cfg.BaseURL, client.config.BaseURL)
	assert.Equal(t, cfg.Token, client.config.Token)
	assert.NotNil(t, client.http)
}

func TestClient_FetchMRChanges_Success(t *testing.T) {
	// Create test server that returns mock GitLab response
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request
		assert.Equal(t, "GET", r.Method)
		assert.Contains(t, r.URL.Path, "/api/v4/projects/123/merge_requests/456/changes")
		assert.Equal(t, "Bearer test-token", r.Header.Get("Authorization"))
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))

		// Return mock response
		mockResponse := MRChanges{
			Changes: []struct {
				OldPath     string `json:"old_path"`
				NewPath     string `json:"new_path"`
				AMode       string `json:"a_mode"`
				BMode       string `json:"b_mode"`
				NewFile     bool   `json:"new_file"`
				RenamedFile bool   `json:"renamed_file"`
				DeletedFile bool   `json:"deleted_file"`
				Diff        string `json:"diff"`
			}{
				{
					OldPath:     "dataproducts/agg/test/product.yaml",
					NewPath:     "dataproducts/agg/test/product.yaml",
					AMode:       "100644",
					BMode:       "100644",
					NewFile:     false,
					RenamedFile: false,
					DeletedFile: false,
					Diff:        "@@ -5,7 +5,7 @@ warehouses:\n-    size: MEDIUM\n+    size: LARGE",
				},
				{
					OldPath:     "",
					NewPath:     "dataproducts/source/new/sourcebinding.yaml",
					AMode:       "000000",
					BMode:       "100644",
					NewFile:     true,
					RenamedFile: false,
					DeletedFile: false,
					Diff:        "@@ -0,0 +1,5 @@\n+kind: SourceBinding\n+consumers:\n+- test",
				},
			},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(mockResponse)
	}))
	defer server.Close()

	cfg := config.GitLabConfig{
		BaseURL: server.URL,
		Token:   "test-token",
	}
	client := NewClient(cfg)

	changes, err := client.FetchMRChanges(123, 456)

	assert.NoError(t, err)
	assert.Len(t, changes, 2)

	// Verify first change
	assert.Equal(t, "dataproducts/agg/test/product.yaml", changes[0].OldPath)
	assert.Equal(t, "dataproducts/agg/test/product.yaml", changes[0].NewPath)
	assert.False(t, changes[0].NewFile)
	assert.False(t, changes[0].DeletedFile)
	assert.Contains(t, changes[0].Diff, "size: LARGE")

	// Verify second change (new file)
	assert.Equal(t, "", changes[1].OldPath)
	assert.Equal(t, "dataproducts/source/new/sourcebinding.yaml", changes[1].NewPath)
	assert.True(t, changes[1].NewFile)
	assert.False(t, changes[1].DeletedFile)
}

func TestClient_FetchMRChanges_HTTPErrors(t *testing.T) {
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
			name:          "404 Not Found",
			statusCode:    404,
			responseBody:  `{"message": "404 Project Not Found"}`,
			expectedError: "GitLab API error 404",
		},
		{
			name:          "500 Internal Server Error",
			statusCode:    500,
			responseBody:  `{"message": "Internal Server Error"}`,
			expectedError: "GitLab API error 500",
		},
		{
			name:          "403 Forbidden",
			statusCode:    403,
			responseBody:  `{"message": "Forbidden"}`,
			expectedError: "GitLab API error 403",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.statusCode)
				w.Write([]byte(tt.responseBody))
			}))
			defer server.Close()

			cfg := config.GitLabConfig{
				BaseURL: server.URL,
				Token:   "test-token",
			}
			client := NewClient(cfg)

			changes, err := client.FetchMRChanges(123, 456)

			assert.Error(t, err)
			assert.Nil(t, changes)
			assert.Contains(t, err.Error(), tt.expectedError)
		})
	}
}

func TestClient_FetchMRChanges_InvalidJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"invalid": json}`)) // Invalid JSON
	}))
	defer server.Close()

	cfg := config.GitLabConfig{
		BaseURL: server.URL,
		Token:   "test-token",
	}
	client := NewClient(cfg)

	changes, err := client.FetchMRChanges(123, 456)

	assert.Error(t, err)
	assert.Nil(t, changes)
	assert.Contains(t, err.Error(), "invalid character")
}

func TestClient_FetchMRChanges_NetworkError(t *testing.T) {
	cfg := config.GitLabConfig{
		BaseURL: "http://localhost:99999", // Non-existent server
		Token:   "test-token",
	}
	client := NewClient(cfg)

	changes, err := client.FetchMRChanges(123, 456)

	assert.Error(t, err)
	assert.Nil(t, changes)
	// Just verify we get a network-related error
	assert.NotEmpty(t, err.Error())
}

func TestClient_FetchMRChanges_URLConstruction(t *testing.T) {
	tests := []struct {
		name        string
		baseURL     string
		projectID   int
		mrIID       int
		expectedURL string
	}{
		{
			name:        "standard URL",
			baseURL:     "https://gitlab.com",
			projectID:   123,
			mrIID:       456,
			expectedURL: "/api/v4/projects/123/merge_requests/456/changes",
		},
		{
			name:        "URL with trailing slash",
			baseURL:     "https://gitlab.example.com/",
			projectID:   789,
			mrIID:       101,
			expectedURL: "/api/v4/projects/789/merge_requests/101/changes",
		},
		{
			name:        "custom GitLab instance",
			baseURL:     "https://git.company.com",
			projectID:   999,
			mrIID:       888,
			expectedURL: "/api/v4/projects/999/merge_requests/888/changes",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var requestURL string
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				requestURL = r.URL.Path
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(MRChanges{Changes: []struct {
					OldPath     string `json:"old_path"`
					NewPath     string `json:"new_path"`
					AMode       string `json:"a_mode"`
					BMode       string `json:"b_mode"`
					NewFile     bool   `json:"new_file"`
					RenamedFile bool   `json:"renamed_file"`
					DeletedFile bool   `json:"deleted_file"`
					Diff        string `json:"diff"`
				}{}})
			}))
			defer server.Close()

			cfg := config.GitLabConfig{
				BaseURL: server.URL,
				Token:   "test-token",
			}
			client := NewClient(cfg)

			_, err := client.FetchMRChanges(tt.projectID, tt.mrIID)

			assert.NoError(t, err)
			assert.Equal(t, tt.expectedURL, requestURL)
		})
	}
}

func TestExtractMRInfo_Success(t *testing.T) {
	tests := []struct {
		name     string
		payload  map[string]interface{}
		expected *MRInfo
	}{
		{
			name: "complete payload with all fields",
			payload: map[string]interface{}{
				"object_attributes": map[string]interface{}{
					"iid":           float64(123),
					"title":         "Update warehouse configuration",
					"source_branch": "feature/update-warehouse",
					"target_branch": "main",
				},
				"project": map[string]interface{}{
					"id": float64(456),
				},
				"user": map[string]interface{}{
					"username": "developer1",
				},
			},
			expected: &MRInfo{
				ProjectID:    456,
				MRIID:        123,
				Title:        "Update warehouse configuration",
				Author:       "developer1",
				SourceBranch: "feature/update-warehouse",
				TargetBranch: "main",
			},
		},
		{
			name: "payload with integer types",
			payload: map[string]interface{}{
				"object_attributes": map[string]interface{}{
					"iid":           123, // int instead of float64
					"title":         "Fix sourcebinding",
					"source_branch": "fix/sourcebinding",
					"target_branch": "develop",
				},
				"project": map[string]interface{}{
					"id": 789, // int instead of float64
				},
				"user": map[string]interface{}{
					"username": "maintainer",
				},
			},
			expected: &MRInfo{
				ProjectID:    789,
				MRIID:        123,
				Title:        "Fix sourcebinding",
				Author:       "maintainer",
				SourceBranch: "fix/sourcebinding",
				TargetBranch: "develop",
			},
		},
		{
			name: "payload with string IDs",
			payload: map[string]interface{}{
				"object_attributes": map[string]interface{}{
					"iid":           "999", // string instead of number
					"title":         "Add new feature",
					"source_branch": "feature/new",
					"target_branch": "main",
				},
				"project": map[string]interface{}{
					"id": "111", // string instead of number
				},
				"user": map[string]interface{}{
					"username": "contributor",
				},
			},
			expected: &MRInfo{
				ProjectID:    111,
				MRIID:        999,
				Title:        "Add new feature",
				Author:       "contributor",
				SourceBranch: "feature/new",
				TargetBranch: "main",
			},
		},
		{
			name: "minimal payload with only required fields",
			payload: map[string]interface{}{
				"object_attributes": map[string]interface{}{
					"iid": float64(555),
				},
				"project": map[string]interface{}{
					"id": float64(666),
				},
			},
			expected: &MRInfo{
				ProjectID:    666,
				MRIID:        555,
				Title:        "",
				Author:       "",
				SourceBranch: "",
				TargetBranch: "",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ExtractMRInfo(tt.payload)

			assert.NoError(t, err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestExtractMRInfo_Errors(t *testing.T) {
	tests := []struct {
		name          string
		payload       map[string]interface{}
		expectedError string
	}{
		{
			name:          "missing object_attributes",
			payload:       map[string]interface{}{},
			expectedError: "missing project ID (0) or MR IID (0)",
		},
		{
			name: "missing iid",
			payload: map[string]interface{}{
				"object_attributes": map[string]interface{}{
					"title": "Test MR",
				},
				"project": map[string]interface{}{
					"id": float64(123),
				},
			},
			expectedError: "missing project ID (123) or MR IID (0)",
		},
		{
			name: "missing project",
			payload: map[string]interface{}{
				"object_attributes": map[string]interface{}{
					"iid": float64(456),
				},
			},
			expectedError: "missing project ID (0) or MR IID (456)",
		},
		{
			name: "missing project id",
			payload: map[string]interface{}{
				"object_attributes": map[string]interface{}{
					"iid": float64(789),
				},
				"project": map[string]interface{}{
					"name": "test-project",
				},
			},
			expectedError: "missing project ID (0) or MR IID (789)",
		},
		{
			name: "invalid iid type",
			payload: map[string]interface{}{
				"object_attributes": map[string]interface{}{
					"iid": []string{"invalid"}, // Invalid type
				},
				"project": map[string]interface{}{
					"id": float64(123),
				},
			},
			expectedError: "missing project ID (123) or MR IID (0)",
		},
		{
			name: "invalid project id type",
			payload: map[string]interface{}{
				"object_attributes": map[string]interface{}{
					"iid": float64(456),
				},
				"project": map[string]interface{}{
					"id": map[string]string{"invalid": "type"}, // Invalid type
				},
			},
			expectedError: "missing project ID (0) or MR IID (456)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ExtractMRInfo(tt.payload)

			assert.Error(t, err)
			assert.Nil(t, result)
			assert.Contains(t, err.Error(), tt.expectedError)
		})
	}
}

func TestExtractMRInfo_TypeConversion(t *testing.T) {
	tests := []struct {
		name     string
		iidValue interface{}
		idValue  interface{}
		expected *MRInfo
	}{
		{
			name:     "float64 values",
			iidValue: float64(123),
			idValue:  float64(456),
			expected: &MRInfo{ProjectID: 456, MRIID: 123},
		},
		{
			name:     "int values",
			iidValue: 789,
			idValue:  101112,
			expected: &MRInfo{ProjectID: 101112, MRIID: 789},
		},
		{
			name:     "string values",
			iidValue: "999",
			idValue:  "888",
			expected: &MRInfo{ProjectID: 888, MRIID: 999},
		},
		{
			name:     "mixed types",
			iidValue: "777",
			idValue:  float64(666),
			expected: &MRInfo{ProjectID: 666, MRIID: 777},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			payload := map[string]interface{}{
				"object_attributes": map[string]interface{}{
					"iid": tt.iidValue,
				},
				"project": map[string]interface{}{
					"id": tt.idValue,
				},
			}

			result, err := ExtractMRInfo(payload)

			assert.NoError(t, err)
			assert.Equal(t, tt.expected.ProjectID, result.ProjectID)
			assert.Equal(t, tt.expected.MRIID, result.MRIID)
		})
	}
}

func TestExtractMRInfo_PartialData(t *testing.T) {
	// Test payload with some missing optional fields
	payload := map[string]interface{}{
		"object_attributes": map[string]interface{}{
			"iid":   float64(123),
			"title": "Partial MR",
			// Missing source_branch and target_branch
		},
		"project": map[string]interface{}{
			"id": float64(456),
		},
		// Missing user section
	}

	result, err := ExtractMRInfo(payload)

	assert.NoError(t, err)
	assert.Equal(t, 456, result.ProjectID)
	assert.Equal(t, 123, result.MRIID)
	assert.Equal(t, "Partial MR", result.Title)
	assert.Equal(t, "", result.Author)
	assert.Equal(t, "", result.SourceBranch)
	assert.Equal(t, "", result.TargetBranch)
}

func TestClient_FetchMRChanges_EmptyResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(MRChanges{Changes: []struct {
			OldPath     string `json:"old_path"`
			NewPath     string `json:"new_path"`
			AMode       string `json:"a_mode"`
			BMode       string `json:"b_mode"`
			NewFile     bool   `json:"new_file"`
			RenamedFile bool   `json:"renamed_file"`
			DeletedFile bool   `json:"deleted_file"`
			Diff        string `json:"diff"`
		}{}}) // Empty changes array
	}))
	defer server.Close()

	cfg := config.GitLabConfig{
		BaseURL: server.URL,
		Token:   "test-token",
	}
	client := NewClient(cfg)

	changes, err := client.FetchMRChanges(123, 456)

	assert.NoError(t, err)
	assert.Empty(t, changes)
}

func TestClient_RequestHeaders(t *testing.T) {
	var capturedHeaders http.Header
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedHeaders = r.Header
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(MRChanges{Changes: []struct {
			OldPath     string `json:"old_path"`
			NewPath     string `json:"new_path"`
			AMode       string `json:"a_mode"`
			BMode       string `json:"b_mode"`
			NewFile     bool   `json:"new_file"`
			RenamedFile bool   `json:"renamed_file"`
			DeletedFile bool   `json:"deleted_file"`
			Diff        string `json:"diff"`
		}{}})
	}))
	defer server.Close()

	cfg := config.GitLabConfig{
		BaseURL: server.URL,
		Token:   "test-token-xyz",
	}
	client := NewClient(cfg)

	_, err := client.FetchMRChanges(123, 456)

	assert.NoError(t, err)
	assert.Equal(t, "Bearer test-token-xyz", capturedHeaders.Get("Authorization"))
	assert.Equal(t, "application/json", capturedHeaders.Get("Content-Type"))
}
