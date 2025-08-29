package common

import (
	"strings"
	"testing"

	"github.com/redhat-data-and-ai/naysayer/internal/rules/shared"
	"github.com/stretchr/testify/assert"
)

func TestMetadataRule_ValidateLines_DocumentationFiles(t *testing.T) {
	rule := NewMetadataRule()

	tests := []struct {
		name                   string
		filePath               string
		fileContent            string
		expectedDecision       shared.DecisionType
		expectedReasonContains string
	}{
		{
			name:                   "README.md file",
			filePath:               "dataproducts/test/README.md",
			fileContent:            "# Test Project\nThis is a test project.",
			expectedDecision:       shared.Approve,
			expectedReasonContains: "README file changes are documentation updates",
		},
		{
			name:                   "data_elements.md file",
			filePath:               "dataproducts/analytics/data_elements.md",
			fileContent:            "# Data Elements\n- user_id\n- timestamp",
			expectedDecision:       shared.Approve,
			expectedReasonContains: "Data elements documentation changes are metadata updates",
		},
		{
			name:                   "promotion_checklist.md file",
			filePath:               "dataproducts/prod/promotion_checklist.md",
			fileContent:            "# Promotion Checklist\n- [ ] Tests pass\n- [ ] Documentation updated",
			expectedDecision:       shared.Approve,
			expectedReasonContains: "Promotion checklist changes are process documentation",
		},
		{
			name:                   "developers.yaml file",
			filePath:               "dataproducts/team/developers.yaml",
			fileContent:            "team:\n  - name: John Doe\n    email: john@example.com",
			expectedDecision:       shared.Approve,
			expectedReasonContains: "Developer configuration changes are team metadata",
		},
		{
			name:                   "changelog file",
			filePath:               "CHANGELOG.md",
			fileContent:            "# Changelog\n## v1.0.0\n- Initial release",
			expectedDecision:       shared.Approve,
			expectedReasonContains: "Changelog updates are version history metadata",
		},
		{
			name:                   "license file",
			filePath:               "LICENSE",
			fileContent:            "MIT License\nCopyright (c) 2024",
			expectedDecision:       shared.Approve,
			expectedReasonContains: "License file changes are legal metadata",
		},
		{
			name:                   "docs directory file",
			filePath:               "docs/api/endpoints.md",
			fileContent:            "# API Endpoints\n## GET /users",
			expectedDecision:       shared.Approve,
			expectedReasonContains: "Documentation directory changes are content updates",
		},
		{
			name:                   "generic markdown file",
			filePath:               "notes.md",
			fileContent:            "# Notes\nSome project notes",
			expectedDecision:       shared.Approve,
			expectedReasonContains: "Markdown documentation changes are generally safe",
		},
		{
			name:                   "text file",
			filePath:               "requirements.txt",
			fileContent:            "requests==2.25.1\nflask==2.0.1",
			expectedDecision:       shared.Approve,
			expectedReasonContains: "Text file changes are documentation updates",
		},
		{
			name:                   "CODEOWNERS file",
			filePath:               ".github/CODEOWNERS",
			fileContent:            "* @team-lead\n/docs/ @docs-team",
			expectedDecision:       shared.Approve,
			expectedReasonContains: "Metadata file changes are generally safe",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			decision, reason := rule.ValidateLines(tt.filePath, tt.fileContent, []shared.LineRange{})

			assert.Equal(t, tt.expectedDecision, decision)
			assert.Contains(t, reason, tt.expectedReasonContains)
		})
	}
}

func TestMetadataRule_ValidateLines_DBTMetadataSection(t *testing.T) {
	rule := NewMetadataRule()

	tests := []struct {
		name                   string
		filePath               string
		fileContent            string
		expectedDecision       shared.DecisionType
		expectedReasonContains string
	}{
		{
			name:     "DBT metadata in product.yaml",
			filePath: "dataproducts/analytics/product.yaml",
			fileContent: `name: analytics
service_account:
  dbt: true
warehouses:
  - type: user
    size: SMALL`,
			expectedDecision:       shared.Approve,
			expectedReasonContains: "DBT metadata configuration changes are safe",
		},
		{
			name:     "Product file without DBT metadata",
			filePath: "dataproducts/analytics/product.yaml",
			fileContent: `name: analytics
warehouses:
  - type: user
    size: SMALL`,
			expectedDecision:       shared.ManualReview,
			expectedReasonContains: "Not a metadata file - requires manual review",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			decision, reason := rule.ValidateLines(tt.filePath, tt.fileContent, []shared.LineRange{})

			assert.Equal(t, tt.expectedDecision, decision)
			assert.Contains(t, reason, tt.expectedReasonContains)
		})
	}
}

