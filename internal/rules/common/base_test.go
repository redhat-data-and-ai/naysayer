package common

import (
	"testing"

	"github.com/redhat-data-and-ai/naysayer/internal/gitlab"
	"github.com/redhat-data-and-ai/naysayer/internal/rules/shared"
	"github.com/stretchr/testify/assert"
)

func TestNewBaseRule(t *testing.T) {
	rule := NewBaseRule("test_rule", "Test rule description")

	assert.Equal(t, "test_rule", rule.Name())
	assert.Equal(t, "Test rule description", rule.Description())
	assert.Nil(t, rule.GetMRContext())
}

func TestBaseRule_Name(t *testing.T) {
	tests := []struct {
		name         string
		ruleName     string
		expectedName string
	}{
		{"simple name", "warehouse_rule", "warehouse_rule"},
		{"complex name", "metadata-validation-rule", "metadata-validation-rule"},
		{"empty name", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rule := NewBaseRule(tt.ruleName, "Test description")
			assert.Equal(t, tt.expectedName, rule.Name())
		})
	}
}

func TestBaseRule_Description(t *testing.T) {
	tests := []struct {
		name                string
		description         string
		expectedDescription string
	}{
		{"simple description", "Validates warehouse configurations", "Validates warehouse configurations"},
		{"complex description", "Auto-approves metadata files and DBT configurations", "Auto-approves metadata files and DBT configurations"},
		{"empty description", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rule := NewBaseRule("test_rule", tt.description)
			assert.Equal(t, tt.expectedDescription, rule.Description())
		})
	}
}

func TestBaseRule_SetMRContext(t *testing.T) {
	rule := NewBaseRule("test_rule", "Test description")

	// Initially no context
	assert.Nil(t, rule.GetMRContext())

	// Set context
	mrCtx := &shared.MRContext{
		ProjectID: 123,
		MRIID:     456,
		Changes: []gitlab.FileChange{
			{NewPath: "test.yaml"},
		},
		MRInfo: &gitlab.MRInfo{
			Title:  "Test MR",
			Author: "testuser",
		},
	}

	rule.SetMRContext(mrCtx)
	assert.Equal(t, mrCtx, rule.GetMRContext())
	assert.Equal(t, 123, rule.GetMRContext().ProjectID)
	assert.Equal(t, 456, rule.GetMRContext().MRIID)
}

func TestBaseRule_GetMRContext(t *testing.T) {
	rule := NewBaseRule("test_rule", "Test description")

	// Test nil context
	assert.Nil(t, rule.GetMRContext())

	// Test with context
	mrCtx := &shared.MRContext{
		ProjectID: 789,
		MRIID:     101112,
	}
	rule.SetMRContext(mrCtx)

	retrievedCtx := rule.GetMRContext()
	assert.NotNil(t, retrievedCtx)
	assert.Equal(t, 789, retrievedCtx.ProjectID)
	assert.Equal(t, 101112, retrievedCtx.MRIID)
}

func TestBaseRule_GetFullFileCoverage(t *testing.T) {
	rule := NewBaseRule("test_rule", "Test description")

	tests := []struct {
		name          string
		filePath      string
		fileContent   string
		expectedLines int
		expectedStart int
		expectedEnd   int
	}{
		{
			name:          "single line file",
			filePath:      "test.yaml",
			fileContent:   "name: test",
			expectedLines: 1,
			expectedStart: 1,
			expectedEnd:   1,
		},
		{
			name:          "multi line file",
			filePath:      "config.yaml",
			fileContent:   "name: test\nversion: 1.0\ndescription: Test config",
			expectedLines: 1,
			expectedStart: 1,
			expectedEnd:   3,
		},
		{
			name:          "file with trailing newline",
			filePath:      "data.yaml",
			fileContent:   "line1\nline2\nline3\n",
			expectedLines: 1,
			expectedStart: 1,
			expectedEnd:   4,
		},
		{
			name:          "empty file",
			filePath:      "empty.yaml",
			fileContent:   "",
			expectedLines: 0,
			expectedStart: 0,
			expectedEnd:   0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lines := rule.GetFullFileCoverage(tt.filePath, tt.fileContent)

			if tt.expectedLines == 0 {
				assert.Len(t, lines, 0)
			} else {
				assert.Len(t, lines, tt.expectedLines)
				assert.Equal(t, tt.filePath, lines[0].FilePath)
				assert.Equal(t, tt.expectedStart, lines[0].StartLine)
				assert.Equal(t, tt.expectedEnd, lines[0].EndLine)
			}
		})
	}
}

func TestBaseRule_ContainsYAMLField(t *testing.T) {
	rule := NewBaseRule("test_rule", "Test description")

	tests := []struct {
		name        string
		fileContent string
		field       string
		expected    bool
	}{
		{
			name:        "simple field present",
			fileContent: "name: test\nversion: 1.0",
			field:       "name",
			expected:    true,
		},
		{
			name:        "simple field not present",
			fileContent: "version: 1.0\ndescription: test",
			field:       "name",
			expected:    false,
		},
		{
			name:        "nested field under spec",
			fileContent: "spec:\n  warehouse:\n    size: SMALL",
			field:       "warehouse",
			expected:    true,
		},
		{
			name:        "field in middle of content",
			fileContent: "metadata:\n  name: test\n  version: 1.0\nwarehouse:\n  size: LARGE",
			field:       "warehouse",
			expected:    true,
		},
		{
			name:        "field with colon in value",
			fileContent: "database:\n  url: postgres://localhost:5432/db",
			field:       "database",
			expected:    true,
		},
		{
			name:        "partial field match should not match",
			fileContent: "warehouse_size: LARGE\nother: value",
			field:       "warehouse",
			expected:    false,
		},
		{
			name:        "empty content",
			fileContent: "",
			field:       "name",
			expected:    false,
		},
		{
			name:        "empty field",
			fileContent: "name: test",
			field:       "",
			expected:    true, // Empty field with colon will be found
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := rule.ContainsYAMLField(tt.fileContent, tt.field)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestBaseRule_Integration(t *testing.T) {
	// Test full integration of BaseRule functionality
	rule := NewBaseRule("integration_test_rule", "Integration test for BaseRule")

	// Test initial state
	assert.Equal(t, "integration_test_rule", rule.Name())
	assert.Equal(t, "Integration test for BaseRule", rule.Description())
	assert.Nil(t, rule.GetMRContext())

	// Test MR context setting
	mrCtx := &shared.MRContext{
		ProjectID:   12345,
		MRIID:       67890,
		Environment: "test",
		Labels:      []string{"enhancement", "test"},
	}
	rule.SetMRContext(mrCtx)
	assert.Equal(t, mrCtx, rule.GetMRContext())

	// Test file coverage
	fileContent := "name: test-service\nversion: 1.0.0\ndescription: Test service\n"
	coverage := rule.GetFullFileCoverage("service.yaml", fileContent)
	assert.Len(t, coverage, 1)
	assert.Equal(t, "service.yaml", coverage[0].FilePath)
	assert.Equal(t, 1, coverage[0].StartLine)
	assert.Equal(t, 4, coverage[0].EndLine) // 3 lines + trailing newline

	// Test YAML field detection
	assert.True(t, rule.ContainsYAMLField(fileContent, "name"))
	assert.True(t, rule.ContainsYAMLField(fileContent, "version"))
	assert.False(t, rule.ContainsYAMLField(fileContent, "database"))
}
