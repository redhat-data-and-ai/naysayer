package common

import (
	"strings"
)

// FileTypeMatcher provides common file type detection logic
type FileTypeMatcher struct{}

// NewFileTypeMatcher creates a new file type matcher
func NewFileTypeMatcher() *FileTypeMatcher {
	return &FileTypeMatcher{}
}

// IsProductFile checks if a file is a product configuration file
func (m *FileTypeMatcher) IsProductFile(filePath string) bool {
	if filePath == "" {
		return false
	}

	lowerPath := strings.ToLower(filePath)
	return strings.HasSuffix(lowerPath, "product.yaml") ||
		strings.HasSuffix(lowerPath, "product.yml")
}

// IsDocumentationFile checks if a file is a documentation file
func (m *FileTypeMatcher) IsDocumentationFile(filePath string) bool {
	if filePath == "" {
		return false
	}

	lowerPath := strings.ToLower(filePath)

	// Check for documentation file patterns
	return strings.HasSuffix(lowerPath, "readme.md") ||
		strings.HasSuffix(lowerPath, "data_elements.md") ||
		strings.HasSuffix(lowerPath, "promotion_checklist.md") ||
		strings.HasSuffix(lowerPath, "developers.yaml") ||
		strings.HasSuffix(lowerPath, "developers.yml")
}

// IsWarehouseFile checks if a file is a warehouse configuration file
func (m *FileTypeMatcher) IsWarehouseFile(filePath string) bool {
	return m.IsProductFile(filePath) // Warehouse rules apply to product files
}

// Note: Pattern matching functionality moved to shared.PatternMatcher