func TestMetadataRule_ValidateLines_NonMetadataFiles(t *testing.T) {
	rule := NewMetadataRule()

	tests := []struct {
		name             string
		filePath         string
		fileContent      string
		expectedDecision shared.DecisionType
	}{
		{
			name:             "Python source file",
			filePath:         "src/main.py",
			fileContent:      "def main():\n    print('Hello, World!')",
			expectedDecision: shared.ManualReview,
		},
		{
			name:             "Configuration file",
			filePath:         "config/database.json",
			fileContent:      `{"host": "localhost", "port": 5432}`,
			expectedDecision: shared.ManualReview,
		},
		{
			name:             "SQL migration",
			filePath:         "migrations/001_create_users.sql",
			fileContent:      "CREATE TABLE users (id INT PRIMARY KEY);",
			expectedDecision: shared.ManualReview,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			decision, _ := rule.ValidateLines(tt.filePath, tt.fileContent, []shared.LineRange{})

			assert.Equal(t, tt.expectedDecision, decision)
		})
	}
}

func TestMetadataRule_GetCoveredLines(t *testing.T) {
	rule := NewMetadataRule()

	tests := []struct {
		name                 string
		filePath             string
		fileContent          string
		expectedCoverageType string
	}{
		{
			name:                 "README file gets full coverage",
			filePath:             "README.md",
			fileContent:          "# Test\nLine 1\nLine 2\nLine 3",
			expectedCoverageType: "full",
		},
		{
			name:                 "DBT metadata gets placeholder coverage",
			filePath:             "product.yaml",
			fileContent:          "service_account:\n  dbt: true",
			expectedCoverageType: "placeholder",
		},
		{
			name:                 "Non-metadata file gets no coverage",
			filePath:             "src/main.py",
			fileContent:          "def main(): pass",
			expectedCoverageType: "none",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			coveredLines := rule.GetCoveredLines(tt.filePath, tt.fileContent)

			switch tt.expectedCoverageType {
			case "full":
				assert.NotEmpty(t, coveredLines)
				// Should cover all lines in the file
				expectedLines := len(strings.Split(tt.fileContent, "\n"))
				if len(coveredLines) > 0 {
					assert.Equal(t, expectedLines, coveredLines[0].EndLine)
				}
			case "placeholder":
				assert.Len(t, coveredLines, 1)
				assert.Equal(t, 1, coveredLines[0].StartLine)
				assert.Equal(t, 1, coveredLines[0].EndLine)
			case "none":
				assert.Empty(t, coveredLines)
			}
		})
	}
}

func TestMetadataRule_IsMetadataFile(t *testing.T) {
	rule := NewMetadataRule()

	tests := []struct {
		name     string
		filePath string
		expected bool
	}{
		// Documentation files
		{"README", "README.md", true},
		{"Data elements", "data_elements.md", true},
		{"Promotion checklist", "promotion_checklist.md", true},
		{"Developers config", "developers.yaml", true},

		// Additional metadata files
		{"Changelog", "CHANGELOG.md", true},
		{"License", "LICENSE", true},
		{"Authors", "AUTHORS.md", true},
		{"Contributors", "CONTRIBUTORS.md", true},
		{"CODEOWNERS", ".github/CODEOWNERS", true},

		// Directory patterns
		{"Docs directory", "docs/api/reference.md", true},
		{"Documentation directory", "documentation/setup.md", true},

		// Generic patterns
		{"Any markdown", "notes.md", true},
		{"Text file", "requirements.txt", true},

		// Non-metadata files
		{"Python file", "src/main.py", false},
		{"JSON config", "config.json", false},
		{"SQL file", "schema.sql", false},
		{"Empty path", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := rule.isMetadataFile(tt.filePath)
			assert.Equal(t, tt.expected, result)
		})
	}
}
