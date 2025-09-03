package shared

import (
	"strings"
)

// IsDataProductFile checks if a file is a dataproduct configuration file
func IsDataProductFile(path string) bool {
	if path == "" {
		return false
	}

	lowerPath := strings.ToLower(path)
	return strings.HasSuffix(lowerPath, "product.yaml") || strings.HasSuffix(lowerPath, "product.yml")
}

// IsMigrationFile checks if a file is a migration file
func IsMigrationFile(path string) bool {
	if path == "" {
		return false
	}

	lowerPath := strings.ToLower(path)
	return strings.Contains(lowerPath, "/migrations/") &&
		(strings.HasSuffix(lowerPath, ".sql") || strings.HasSuffix(lowerPath, ".yaml") || strings.HasSuffix(lowerPath, ".yml"))
}