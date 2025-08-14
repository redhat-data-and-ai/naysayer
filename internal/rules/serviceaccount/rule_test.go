package serviceaccount

import (
	"errors"
	"testing"

	"github.com/redhat-data-and-ai/naysayer/internal/gitlab"
	"github.com/redhat-data-and-ai/naysayer/internal/rules/shared"
	"github.com/stretchr/testify/assert"
)

// MockGitLabClient for testing
type MockGitLabClient struct {
	fileContent  string
	returnError  bool
	fileContents map[string]string // For multiple files
}

func (m *MockGitLabClient) FetchFileContent(projectID int, filePath, ref string) (*gitlab.FileContent, error) {
	if m.returnError {
		return nil, errors.New("mock error")
	}
	
	// Check if we have specific content for this file
	if m.fileContents != nil {
		if content, exists := m.fileContents[filePath]; exists {
			return &gitlab.FileContent{Content: content}, nil
		}
	}
	
	// Fallback to single content
	return &gitlab.FileContent{Content: m.fileContent}, nil
}

func TestServiceAccountRule_Applies(t *testing.T) {
	rule := NewRule(&MockGitLabClient{})

	// Test applies to service account files
	mrCtx := &shared.MRContext{
		Changes: []gitlab.FileChange{{NewPath: "serviceaccounts/dev/marketo_astro_dev_appuser.yaml"}},
	}
	assert.True(t, rule.Applies(mrCtx))

	// Test does not apply to other files
	mrCtx = &shared.MRContext{
		Changes: []gitlab.FileChange{{NewPath: "dataproducts/source/myproduct/dev/product.yaml"}},
	}
	assert.False(t, rule.Applies(mrCtx))

	// Test applies to yml extension
	mrCtx = &shared.MRContext{
		Changes: []gitlab.FileChange{{NewPath: "serviceaccounts/prod/dataverse_operator_prod_appuser.yml"}},
	}
	assert.True(t, rule.Applies(mrCtx))
}

func TestServiceAccountRule_ShouldApprove_ValidConfig(t *testing.T) {
	validYAML := `---
name: marketo_custom_dev_appuser
comment: "service account for marketo in dev environment"
email: john.doe@redhat.com
role: MARKETO_READER
`

	mockClient := &MockGitLabClient{fileContent: validYAML}
	rule := NewRule(mockClient)
	mrCtx := &shared.MRContext{
		ProjectID: 123,
		MRIID:     456,
		Changes: []gitlab.FileChange{
			{NewPath: "serviceaccounts/dev/marketo_custom_dev_appuser.yaml"},
		},
	}

	decision, reason := rule.ShouldApprove(mrCtx)

	assert.Equal(t, shared.Approve, decision)
	assert.Contains(t, reason, "Service account validation passed")
}

func TestServiceAccountRule_ShouldApprove_InvalidEmail(t *testing.T) {
	invalidYAML := `---
name: marketo_astro_dev_appuser
comment: "service account for marketo"
email: dataverse-platform-team@redhat.com
role: MARKETO_READER
`

	mockClient := &MockGitLabClient{fileContent: invalidYAML}
	rule := NewRule(mockClient)
	mrCtx := &shared.MRContext{
		ProjectID: 123,
		MRIID:     456,
		Changes: []gitlab.FileChange{
			{NewPath: "serviceaccounts/dev/marketo_astro_dev_appuser.yaml"},
		},
	}

	decision, reason := rule.ShouldApprove(mrCtx)

	assert.Equal(t, shared.ManualReview, decision)
	assert.Contains(t, reason, "Group email addresses are not allowed")
}

func TestServiceAccountRule_ShouldApprove_AstroInDevEnvironment(t *testing.T) {
	invalidYAML := `---
name: marketo_astro_dev_appuser
comment: "service account for marketo astro in dev"
email: john.doe@redhat.com
role: MARKETO_READER
`

	mockClient := &MockGitLabClient{fileContent: invalidYAML}
	rule := NewRule(mockClient)
	mrCtx := &shared.MRContext{
		ProjectID: 123,
		MRIID:     456,
		Changes: []gitlab.FileChange{
			{NewPath: "serviceaccounts/dev/marketo_astro_dev_appuser.yaml"},
		},
	}

	decision, reason := rule.ShouldApprove(mrCtx)

	assert.Equal(t, shared.ManualReview, decision)
	assert.Contains(t, reason, "Astro service accounts are only allowed in preprod and prod")
}

