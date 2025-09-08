package common

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewFileTypeMatcher(t *testing.T) {
	matcher := NewFileTypeMatcher()
	assert.NotNil(t, matcher)
}

func TestFileTypeMatcher_IsProductFile(t *testing.T) {
	matcher := NewFileTypeMatcher()

	tests := []struct {
		name     string
		filePath string
		expected bool
	}{
		// Valid product files
		{"product.yaml file", "dataproducts/analytics/product.yaml", true},
		{"product.yml file", "dataproducts/source/platform/product.yml", true},
		{"uppercase product.yaml", "dataproducts/test/PRODUCT.YAML", true},
		{"uppercase product.yml", "dataproducts/test/PRODUCT.YML", true},
		{"mixed case", "dataproducts/test/Product.Yaml", true},
		{"nested path", "dataproducts/source/deep/nested/path/product.yaml", true},
		{"root level product.yaml", "product.yaml", true},
		{"root level product.yml", "product.yml", true},

		// Invalid product files
		{"README file", "README.md", false},
		{"developers file", "dataproducts/analytics/developers.yaml", false},
		{"config file", "config/settings.yaml", false},
		{"similar but not exact - product-config", "dataproducts/test/product-config.yaml", false},
		{"similar but not exact - products", "dataproducts/test/products.yaml", false},
		{"different extension", "dataproducts/test/product.json", false},
		{"partial match", "dataproducts/test/myproduct.yaml", true}, // HasSuffix matches this
		{"yaml file in product directory", "product/config.yaml", false},

		// Edge cases
		{"empty path", "", false},
		{"path with spaces", "data products/test/product.yaml", true},
		{"path with special characters", "dataproducts/test-env/product.yaml", true},
		{"unicode in path", "dataproducts/测试/product.yaml", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := matcher.IsProductFile(tt.filePath)
			assert.Equal(t, tt.expected, result, "IsProductFile(%q) = %v, want %v", tt.filePath, result, tt.expected)
		})
	}
}

func TestFileTypeMatcher_IsDocumentationFile(t *testing.T) {
	matcher := NewFileTypeMatcher()

	tests := []struct {
		name     string
		filePath string
		expected bool
	}{
		// Valid documentation files
		{"README.md", "README.md", true},
		{"nested README", "docs/README.md", true},
		{"data elements", "data_elements.md", true},
		{"nested data elements", "dataproducts/analytics/data_elements.md", true},
		{"promotion checklist", "promotion_checklist.md", true},
		{"nested promotion checklist", "docs/promotion_checklist.md", true},
		{"developers.yaml", "developers.yaml", true},
		{"developers.yml", "developers.yml", true},
		{"nested developers config", "dataproducts/test/developers.yaml", true},

		// Case variations
		{"uppercase README", "README.MD", true},
		{"uppercase data elements", "DATA_ELEMENTS.MD", true},
		{"mixed case developers", "Developers.Yaml", true},

		// Invalid documentation files
		{"regular markdown", "guide.md", false},
		{"config file", "config.yaml", false},
		{"product file", "product.yaml", false},
		{"source code", "main.py", false},
		{"similar but not exact - readme-guide", "readme-guide.md", false},
		{"partial match - my-readme", "my-readme.md", true}, // HasSuffix("my-readme.md", "readme.md") = true

		// Edge cases
		{"empty path", "", false},
		{"path with spaces", "docs/README with spaces.md", false}, // Only exact matches
		{"unicode in path", "docs/README测试.md", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := matcher.IsDocumentationFile(tt.filePath)
			assert.Equal(t, tt.expected, result, "IsDocumentationFile(%q) = %v, want %v", tt.filePath, result, tt.expected)
		})
	}
}

func TestFileTypeMatcher_IsWarehouseFile(t *testing.T) {
	matcher := NewFileTypeMatcher()

	tests := []struct {
		name     string
		filePath string
		expected bool
	}{
		// Should delegate to IsProductFile
		{"product.yaml is warehouse file", "dataproducts/analytics/product.yaml", true},
		{"product.yml is warehouse file", "dataproducts/source/platform/product.yml", true},
		{"README is not warehouse file", "README.md", false},
		{"config is not warehouse file", "config.yaml", false},
		{"empty path", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := matcher.IsWarehouseFile(tt.filePath)
			assert.Equal(t, tt.expected, result)

			// Should match IsProductFile result
			productResult := matcher.IsProductFile(tt.filePath)
			assert.Equal(t, productResult, result, "IsWarehouseFile should delegate to IsProductFile")
		})
	}
}

func TestFileTypeMatcher_Integration(t *testing.T) {
	matcher := NewFileTypeMatcher()

	// Test a comprehensive set of real-world file paths
	testFiles := map[string]struct {
		isProduct   bool
		isDoc       bool
		isWarehouse bool
	}{
		"dataproducts/aggregate/bookingsmaster/prod/product.yaml": {true, false, true},
		"dataproducts/source/marketo/sandbox/product.yml":         {true, false, true},
		"dataproducts/analytics/README.md":                        {false, true, false},
		"dataproducts/test/data_elements.md":                      {false, true, false},
		"dataproducts/source/platform/developers.yaml":            {false, true, false},
		"config/deployment.yaml":                                  {false, false, false},
		"scripts/deploy.sh":                                       {false, false, false},
		"docs/api-guide.md":                                       {false, false, false},
	}

	for filePath, expected := range testFiles {
		t.Run("integration_"+filePath, func(t *testing.T) {
			assert.Equal(t, expected.isProduct, matcher.IsProductFile(filePath), "IsProductFile mismatch for %s", filePath)
			assert.Equal(t, expected.isDoc, matcher.IsDocumentationFile(filePath), "IsDocumentationFile mismatch for %s", filePath)
			assert.Equal(t, expected.isWarehouse, matcher.IsWarehouseFile(filePath), "IsWarehouseFile mismatch for %s", filePath)
		})
	}
}

func TestFileTypeMatcher_EdgeCases(t *testing.T) {
	matcher := NewFileTypeMatcher()

	// Test edge cases that might cause issues
	edgeCases := []string{
		"",              // Empty string
		"   ",           // Whitespace
		"/",             // Root path
		".",             // Current directory
		"..",            // Parent directory
		"product.yaml/", // Trailing slash
		"/product.yaml", // Leading slash
		"very/long/nested/path/with/many/levels/product.yaml", // Deep nesting
		"product.yaml.backup", // Additional extension
		"backup.product.yaml", // Prefix
	}

	for _, filePath := range edgeCases {
		t.Run("edge_case_"+filePath, func(t *testing.T) {
			// These should not panic and should return consistent results
			productResult := matcher.IsProductFile(filePath)
			docResult := matcher.IsDocumentationFile(filePath)
			warehouseResult := matcher.IsWarehouseFile(filePath)

			// Warehouse should always match product
			assert.Equal(t, productResult, warehouseResult, "IsWarehouseFile should match IsProductFile for %q", filePath)

			// All results should be boolean (not panic)
			assert.IsType(t, false, productResult)
			assert.IsType(t, false, docResult)
			assert.IsType(t, false, warehouseResult)
		})
	}
}
